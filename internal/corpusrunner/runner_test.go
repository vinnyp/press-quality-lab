//go:build scripted

// Package corpusrunner tests collection independently of production decoders.
package corpusrunner

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/caseassembly"
	"github.com/vinnyp/press-quality-lab/internal/scripted"
)

const kit = "../../docs/specs/conformance-collection-1.0"

func object(v any) map[string]any {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	return m
}
func list(v any) []any {
	a, ok := v.([]any)
	if !ok {
		return nil
	}
	return a
}
func spelling(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// Input arrays have no order; sorting retains duplicate multiplicities.
func unordered(v any) any {
	switch x := v.(type) {
	case []any:
		a := make([]any, len(x))
		for i, e := range x {
			a[i] = unordered(e)
		}
		slices.SortFunc(a, func(a, b any) int { return strings.Compare(spelling(a), spelling(b)) })
		return a
	case map[string]any:
		m := make(map[string]any, len(x))
		for k, e := range x {
			m[k] = unordered(e)
		}
		return m
	default:
		return v
	}
}

func sameFields(got, want map[string]any, fields []string) error {
	for _, f := range fields {
		a, aok := got[f]
		b, bok := want[f]
		if !aok || !bok || !reflect.DeepEqual(unordered(a), unordered(b)) {
			return fmt.Errorf("field %s differs or is absent", f)
		}
	}
	return nil
}

func compareInput(got, want map[string]any) error {
	fields := []string{"request", "scope", "claimed_tier", "subject_facts", "required", "trials", "markers", "baseline", "retired", "accepted_regressions"}
	if err := sameFields(got, want, fields); err != nil {
		return err
	}
	if len(got) != len(want) {
		return errors.New("input field set differs")
	}
	ga, gok := got["evidence"].([]any)
	wa, wok := want["evidence"].([]any)
	if !gok || !wok || len(ga) != len(wa) {
		return errors.New("evidence count differs or absent")
	}
	index := map[string]map[string]any{}
	for _, v := range ga {
		m := object(v)
		id, ok := m["check_id"].(string)
		if !ok || index[id] != nil {
			return errors.New("missing or repeated check_id")
		}
		index[id] = m
	}
	seen := map[string]bool{}
	for _, v := range wa {
		w := object(v)
		id, ok := w["check_id"].(string)
		if !ok || seen[id] {
			return errors.New("invalid expected check_id")
		}
		seen[id] = true
		g := index[id]
		if g == nil {
			return errors.New("missing evidence")
		}
		if err := sameFields(g, w, []string{"check_id", "outcome", "reason", "failure_kind", "target_size", "precondition_observed", "tier"}); err != nil {
			return err
		}
		gt, gok := g["targets"].([]any)
		wt, wok := w["targets"].([]any)
		if !gok || !wok || len(gt) != len(wt) {
			return errors.New("targets differ or absent")
		}
		// Project only QE19's three target fields, requiring their presence.
		project := func(a []any) ([]any, error) {
			out := make([]any, 0, len(a))
			for _, v := range a {
				m := object(v)
				p := map[string]any{}
				for _, f := range []string{"kind", "id", "tier"} {
					e, ok := m[f]
					if !ok {
						return nil, fmt.Errorf("target field %s absent", f)
					}
					p[f] = e
				}
				out = append(out, p)
			}
			return out, nil
		}
		gp, err := project(gt)
		if err != nil {
			return err
		}
		wp, err := project(wt)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(unordered(gp), unordered(wp)) {
			return errors.New("target fields differ")
		}
	}
	return nil
}

// Trace is the append-only JSON stream shared with the scripted source in PR6.
// The first event is {"kind":"required","value":[]}; subsequent events append
// one served key or one home. Even a rejection writes the empty required event.
// The trace path lives outside the per-run workspace.
type trace struct {
	Required []any
	Served   []any
	Homes    []string
}

func readTrace(path string) (trace, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return trace{}, err
	}
	tr := trace{Served: []any{}, Homes: []string{}}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	for {
		var event map[string]any
		err := decoder.Decode(&event)
		if err == io.EOF {
			break
		}
		if err != nil {
			return tr, err
		}
		if len(event) != 2 {
			return tr, errors.New("trace event fields differ")
		}
		if tr.Required == nil && event["kind"] != "required" {
			return tr, errors.New("trace required event missing")
		}
		switch event["kind"] {
		case "required":
			required, ok := event["value"].([]any)
			if !ok || tr.Required != nil {
				return tr, errors.New("invalid or repeated required event")
			}
			tr.Required = required
		case "served":
			if object(event["value"]) == nil {
				return tr, errors.New("trace served key missing")
			}
			tr.Served = append(tr.Served, event["value"])
		case "home":
			home, ok := event["value"].(string)
			if !ok || home == "" {
				return tr, errors.New("trace home missing")
			}
			tr.Homes = append(tr.Homes, home)
		default:
			return tr, errors.New("unknown trace event")
		}
	}
	if tr.Required == nil {
		return tr, errors.New("trace required event missing")
	}
	return tr, nil
}

func placeholder(v any) bool { _, ok := object(v)["placeholder"]; return ok }
func discovery(k map[string]any) bool {
	words := list(k["words"])
	if k["home"] != "empty" || k["computation"] != nil || len(words) == 0 {
		return false
	}
	return words[len(words)-1] == "--help"
}

// levelTwoPlaceholder independently follows QS5's dependency bullets. Command
// selection uses the longest declared prefix, including the bare binary.
func levelTwoPlaceholder(subject, pre map[string]any) bool {
	if placeholder(subject["binary"]) {
		return true
	}
	probe := pre["probe"]
	if probe == "discovery" {
		return placeholder(subject["declared_count"])
	}
	if placeholder(subject["seed"]) || placeholder(subject["store"]) {
		return true
	}
	blockedCommand := func(words []any) bool {
		longest := -1
		blocked := false
		for _, value := range list(subject["commands"]) {
			command := object(value)
			prefix := list(command["words"])
			if len(prefix) <= len(words) && len(prefix) > longest && (len(prefix) == 0 || reflect.DeepEqual(prefix, words[:len(prefix)])) {
				longest = len(prefix)
				blocked = placeholder(command["class"]) || placeholder(command["writes"])
			}
		}
		return blocked
	}
	if probe == "planted_secret" {
		if placeholder(subject["secret"]) {
			return true
		}
		for _, value := range list(object(subject["secret"])["commands"]) {
			if blockedCommand(list(value)) {
				return true
			}
		}
		for _, value := range list(object(pre["parameters"])["commands"]) {
			if blockedCommand(list(object(value)["words"])) {
				return true
			}
		}
		return false
	}
	return blockedCommand(list(pre["words"]))
}

func hasPlaceholder(value any) bool {
	if placeholder(value) {
		return true
	}
	for _, v := range object(value) {
		if hasPlaceholder(v) {
			return true
		}
	}
	for _, v := range list(value) {
		if hasPlaceholder(v) {
			return true
		}
	}
	return false
}

func placeholderClass(subject, pre map[string]any, invocations []any) (string, error) {
	if levelTwoPlaceholder(subject, pre) {
		return "level_two", nil
	}
	trigger := hasPlaceholder(subject)
	// Discovery help requests identify discovered commands independently of
	// evidence's collector-chosen expected and observed fields (QE19).
	for _, value := range invocations {
		key := object(value)
		if !discovery(key) {
			continue
		}
		words := list(key["words"])
		words = words[:len(words)-1]
		if len(words) == 0 {
			continue
		}
		declared := false
		for _, command := range list(subject["commands"]) {
			declared = declared || reflect.DeepEqual(words, list(object(command)["words"]))
		}
		trigger = trigger || !declared
	}
	// Rulebook §6 lists precisely these five structurally excludable dimensions.
	dimension, ok := pre["dimension"].(string)
	excluded := slices.Contains([]string{"bounding", "idempotency", "destructive-guard", "side-effect-integrity", "flag-conformance"}, dimension)
	if trigger && ok && dimension != "" && !excluded {
		return "override", nil
	}
	return "", errors.New("placeholder check has neither level-two dependency nor override")
}

func checkTrace(c caseassembly.Case, subject, input, preimages map[string]any, tr trace) error {
	if tr.Required == nil {
		return errors.New("trace not read")
	}
	if c.Kind == "derivation-only" && (len(tr.Served) > 0 || len(tr.Homes) > 0) {
		return errors.New("derivation collected")
	}
	for _, value := range list(input["evidence"]) {
		ev := object(value)
		if ev["reason"] == "placeholder_unfilled" {
			if _, err := placeholderClass(subject, object(preimages[fmt.Sprint(ev["check_id"])]), c.Invocations); err != nil {
				return err
			}
		}
	}
	host := c.Input.Host
	noRun := object(host["confinement"])["applied"] == false || host["self_test"] == "failed" || host["private_copy"] == "changed" || placeholder(subject["binary"])
	if noRun && (len(tr.Served) > 0 || len(tr.Homes) > 0) {
		return errors.New("collection before host/subject guard")
	}
	allowed := map[string]bool{}
	for _, key := range c.Invocations {
		allowed[spelling(key)] = true
	}
	for _, key := range tr.Served {
		if !allowed[spelling(key)] {
			return errors.New("served key outside invocations")
		}
		k := object(key)
		if (placeholder(subject["seed"]) || placeholder(subject["store"])) && !discovery(k) {
			return errors.New("non-discovery invocation with placeholder seed/store")
		}
		if computationCaptureCut(k, subject, input, preimages) {
			return errors.New("computation capture was cut")
		}
		// A key shared with an executable check remains allowed. Keys belonging
		// solely to checks stopped before collection must not reach the source.
		matched, runnable := false, false
		for _, e := range list(input["evidence"]) {
			ev := object(e)
			id, ok := ev["check_id"].(string)
			if !ok {
				return errors.New("evidence without check_id")
			}
			p := object(preimages[id])
			match := false
			for _, expectedKey := range preimageKeys(p) {
				if reflect.DeepEqual(k, expectedKey) {
					match = true
				}
			}
			if match {
				matched = true
				if ev["reason"] != "earlier_step_failed" && (ev["reason"] != "placeholder_unfilled" || !levelTwoPlaceholder(subject, p)) {
					runnable = true
				}
			}
		}
		if servedOnlyForBlockedChecks(matched, runnable, k) {
			return errors.New("served only for blocked checks")
		}
	}
	return nil
}

// Discovery requests remain allowed independently of blocked-check evidence.
func servedOnlyForBlockedChecks(matched, runnable bool, key map[string]any) bool {
	return matched && !runnable && !discovery(key)
}

// Captures read stdout of an earlier step in the same runbook (QA4).
func computationCaptureCut(key, subject, input, preimages map[string]any) bool {
	if key["computation"] == nil {
		return false
	}
	for _, bookValue := range list(subject["runbooks"]) {
		book := object(bookValue)
		if book["name"] != key["runbook"] {
			continue
		}
		for _, stepValue := range list(book["steps"]) {
			step := object(stepValue)
			if step["name"] != key["step"] {
				continue
			}
			for _, checkValue := range list(step["checks"]) {
				check := object(checkValue)
				for _, source := range []map[string]any{object(check["expected"]), object(check["precondition"])} {
					computation := object(source["computation"])
					if !reflect.DeepEqual(computation["argv"], key["computation"]) {
						continue
					}
					for _, capture := range list(computation["captures"]) {
						for _, evValue := range list(input["evidence"]) {
							ev := object(evValue)
							id, ok := ev["check_id"].(string)
							if !ok {
								continue
							}
							pre := object(preimages[id])
							if pre["runbook"] == book["name"] && pre["step"] == capture && object(object(ev["observed"])["stdout"])["cut"] == true {
								return true
							}
						}
					}
				}
			}
		}
	}
	return false
}

type result struct {
	Exit     int
	Record   map[string]any
	Baseline map[string]any
	Rule     string
	Trace    trace
}

func compareResult(c caseassembly.Case, want map[string]any, r result) error {
	n, ok := c.Expect["exit_code"].(json.Number)
	expected, err := n.Int64()
	if !ok || err != nil || int64(r.Exit) != expected {
		return errors.New("exit code differs")
	}
	if c.Kind == "derivation-only" {
		if r.Record != nil {
			return errors.New("derivation stored a record")
		}
		if r.Rule != c.Expect["rule"] {
			return errors.New("rejection rule differs")
		}
		return nil
	}
	if r.Record == nil {
		return errors.New("stored record missing")
	}
	if r.Record["judge_rejected"] != false {
		return errors.New("judge rejected or missing rejection flag")
	}
	if err := compareInput(object(r.Record["judge_input"]), want); err != nil {
		return err
	}
	output, present := r.Record["judge_output"]
	expectedOutput, expectedPresent := c.Expect["judge_output"]
	if !present || !expectedPresent || !reflect.DeepEqual(output, expectedOutput) {
		return errors.New("judge output differs")
	}
	return nil
}

func runGrade(binary string, c caseassembly.Case, l caseassembly.Layout) (result, error) {
	host, err := json.Marshal(c.Input.Host)
	if err != nil {
		return result{}, err
	}
	hp := filepath.Join(l.Work, "host.json")
	if err := os.WriteFile(hp, host, 0600); err != nil {
		return result{}, err
	}
	recording, err := filepath.Abs(filepath.Join(kit, "behaviour", c.Input.Form+".json"))
	if err != nil {
		return result{}, err
	}
	args := []string{c.Input.Request, filepath.Join(l.Subject, c.Input.SubjectFile), "--verifier", "scripted", "--host", hp, "--behaviour", recording}
	if c.Input.Request == "grade" {
		args = append(args, "--scope", strings.Join(c.Input.Scope, ","))
	}
	if l.Baseline != "" {
		args = append(args, "--baseline", l.Baseline)
	}
	cmd := exec.Command(binary, args...)
	cmd.Dir = l.Work
	// Both platform config locations stay inside the fixture's ambient home.
	if err := os.MkdirAll(filepath.Join(l.Home, "Library", "Application Support"), 0700); err != nil {
		return result{}, err
	}
	cmd.Env = append(os.Environ(), "HOME="+l.Home, "XDG_CONFIG_HOME="+l.Config, scripted.TraceEnv+"="+l.Trace)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	_, runErr := cmd.Output()
	r := result{}
	if runErr != nil {
		var exit *exec.ExitError
		if !errors.As(runErr, &exit) {
			return r, runErr
		}
		r.Exit = exit.ExitCode()
	}
	match := regexp.MustCompile(`(?m)^pql: rejected: (Q[A-Z]+[0-9]+): [^\n]+$`).FindStringSubmatch(stderr.String())
	if len(match) > 0 {
		r.Rule = match[1]
	}
	r.Trace, err = readTrace(l.Trace)
	if errors.Is(err, os.ErrNotExist) && r.Rule != "" {
		r.Trace = trace{Required: []any{}, Served: []any{}, Homes: []string{}}
		err = nil
	}
	if err != nil {
		return r, err
	}
	err = filepath.WalkDir(l.Home, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		if r.Record != nil {
			return errors.New("multiple stored records")
		}
		return caseassembly.Read(path, &r.Record)
	})
	if err != nil {
		return r, err
	}
	err = filepath.WalkDir(l.Config, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		if r.Record != nil {
			return errors.New("multiple stored records")
		}
		return caseassembly.Read(path, &r.Record)
	})
	return r, err
}

func ratchet(pending bool, failure error) error {
	if pending && failure == nil {
		return errors.New("pending case passed; remove it from pending")
	}
	if !pending && failure != nil {
		return fmt.Errorf("required case failed: %w", failure)
	}
	return nil
}
func exportCopies(directory, id string, record, baseline map[string]any) error {
	if directory == "" {
		return nil
	}
	if !filepath.IsAbs(directory) {
		return errors.New("copy directory must be absolute")
	}
	// Baselines are validated when the real path stores one. Scripted grades store records only.
	for suffix, document := range map[string]map[string]any{".record.json": record, ".baseline.json": baseline} {
		if document == nil && suffix == ".baseline.json" {
			continue
		}
		if document == nil {
			return fmt.Errorf("missing export %s", suffix)
		}
		data, err := json.Marshal(document)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(directory, id+suffix), data, 0600); err != nil {
			return err
		}
	}
	return nil
}

func pendingSet(data string) map[string]bool {
	m := map[string]bool{}
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			m[line] = true
		}
	}
	return m
}

func TestCorpus(t *testing.T) {
	// QT6, QE19, QO1: execute the real command and independently compare its record and trace.
	// QE3, QE7: every judge-checked required set matches the trace (KTD2.7).
	binary := filepath.Join(t.TempDir(), "pql")
	build := exec.Command("go", "build", "-tags", "scripted", "-o", binary, "../../cmd/pql")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %s: %v", output, err)
	}
	predictions := readPredictions(t)

	entries, err := os.ReadDir(filepath.Join(kit, "cases"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 152 {
		t.Fatalf("case count %d, want 152", len(entries))
	}
	data, err := os.ReadFile("pending.txt")
	if err != nil {
		t.Fatal(err)
	}
	pending := pendingSet(string(data))
	found := map[string]bool{}
	var real []string
	for _, entry := range entries {
		source := filepath.Join(kit, "cases", entry.Name())
		c, want, subject, preimages := fixture(t, entry.Name())
		if c.ID != entry.Name() {
			t.Fatalf("case id mismatch: %s", entry.Name())
		}
		found[c.ID] = true
		if c.Input.Verifier == "real" {
			if err := realPending(c.ID, pending); err != nil {
				t.Fatal(err)
			}
			real = append(real, c.ID)
			continue
		}
		if c.Input.Verifier != "scripted" {
			t.Fatalf("unknown verifier %s", c.Input.Verifier)
		}
		t.Run(strings.ReplaceAll(c.ID, "-", "_"), func(t *testing.T) {
			layout := caseassembly.Assemble(t, kit, source, c)
			r, failure := runGrade(binary, c, layout)
			if failure != nil {
				t.Fatal(failure)
			}
			if pending[c.ID] {
				if err := checkEnding(predictions[c.ID].Ending, r.Exit, r.Rule); err != nil {
					t.Fatal(err)
				}
			}
			if err := checkTrace(c, subject, want, preimages, r.Trace); err != nil {
				t.Fatal(err)
			}
			if c.Kind == "judge-checked" {
				if err := checkRequired(r.Trace, want); err != nil {
					t.Fatal(err)
				}
			}
			if failure == nil {
				failure = compareResult(c, want, r)
				if failure == nil {
					failure = checkTrace(c, subject, want, preimages, r.Trace)
				}
				if failure == nil && c.Kind == "judge-checked" {
					failure = rejudge(binary, r.Record)
					if failure == nil {
						failure = exportCopies(os.Getenv("PQL_CORPUS_COPIES"), c.ID, r.Record, r.Baseline)
					}
				}
			}
			if err := ratchet(pending[c.ID], failure); err != nil {
				t.Fatal(err)
			}
			t.Logf("result %s exit=%d rule=%s pending=%v required=%d", c.ID, r.Exit, r.Rule, pending[c.ID], len(r.Trace.Required))
			if pending[c.ID] {
				t.Logf("pending: exit=%d rule=%s comparison=%v", r.Exit, r.Rule, failure)
			}
		})
	}
	if !reflect.DeepEqual(real, []string{"case-069", "case-070", "case-143"}) {
		t.Fatalf("real exclusions %v", real)
	}
	for id := range pending {
		if !found[id] {
			t.Fatalf("unknown pending case %s", id)
		}
	}
}

// KTD2.4: PR6 must invoke pql judge on the stored input and compare its output.
func rejudge(binary string, record map[string]any) error {
	input, err := json.Marshal(record["judge_input"])
	if err != nil {
		return err
	}
	cmd := exec.Command(binary, "judge")
	cmd.Stdin = bytes.NewReader(input)
	output, runErr := cmd.Output()
	code := 0
	if runErr != nil {
		var exit *exec.ExitError
		if !errors.As(runErr, &exit) {
			return runErr
		}
		code = exit.ExitCode()
	}
	decoder := json.NewDecoder(bytes.NewReader(output))
	decoder.UseNumber()
	var got any
	if err := decoder.Decode(&got); err != nil {
		return err
	}
	if !reflect.DeepEqual(got, record["judge_output"]) {
		return errors.New("re-judged output differs")
	}
	n, ok := object(record["judge_output"])["exit_code"].(json.Number)
	expected, err := n.Int64()
	if !ok || err != nil || int64(code) != expected {
		return errors.New("re-judged exit code differs")
	}
	return nil
}

func realPending(id string, pending map[string]bool) error {
	if !pending[id] {
		return fmt.Errorf("real-verifier case must remain pending: %s", id)
	}
	return nil
}

// preimageKeys follows QT6's full key, including environment names in byte order.
func preimageKeys(p map[string]any) []map[string]any {
	home := "seed"
	if p["runbook"] != nil {
		home = "runbook"
	} else if p["probe"] == "planted_secret" {
		home = "planted"
	}
	commands := []any{p}
	if p["probe"] == "planted_secret" {
		commands = list(object(p["parameters"])["commands"])
	}
	var keys []map[string]any
	for _, command := range commands {
		c := object(command)
		env := []string{}
		for name := range object(c["env"]) {
			env = append(env, name)
		}
		slices.Sort(env)
		names := []any{}
		for _, name := range env {
			names = append(names, name)
		}
		keys = append(keys, map[string]any{"words": c["words"], "offline_args": c["offline_args"], "env": names, "runbook": p["runbook"], "step": p["step"], "computation": nil, "home": home})
	}
	expected := object(p["expected"])
	for _, source := range []map[string]any{expected, object(expected["precondition"])} {
		if argv := object(source["computation"])["argv"]; argv != nil {
			keys = append(keys, map[string]any{"words": nil, "offline_args": []any{}, "env": []any{}, "runbook": p["runbook"], "step": p["step"], "computation": argv, "home": "scratch"})
		}
	}
	return keys
}

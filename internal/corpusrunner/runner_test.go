//go:build scripted

// Package corpusrunner tests collection independently of production decoders.
package corpusrunner

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/caseassembly"
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
func checkTrace(c caseassembly.Case, subject, input, preimages map[string]any, tr trace) error {
	if tr.Required == nil {
		return errors.New("trace not read")
	}
	if c.Kind == "derivation-only" && (len(tr.Served) > 0 || len(tr.Homes) > 0) {
		return errors.New("derivation collected")
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
		// solely to stopped/placeholder checks must not reach the source.
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
				if ev["reason"] != "earlier_step_failed" && ev["reason"] != "placeholder_unfilled" {
					runnable = true
				}
			}
		}
		if matched && !runnable && !discovery(k) {
			return errors.New("served only for blocked checks")
		}
	}
	return nil
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

// runGradeNotBuilt is the only missing integration. PR6 replaces this with
// grade/anchor, stored-file reads, trace reads, re-judging, and export copies.
// Returning an error is intentional: a case removed from pending fails closed.
func runGradeNotBuilt(_ caseassembly.Case, _ caseassembly.Layout) (result, error) {
	return result{}, errors.New("grade/anchor not built")
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
		var c caseassembly.Case
		if err := caseassembly.Read(filepath.Join(source, "case.json"), &c); err != nil {
			t.Fatal(err)
		}
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
		t.Run(c.ID, func(t *testing.T) {
			layout := caseassembly.Assemble(t, kit, source, c)
			r, failure := runGradeNotBuilt(c, layout)
			if failure == nil {
				var want, subject, preimages map[string]any
				if err := caseassembly.Read(filepath.Join(source, c.Input.SubjectFile), &subject); err != nil {
					t.Fatal(err)
				}
				if c.Kind == "judge-checked" {
					if err := caseassembly.Read(filepath.Join(source, "input.json"), &want); err != nil {
						t.Fatal(err)
					}
					if err := caseassembly.Read(filepath.Join(source, "preimages.json"), &preimages); err != nil {
						t.Fatal(err)
					}
				}
				failure = compareResult(c, want, r)
				if failure == nil {
					failure = checkTrace(c, subject, want, preimages, r.Trace)
				}
				if failure == nil && c.Kind == "judge-checked" {
					failure = rejudgeNotBuilt(r.Record)
					if failure == nil {
						failure = exportCopies(os.Getenv("PQL_CORPUS_COPIES"), c.ID, r.Record, r.Baseline)
					}
				}
			}
			if err := ratchet(pending[c.ID], failure); err != nil {
				t.Fatal(err)
			}
			if pending[c.ID] {
				t.Logf("pending: %v", failure)
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
func rejudgeNotBuilt(_ map[string]any) error { return errors.New("pql judge re-judge not built") }

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

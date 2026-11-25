//go:build scripted

package corpusrunner

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/caseassembly"
)

func read(t *testing.T, path string, v any) {
	t.Helper()
	if err := caseassembly.Read(path, v); err != nil {
		t.Fatal(err)
	}
}
func fixture(t *testing.T, id string) (caseassembly.Case, map[string]any, map[string]any, map[string]any) {
	t.Helper()
	source := filepath.Join(kit, "cases", id)
	var c caseassembly.Case
	var input, subject, pre map[string]any
	read(t, filepath.Join(source, "case.json"), &c)
	read(t, filepath.Join(source, c.Input.SubjectFile), &subject)
	if c.Kind == "judge-checked" {
		read(t, filepath.Join(source, "input.json"), &input)
		read(t, filepath.Join(source, "preimages.json"), &pre)
	}
	return c, input, subject, pre
}
func clone(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.json")
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	read(t, path, &out)
	return out
}
func assertError(t *testing.T, err error, want string) {
	t.Helper()
	got := ""
	if err != nil {
		got = err.Error()
	}
	if got != want {
		t.Fatalf("error %q, want %q", got, want)
	}
}
func TestComparatorMutations(t *testing.T) {
	_, want, _, _ := fixture(t, "case-001")
	assertError(t, compareInput(clone(t, want), want), "")
	for _, field := range []string{"check_id", "outcome", "reason", "failure_kind", "target_size", "precondition_observed", "tier"} {
		t.Run(field, func(t *testing.T) {
			got := clone(t, want)
			object(list(got["evidence"])[0])[field] = "mutated"
			expected := "field " + field + " differs or is absent"
			if field == "check_id" {
				expected = "missing evidence"
			}
			assertError(t, compareInput(got, want), expected)
		})
	}
	for _, field := range []string{"kind", "id", "tier"} {
		t.Run("target/"+field, func(t *testing.T) {
			got := clone(t, want)
			object(list(object(list(got["evidence"])[0])["targets"])[0])[field] = "mutated"
			assertError(t, compareInput(got, want), "target fields differ")
		})
	}
	for _, mutation := range []string{"required", "extra", "missing", "renamed", "absent-both", "target-absent-both", "duplicate"} {
		t.Run(mutation, func(t *testing.T) {
			got := clone(t, want)
			expected := clone(t, want)
			message := "evidence count differs or absent"
			switch mutation {
			case "required":
				got["required"] = []any{}
				message = "field required differs or is absent"
			case "extra":
				got["evidence"] = append(list(got["evidence"]), list(got["evidence"])[0])
			case "missing":
				got["evidence"] = list(got["evidence"])[1:]
			case "renamed":
				got["renamed"] = got["required"]
				delete(got, "required")
				message = "field required differs or is absent"
			case "absent-both":
				delete(object(list(got["evidence"])[0]), "reason")
				delete(object(list(expected["evidence"])[0]), "reason")
				message = "field reason differs or is absent"
			case "target-absent-both":
				delete(object(list(object(list(got["evidence"])[0])["targets"])[0]), "tier")
				delete(object(list(object(list(expected["evidence"])[0])["targets"])[0]), "tier")
				message = "target field tier absent"
			case "duplicate":
				list(got["evidence"])[1] = list(got["evidence"])[0]
				message = "missing or repeated check_id"
			}
			assertError(t, compareInput(got, expected), message)
		})
	}
	for _, variation := range []string{"evidence-order", "content", "top-level-lists"} {
		t.Run(variation, func(t *testing.T) {
			got := clone(t, want)
			switch variation {
			case "evidence-order":
				a := list(got["evidence"])
				a[0], a[len(a)-1] = a[len(a)-1], a[0]
			case "content":
				e := object(list(got["evidence"])[0])
				e["expected"] = "collector choice"
				e["observed"] = nil
			case "top-level-lists":
				for k, v := range got {
					if a, ok := v.([]any); ok && k != "evidence" && len(a) > 1 {
						a[0], a[len(a)-1] = a[len(a)-1], a[0]
					}
				}
			}
			assertError(t, compareInput(got, want), "")
		})
	}
	// Duplicate list elements are not collapsed into sets.
	assertError(t, sameFields(map[string]any{"scope": []any{"a", "a"}}, map[string]any{"scope": []any{"a", "b"}}, []string{"scope"}), "field scope differs or is absent")
}
func TestResultAndRatchet(t *testing.T) {
	c, input, _, _ := fixture(t, "case-001")
	r := result{Exit: c.Expect["exit_code"], Record: map[string]any{"judge_input": input, "judge_output": c.Expect["judge_output"], "judge_rejected": false}}
	assertError(t, compareResult(c, input, r), "")
	r.Record = nil
	assertError(t, compareResult(c, input, r), "stored record missing")
	d, _, _, _ := fixture(t, "case-100")
	r = result{Exit: d.Expect["exit_code"], Rule: "wrong"}
	assertError(t, compareResult(d, nil, r), "rejection rule differs")
	rule, ok := d.Expect["rule"].(string)
	if !ok {
		t.Fatal("fixture rule missing")
	}
	r.Rule = rule
	assertError(t, compareResult(d, nil, r), "")
	r.Record = map[string]any{}
	assertError(t, compareResult(d, nil, r), "derivation stored a record")
	r.Record = nil
	r.Exit = json.Number("70")
	assertError(t, compareResult(d, nil, r), "exit code differs")
	for _, row := range []struct {
		pending bool
		failure error
		message string
	}{
		{true, errors.New("not built"), ""}, {false, nil, ""}, {true, nil, "pending case passed; remove it from pending"}, {false, errors.New("not built"), "required case failed: not built"},
	} {
		assertError(t, ratchet(row.pending, row.failure), row.message)
	}
}
func TestTraceGuards(t *testing.T) {
	c, input, subject, pre := fixture(t, "case-001")
	tr := trace{Required: []any{}, Served: []any{}, Homes: []string{}}
	assertError(t, checkTrace(c, subject, input, pre, tr), "")
	tr.Served = []any{map[string]any{"words": []any{"unknown"}}}
	assertError(t, checkTrace(c, subject, input, pre, tr), "served key outside invocations")
	for _, id := range []string{"case-003", "case-004", "case-060"} {
		t.Run(id, func(t *testing.T) {
			c, in, s, p := fixture(t, id)
			tr := trace{Served: []any{c.Invocations[0]}}
			assertError(t, checkTrace(c, s, in, p, tr), "collection before host/subject guard")
		})
	}
	for _, id := range []string{"case-061", "case-062"} {
		t.Run(id, func(t *testing.T) {
			c, in, s, p := fixture(t, id)
			var ordinary, help any
			for _, k := range c.Invocations {
				if discovery(object(k)) {
					help = k
				} else {
					ordinary = k
				}
			}
			if ordinary == nil || help == nil {
				t.Fatal("fixture lacks keys")
			}
			assertError(t, checkTrace(c, s, in, p, trace{Served: []any{ordinary}}), "non-discovery invocation with placeholder seed/store")
			assertError(t, checkTrace(c, s, in, p, trace{Served: []any{help}}), "")
		})
	}
	d, _, s, p := fixture(t, "case-100")
	assertError(t, checkTrace(d, s, nil, p, trace{Homes: []string{"made"}}), "derivation collected")
	// A fixture key associated only with a stopped check must not be served.
	key := map[string]any{"words": []any{"list"}, "runbook": "book", "step": "second", "computation": nil}
	c.Invocations = []any{key}
	for _, reason := range []string{"earlier_step_failed", "placeholder_unfilled"} {
		input = map[string]any{"evidence": []any{map[string]any{"check_id": "id", "reason": reason}}}
		pre = map[string]any{"id": map[string]any{"words": []any{"list"}, "runbook": "book", "step": "second"}}
		assertError(t, checkTrace(c, subject, input, pre, trace{Served: []any{key}}), "served only for blocked checks")
	}
}
func TestIndependentReaders(t *testing.T) {
	for _, row := range []struct {
		data  string
		valid bool
	}{
		{`{"kind":"required","value":[]}`, true},
		{`{"kind":"required","value":[]} {"kind":"served","value":{"words":[]}} {"kind":"home","value":"made"}`, true},
		{`{"kind":"home","value":"made"}`, false},
		{`{"kind":"required","value":[]} {"kind":"required","value":[]}`, false},
		{`{"kind":"required","value":[]} {"kind":"unknown","value":[]}`, false},
		{`{"kind":"required","value":[]} {"kind":"served","value":null}`, false},
		{`{"kind":"required","value":[]} {"kind":"home","value":null}`, false},
		{``, false},
		{`{"kind":"required","value":[]} {}`, false},
		{`{`, false},
	} {
		path := filepath.Join(t.TempDir(), "trace.json")
		if err := os.WriteFile(path, []byte(row.data), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := readTrace(path)
		if (err == nil) != row.valid {
			t.Fatalf("trace %s: %v", row.data, err)
		}
	}
	tracePath := filepath.Join(t.TempDir(), "trace.json")
	if err := os.WriteFile(tracePath, []byte(`{"kind":"required","value":[9007199254740993]} {"kind":"served","value":{"words":["list"]}} {"kind":"home","value":"made"}`), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := readTrace(tracePath)
	if err != nil {
		t.Fatal(err)
	}
	want := trace{Required: []any{json.Number("9007199254740993")}, Served: []any{map[string]any{"words": []any{"list"}}}, Homes: []string{"made"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("trace %+v want %+v", got, want)
	}
	path := filepath.Join(t.TempDir(), "number.json")
	if err := os.WriteFile(path, []byte(`{"n":9007199254740993}`), 0600); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	read(t, path, &m)
	if !reflect.DeepEqual(m["n"], json.Number("9007199254740993")) {
		t.Fatalf("number lost: %v", m)
	}
	if err := caseassembly.Read(filepath.Join(t.TempDir(), "case.json"), &m); err == nil {
		t.Fatal("empty case directory accepted")
	}
}
func TestPythonControls(t *testing.T) {
	cmd := exec.Command("python3", "-I", "../../scripts/test_corpus_controls.py")
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Python controls: %v\n%s", err, out)
	}
}

func TestRecordOutputAndExports(t *testing.T) {
	c, input, _, _ := fixture(t, "case-001")
	for _, row := range []struct {
		output   any
		rejected any
		message  string
	}{
		{nil, false, "judge output differs"},
		{c.Expect["judge_output"], true, "judge rejected or missing rejection flag"},
		{c.Expect["judge_output"], nil, "judge rejected or missing rejection flag"},
	} {
		r := result{Exit: c.Expect["exit_code"], Record: map[string]any{"judge_input": input, "judge_output": row.output, "judge_rejected": row.rejected}}
		assertError(t, compareResult(c, input, r), row.message)
	}
	dir := t.TempDir()
	record := map[string]any{"record_version": "fixture"}
	baseline := map[string]any{"baseline_version": "fixture"}
	assertError(t, exportCopies(dir, "case-001", record, baseline), "")
	for suffix, want := range map[string]map[string]any{".record.json": record, ".baseline.json": baseline} {
		path := filepath.Join(dir, "case-001"+suffix)
		var got map[string]any
		read(t, path, &got)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("export %s: %v want %v", suffix, got, want)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("export mode %v", info.Mode())
		}
	}
	assertError(t, exportCopies(dir, "case-001", nil, baseline), "missing export .record.json")
	assertError(t, exportCopies("relative", "case-001", record, baseline), "copy directory must be absolute")
	// A deleted stored file is a reading failure, before any comparison can pass.
	path := filepath.Join(t.TempDir(), "deleted.record.json")
	if err := os.WriteFile(path, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := caseassembly.Read(path, &got); err == nil {
		t.Fatal("deleted record accepted")
	}
}

func TestCutComputationTrace(t *testing.T) {
	c, input, subject, pre := fixture(t, "case-139")
	var computation any
	for _, key := range c.Invocations {
		if object(key)["computation"] != nil {
			computation = key
			break
		}
	}
	if computation == nil {
		t.Fatal("fixture lacks computation")
	}
	tr := trace{Served: []any{computation}}
	assertError(t, checkTrace(c, subject, input, pre, tr), "")
	changed := false
	for _, value := range list(input["evidence"]) {
		ev := object(value)
		id, ok := ev["check_id"].(string)
		if !ok {
			t.Fatal("fixture id missing")
		}
		p := object(pre[id])
		if p["runbook"] == "self agreement" && p["step"] == "first" {
			object(object(ev["observed"])["stdout"])["cut"] = true
			changed = true
		}
	}
	if !changed {
		t.Fatal("fixture lacks captured stdout")
	}
	assertError(t, checkTrace(c, subject, input, pre, tr), "computation capture was cut")
}

func TestEachHostGuard(t *testing.T) {
	for _, name := range []string{"confinement", "self_test", "private_copy", "binary"} {
		t.Run(name, func(t *testing.T) {
			c, input, subject, pre := fixture(t, "case-001")
			switch name {
			case "confinement":
				object(c.Input.Host["confinement"])["applied"] = false
			case "self_test":
				c.Input.Host["self_test"] = "failed"
			case "private_copy":
				c.Input.Host["private_copy"] = "changed"
			case "binary":
				subject["binary"] = map[string]any{"placeholder": "fill"}
			}
			assertError(t, checkTrace(c, subject, input, pre, trace{}), "")
			assertError(t, checkTrace(c, subject, input, pre, trace{Homes: []string{"made"}}), "collection before host/subject guard")
			assertError(t, checkTrace(c, subject, input, pre, trace{Served: []any{c.Invocations[0]}}), "collection before host/subject guard")
		})
	}
}

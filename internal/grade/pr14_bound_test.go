package grade

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/record"
	"github.com/vinnyp/press-quality-lab/internal/scorecard"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func TestPR14BoundScorecard(t *testing.T) {
	// QE10, QE14, QD3, QS5, QA4: exhausting the evidence budget loses stored names but retains scorecard names and the stored verdict.
	for _, large := range []bool{false, true} {
		padding := "small"
		if large {
			padding = strings.Repeat("x", 4<<20)
		}
		names := strictjson.Array{}
		for i := 0; i < 513; i++ {
			names = append(names, strictjson.Array{strings.Repeat("n", i+1)})
		}
		observed := strictjson.Object{"probe": "discovery", "discovery": strictjson.Object{"ran": true, "found_count": 513, "declared_count": 0, "found_names": names[:512], "missing_names": strictjson.Array{}, "undeclared_names": names, "undeclared_count": 513, "found_cut": true, "undeclared_cut": false, "request_bound_reached": false, "depth_bound_reached": false}, "placeholders": strictjson.Array{"/binary"}, "invocations": strictjson.Array{strictjson.Object{"computation": strictjson.Array{"failed_computation"}}}, "streams": strictjson.Array{strictjson.Object{"stderr": strictjson.Object{"head": "computation problem", "cut": false}}}}
		input, display, e := record.Build(record.Input{Plan: plan.Plan{Checks: []plan.Check{{ID: "a"}, {ID: "z"}}}, Results: map[string]record.Resolution{"a": {Result: decide.Result{Outcome: "held"}, Expected: padding}, "z": {Result: decide.Result{Outcome: decide.NotObserved, Reason: "computation_failed"}, Observed: observed}}})
		if e != nil {
			t.Fatal(e)
		}
		stored := subjectObject(subjectArray(input["evidence"])[1])["observed"]
		_, digest := subjectObject(stored)["sha256"]
		if digest != large {
			t.Fatal("bound did not trigger", large)
		}
		output := strictjson.Object{"verdict": strictjson.Object{"plain": "known_defect", "sub_label": "known-defects"}, "counts": strictjson.Object{"xfail": 31, "xpass": 17}, "checks": strictjson.Array{strictjson.Object{"id": "z", "result": "unobserved", "reason": "computation_failed"}}}
		doc := strictjson.Object{"request": "grade", "exit_code": 3, "judge_input": input, "judge_output": output}
		var b bytes.Buffer
		if e := scorecard.Render(&b, doc, display, Escape, nil); e != nil {
			t.Fatal(e)
		}
		for _, want := range []string{"pql grade: known_defect (exit 3) (known-defects)", "counts: xfail=31 xpass=17", "placeholder: /binary", "failed_computation", "computation stderr: computation problem", "undeclared command: [\"" + strings.Repeat("n", 513) + "\"]"} {
			if !strings.Contains(b.String(), want) {
				t.Fatal("missing", want)
			}
		}
	}
}
func TestPR14MaskingOutputs(t *testing.T) {
	// QE18, QS7, QP7, QA4: mask C0 and Cf secrets before escaping; short values remain visible in every named field.
	for _, value := range []string{"synthetic\x01value", "synthetic\u200dvalue", "short"} {
		masker := record.NewMasker(nil, []string{value})
		ob := strictjson.Object{"probe": value, "store": value, "placeholders": strictjson.Array{value}, "short_env_names": strictjson.Array{"MODE"}, "discovery": strictjson.Object{"ran": true, "found_names": strictjson.Array{strictjson.Array{value}}, "missing_names": strictjson.Array{strictjson.Array{value}}, "undeclared_names": strictjson.Array{strictjson.Array{value}}}, "invocations": strictjson.Array{strictjson.Object{"words": strictjson.Array{value}, "runbook": value, "step": value, "computation": strictjson.Array{value}}}, "streams": strictjson.Array{strictjson.Object{"stderr": strictjson.Object{"head": value}}}}
		input, display, e := record.Build(record.Input{Plan: plan.Plan{Checks: []plan.Check{{ID: "c"}}}, Results: map[string]record.Resolution{"c": {Result: decide.Result{Outcome: decide.NotObserved, Reason: "computation_failed"}, Observed: ob}}, Masker: masker})
		if e != nil {
			t.Fatal(e)
		}
		_, offline, book := record.Interim(strictjson.Object{"offline": strictjson.Array{strictjson.Object{"applies_to": strictjson.Array{strictjson.Array{value}}, "args": strictjson.Array{value}, "env": strictjson.Object{"MODE": value}}}, "bookkeeping": strictjson.Array{value}}, nil, masker)
		judgeOutput := strictjson.Object{"checks": strictjson.Array{strictjson.Object{"id": "c", "result": "unobserved"}}}
		doc := strictjson.Object{"request": "grade", "exit_code": 4, "judge_input": input, "judge_output": judgeOutput, "offline_settings": offline, "bookkeeping_excluded": book}
		var b bytes.Buffer
		if e := scorecard.Render(&b, doc, display, Escape, nil); e != nil {
			t.Fatal(e)
		}
		want := value
		if len(value) >= 8 {
			want = strings.Repeat("*", utf8.RuneCountInString(value))
		}
		if subjectObject(display["c"].Observed)["store"] != want || subjectObject(subjectArray(offline)[0])["env_names"] == nil {
			t.Fatal("missing masked evidence")
		}

		maskedObserved := subjectObject(display["c"].Observed)
		invocation := subjectObject(subjectArray(maskedObserved["invocations"])[0])
		if invocation["runbook"] != want || invocation["step"] != want || maskedObserved["probe"] != want || !reflect.DeepEqual(maskedObserved["placeholders"], strictjson.Array{want}) {
			t.Fatal(maskedObserved)
		}
		for _, key := range []string{"found_names", "missing_names", "undeclared_names"} {
			if !reflect.DeepEqual(subjectObject(maskedObserved["discovery"])[key], strictjson.Array{strictjson.Array{want}}) {
				t.Fatal(key, maskedObserved)
			}
		}
		for _, line := range []string{
			"invocation: words=[\"" + want + "\"] runbook=" + want + " step=" + want + " computation=[\"" + want + "\"]",
			"computation stderr: " + want,
			"found command: [\"" + want + "\"]",
			"missing command: [\"" + want + "\"]",
			"undeclared command: [\"" + want + "\"]",
			"placeholder: " + want,
			"offline setting 1: applies_to=[[\"" + want + "\"]] args=[\"" + want + "\"] env_names=[\"MODE\"]",
			"bookkeeping excluded: " + want + "\n",
			"bookkeeping excluded: " + want + "-shm",
		} {
			if !strings.Contains(b.String(), line) {
				t.Fatal("masked field omitted", line)
			}
		}
		for _, rendered := range []string{b.String(), string(strictjson.Write(doc)), string(terminalJSON(strictjson.Write(doc)))} {
			if len(value) >= 8 && (strings.Contains(rendered, value) || strings.Contains(rendered, Escape(value)) || bytes.Contains([]byte(rendered), strictjson.AppendValue(nil, value))) {
				t.Fatal("secret survived masking")
			}
			if !strings.Contains(rendered, want) {
				t.Fatal("masked value absent")
			}
		}
	}
}

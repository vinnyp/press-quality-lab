package grade

import (
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/collect"
	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"github.com/vinnyp/press-quality-lab/internal/subject"
)

func TestAuthoredEvidence(t *testing.T) {
	// QA2, QE8, QS5: authored expected values and predicate fields survive, then overrides clear sizes and preconditions.
	size := int64(5)
	pre := true
	ch := collect.Check{Plan: plan.Check{Subject: plan.Object{"facts": plan.Object{"other": plan.Object{"placeholder": "supply fact"}}}, Preimage: plan.Object{"probe": nil, "dimension": "response-correctness"}}, Result: decide.Result{Expected: strictjson.Number("7"), TargetSize: &size, PreconditionObserved: &pre, Outcome: "held", Level: 8}, Invocations: []observe.Invocation{{Run: observe.Run{Stdout: []byte("value"), StdoutSeen: 5}}}}
	r, err := checkResolution(ch, nil)
	if err != nil || r.Expected != strictjson.Number("7") || r.TargetSize != size || r.PreconditionObserved != true {
		t.Fatal(r, err)
	}
	streams, ok := r.Observed.(plan.Array)
	if !ok || len(streams) != 1 {
		t.Fatal(r.Observed)
	}
	ch.Result = decide.FinalOverride(ch.Plan, ch.Result)
	r, err = checkResolution(ch, nil)
	if err != nil || r.Result.Outcome != "not_observed" || r.TargetSize != nil || r.PreconditionObserved != nil {
		t.Fatal(r, err)
	}
	// A stale predicate field must also be suppressed at the emission boundary.
	ch.Result.TargetSize = &size
	ch.Result.PreconditionObserved = &pre
	r, err = checkResolution(ch, nil)
	if err != nil || r.TargetSize != nil || r.PreconditionObserved != nil {
		t.Fatal(r, err)
	}
}

func TestRunbookMarkers(t *testing.T) {
	// QE11: two markers for one runbook retain both distinct dimensions through the grade path.
	o, d, saved := fixture(t)
	s, refusal := subject.Parse([]byte(validSubject))
	if refusal != nil {
		t.Fatal(refusal)
	}
	checks := plan.Array{}
	for _, dim := range []string{"response-correctness", "flag-conformance"} {
		checks = append(checks, plan.Object{"dimension": dim, "predicate": "stdout_equals", "path": nil, "expected": plan.Object{"literal": "value"}, "precondition": nil, "exit_codes": nil})
	}
	s["runbooks"] = plan.Array{plan.Object{"name": "book", "steps": plan.Array{plan.Object{"name": "first", "words": plan.Array{}, "budget_seconds": strictjson.Number("1"), "checks": checks}}}}
	want := plan.Array{plan.Object{"record": "book", "dimension": "response-correctness"}, plan.Object{"record": "book", "dimension": "flag-conformance"}}
	s["markers"] = want
	if err := os.WriteFile(o.SubjectFile, strictjson.Write(s), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(o, d, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(subjectObject(saved.JudgeInput)["markers"], want) {
		t.Fatalf("markers got %v want %v", subjectObject(saved.JudgeInput)["markers"], want)
	}
}

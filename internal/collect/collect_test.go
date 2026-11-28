package collect

import (
	"errors"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/rulebook"
	"github.com/vinnyp/press-quality-lab/internal/subject"
	"os"
	"testing"
)

type fake struct {
	controls     observe.Controls
	err          error
	calls        int
	observations int
}

func (s *fake) Controls() (observe.Controls, error) { s.calls++; return s.controls, s.err }
func (s *fake) Observe(observe.Key, observe.Spec) (observe.Run, error) {
	s.observations++
	return observe.Run{}, errors.New("unexpected invocation")
}
func TestPlanningBeforeCollection(t *testing.T) {
	// QE4: a baseline planning gap comes before controls or decide.
	s := &fake{}
	got, err := Build(plan.Input{Baseline: true}, s)
	var gap *plan.Gap
	if !errors.As(err, &gap) || gap.Rule != "QE4" || gap.Part != "" || got != nil || s.calls != 0 || s.observations != 0 {
		t.Fatalf("%+v %v %+v", got, err, s)
	}
}
func TestLoop(t *testing.T) {
	// QO1: sequence every planned check, without requesting invocation keys.
	b, err := os.ReadFile("../../docs/specs/conformance-collection-1.0/cases/case-004/subject.json")
	if err != nil {
		t.Fatal(err)
	}
	sub, parseErr := subject.Parse(b)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	// Supply the declared computation contents without starting any computation.
	files := map[string][]byte{}
	entries, err := os.ReadDir("../../docs/specs/conformance-collection-1.0/subjects/known-good/computations")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			data, err := os.ReadFile("../../docs/specs/conformance-collection-1.0/subjects/known-good/computations/" + entry.Name())
			if err != nil {
				t.Fatal(err)
			}
			files["computations/"+entry.Name()] = data
		}
	}
	s := &fake{controls: observe.Controls{ConfinementApplied: true, SystemNamed: true}}
	checks, err := Build(plan.Input{Subject: sub, Scopes: []rulebook.Scope{rulebook.ConformanceScope}, Files: files}, s)
	if err != nil || len(checks) != 29 || s.calls != 1 || s.observations != 0 {
		t.Fatalf("checks %d err %v source %+v", len(checks), err, s)
	}
	for _, ch := range checks {
		if ch.Plan.ID == "" || ch.Result.Level != 1 || ch.Result.Outcome != "not_observed" || ch.Result.Reason != "network_unverifiable" || ch.Result.Unbuilt != nil {
			t.Fatalf("%+v", ch)
		}
	}
}
func TestControlFault(t *testing.T) {
	// QT4: a control that could not run cannot produce observations.
	fault := errors.New("control fault")
	s := &fake{err: fault}
	got, err := Collect(plan.Plan{Checks: []plan.Check{{Preimage: plan.Object{"probe": "bare"}}}}, s, Options{RunDirectory: t.TempDir()})
	if !errors.Is(err, fault) || got != nil || s.calls != 1 || s.observations != 0 {
		t.Fatalf("%+v %v %+v", got, err, s)
	}
}

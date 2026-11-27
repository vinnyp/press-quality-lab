package confine_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/collect"
	"github.com/vinnyp/press-quality-lab/internal/confine"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/rulebook"
	"github.com/vinnyp/press-quality-lab/internal/subject"
)

type selfTestSource struct {
	controls     observe.Controls
	observations int
}

func (s *selfTestSource) Controls() (observe.Controls, error) { return s.controls, nil }
func (s *selfTestSource) Observe(observe.Key, observe.Spec) (observe.Run, error) {
	s.observations++
	return observe.Run{}, nil
}

func TestSelfTestFailedAllChecks(t *testing.T) {
	// QH9, QO1 and E-15: a failed clean program reaches level 1 for all 29
	// required checks, plus no-command checks, before any subject invocation.
	root := "../../docs/specs/conformance-collection-1.0"
	b, err := os.ReadFile(filepath.Join(root, "cases/case-004/subject.json"))
	if err != nil {
		t.Fatal(err)
	}
	sub, parseErr := subject.Parse(b)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	files := map[string][]byte{}
	entries, err := os.ReadDir(filepath.Join(root, "subjects/known-good/computations"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, "subjects/known-good/computations", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files["computations/"+entry.Name()] = data
	}
	p, err := plan.Build(plan.Input{Subject: sub, Scopes: []rulebook.Scope{rulebook.ConformanceScope}, Files: files})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Checks) != 29 {
		t.Fatalf("checks %d want 29", len(p.Checks))
	}
	for _, probe := range []string{"store_unchanged", "byte_bound", "planted_secret"} {
		p.Checks = append(p.Checks, plan.Check{Subject: sub, Preimage: plan.Object{"probe": probe}, Required: plan.Object{}})
	}
	s, c := selfTestFixture(false), selfTestFixture(true)
	base := observe.Controls{ConfinementApplied: true, SystemNamed: true}
	positive := confine.ControlFacts{Subject: s, Computation: c}
	if !positive.Apply(base).SelfTestPassed {
		t.Fatal("clean positive control failed")
	}
	s[len(s)-1].Stdout = []byte("wrong_value\n")
	failed := confine.ControlFacts{Subject: s, Computation: c}
	source := &selfTestSource{controls: failed.Apply(base)}
	checks, err := collect.Collect(p, source)
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 32 || source.observations != 0 {
		t.Fatalf("checks %d observations %d", len(checks), source.observations)
	}
	for _, ch := range checks {
		if ch.Result.Level != 1 || ch.Result.Outcome != "not_observed" || ch.Result.Reason != "network_unverifiable" || ch.Result.Unbuilt != nil {
			t.Fatalf("%s: %+v", ch.Plan.ID, ch.Result)
		}
	}
}

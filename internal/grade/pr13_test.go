package grade

import (
	"errors"
	"io"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/collect"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/rulebook"
	"github.com/vinnyp/press-quality-lab/internal/store"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func TestScopeRefusals(t *testing.T) {
	// E-7, X1: invalid scope lists reach the pre-check and never reach collection.
	for _, row := range []struct {
		name  string
		scope []string
	}{
		{"DestructiveAlone", []string{"destructive"}},
		{"Unknown", []string{"conformance", "unknown"}},
		{"Repeated", []string{"conformance", "conformance"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			o, d, _ := fixture(t)
			o.Scope = row.scope
			d.Source = func() (observe.Source, func() error, error) { t.Fatal("source reached"); return nil, nil, nil }
			code, err := Run(o, d, io.Discard)
			assertFailure(t, code, err, 2, "QE6", "X1: user caused pre-check rejection")
		})
	}
}

// safetySource changes source facts after the collector's startup preflight.
type safetySource struct {
	change func()
	served int
}

func (s *safetySource) Controls() (observe.Controls, error) {
	s.change()
	return observe.Controls{ConfinementApplied: true, SelfTestPassed: true, SystemNamed: true, PrivateCopyUnchanged: true, KnownFastWithin: true}, nil
}
func (s *safetySource) Observe(observe.Key, observe.Spec) (observe.Run, error) {
	s.served++
	return observe.Run{Markers: observe.Markers{Confinement: true}, Stdout: []byte("value"), StdoutSeen: 5}, nil
}

func TestDestructiveServeRefusal(t *testing.T) {
	// QS6, QA7, QE3: a post-planning destructive invocation is an engine fault before storage.
	command := plan.Object{"words": plan.Array{}, "class": "safe", "writes": false}
	sub := plan.Object{"commands": plan.Array{command}, "runbooks": plan.Array{plan.Object{"name": "book", "steps": plan.Array{plan.Object{"name": "first", "words": plan.Array{}, "budget_seconds": strictjson.Number("1"), "checks": plan.Array{plan.Object{"dimension": "response-correctness", "predicate": "stdout_equals", "expected": plan.Object{"literal": "value"}}}}}}}}
	p, err := plan.Build(plan.Input{Subject: sub, Scopes: []rulebook.Scope{"conformance"}})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for n < len(p.Checks) && p.Checks[n].Preimage["probe"] != nil {
		n++
	}
	p.Checks = p.Checks[n:]
	for _, book := range p.Runbooks {
		for _, step := range book.Steps {
			for i := range step.Required {
				step.Required[i] -= n
			}
		}
	}
	source := &safetySource{change: func() {}}
	got, err := collect.Collect(p, source)
	if err != nil || len(got) != 1 || got[0].Result.Outcome != "held" || source.served != 1 {
		t.Fatal(got, err, source.served)
	}
	source.served = 0
	source.change = func() { command["class"] = "destructive" }
	o, d, _ := fixture(t)
	writes := 0
	d.Write = func(store.Record) error { writes++; return errors.New("unexpected write") }
	d.Collect = func(plan.Plan, observe.Source) ([]collect.Check, error) { return collect.Collect(p, source) }
	code, err := Run(o, d, io.Discard)
	var fault *Error
	if code != 70 || !errors.As(err, &fault) || fault.Rule != "QS6" || writes != 0 || source.served != 0 {
		t.Fatalf("code=%d err=%v writes=%d served=%d", code, err, writes, source.served)
	}
}

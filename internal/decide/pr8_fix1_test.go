package decide

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"testing"
)

func TestPR8LiteralOverride(t *testing.T) {
	// QS5, E-22: literal objects never cause the final override.
	for _, literal := range []plan.Object{{"placeholder": "x"}, {"placeholder": "x", "k": 1}} {
		s := plan.Object{"runbooks": plan.Array{plan.Object{"steps": plan.Array{plan.Object{"checks": plan.Array{plan.Object{"expected": plan.Object{"literal": literal}}}}}}}}
		ch := plan.Check{Subject: s, Preimage: plan.Object{"dimension": "availability"}}
		held := Result{Outcome: "held", Level: 8}
		if got := FinalOverride(ch, held); got != held {
			t.Fatal(got)
		}
	}
}

func TestPR8ResolveBeforeOverride(t *testing.T) {
	// QS5, QA7: a held step remains held until its runbook finishes.
	saved := levels
	t.Cleanup(func() { levels = saved })
	levels = [...]level{func(plan.Check, observe.Controls, Evidence) Result { return Result{Outcome: "held", Level: 8} }, nil, nil, nil, nil, nil, nil, nil}
	ch := plan.Check{Subject: plan.Object{"binary": plan.Object{"placeholder": "Supply binary."}}, Preimage: plan.Object{"dimension": "availability"}}
	before := Resolve(ch, passing(), Evidence{})
	if before.Outcome != "held" {
		t.Fatal("runbook would stop", before)
	}
	if got := FinalOverride(ch, before); got.Outcome != NotObserved || got.Reason != "placeholder_unfilled" {
		t.Fatal(got)
	}
}

func TestPR8BarePlaceholders(t *testing.T) {
	// QS5: bare command dependencies block bare and authored checks, never discovery.
	for _, field := range []string{"class", "writes"} {
		t.Run(field, func(t *testing.T) {
			c := plan.Object{"words": plan.Array{}, "class": "safe", "writes": false}
			s := plan.Object{"commands": plan.Array{c}}
			for _, probe := range []any{"bare", nil, "discovery"} {
				ch := plan.Check{Subject: s, Preimage: plan.Object{"probe": probe, "words": plan.Array{}}}
				if got := level2(ch, passing(), Evidence{}); got != (Result{}) {
					t.Fatal(got)
				}
				c[field] = plan.Object{"placeholder": "Supply command setting."}
				got := level2(ch, passing(), Evidence{})
				if probe == "discovery" {
					if got != (Result{}) {
						t.Fatal(got)
					}
				} else if got.Reason != "placeholder_unfilled" {
					t.Fatal(got)
				}
				c[field] = false
				if field == "class" {
					c[field] = "safe"
				}
			}
		})
	}
}

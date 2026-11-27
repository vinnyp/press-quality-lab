package collect

import (
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/rulebook"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"testing"
)

func TestPR10BareBudget(t *testing.T) {
	// QP2: an undeclared bare binary receives a ten-second invocation budget.
	p, err := plan.Build(plan.Input{Subject: plan.Object{"commands": plan.Array{}, "bookkeeping": plan.Array{}}, Scopes: []rulebook.Scope{rulebook.ConformanceScope}})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ch := range p.Checks {
		if ch.Preimage["probe"] == "bare" {
			found = true
			if ch.Preimage["budget_seconds"] != strictjson.Number("10") || len(ch.Keys) != 1 {
				t.Fatalf("bare %+v", ch)
			}
		}
	}
	if !found {
		t.Fatal("bare probe absent")
	}
}

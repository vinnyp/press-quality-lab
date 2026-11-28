package collect

import (
	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"testing"
)

func TestPR8OverrideWired(t *testing.T) {
	// QS5, QA7: all checks resolve as held before the caller changes reported outcomes.
	ch := plan.Check{Subject: plan.Object{"commands": plan.Array{}, "runbooks": plan.Array{}, "binary": plan.Object{"placeholder": "Supply binary."}}, Preimage: plan.Object{"dimension": "availability"}}
	calls := 0
	got, e := collect(plan.Plan{Checks: []plan.Check{ch, ch}, Runbooks: []plan.Runbook{{Steps: []plan.Step{{Key: plan.Object{"words": plan.Array{}}, Required: []int{0, 1}}}}}}, &fake{}, func(plan.Check, observe.Controls, decide.Evidence) decide.Result {
		calls++
		return decide.Result{Outcome: "held", Level: 8}
	})
	if e != nil || calls != 2 || len(got) != 2 {
		t.Fatal(got, e, calls)
	}
	for _, check := range got {
		if check.Result.Outcome != decide.NotObserved || check.Result.Reason != "placeholder_unfilled" {
			t.Fatal(check)
		}
	}
}

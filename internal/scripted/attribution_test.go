package scripted

import (
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
)

// Keep decision values in this untagged test helper, preserving KTD13's boundary.
func CheckAttribution(t *testing.T, r observe.Run, by string) {
	t.Helper()
	want := observe.Exited
	switch by {
	case "unconfined", "before_marker":
		want = observe.Unconfined
	case "could_not_start":
		want = observe.CouldNotStart
	case "confinement":
		want = observe.ConfinementKill
	case "budget":
		want = observe.BudgetKill
	case "engine":
		want = observe.EngineKill
	case "memory", "code_signing", "signal":
		want = observe.Crashed
	}
	got, err := observe.Attribute(r)
	if err != nil || got != want {
		t.Fatalf("by %s got %v err %v want %v", by, got, err, want)
	}
}

func CheckInvocationDecision(t *testing.T, pre plan.Object, runs []observe.Run, c observe.Controls, outcome, reason, failure string) {
	t.Helper()
	want := decide.Result{}
	if outcome != "held" {
		want = decide.Result{Level: 7, Outcome: decide.Outcome(outcome), Reason: decide.Reason(reason), FailureKind: failure}
	}
	got := decide.Result{}
	for _, r := range runs {
		if decision := decide.InvocationOutcome(plan.Check{Preimage: pre}, c, r); decision.Level != 0 {
			got = decision
			break
		}
	}
	if got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

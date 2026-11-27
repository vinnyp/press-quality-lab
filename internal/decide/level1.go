package decide

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
)

func level1(_ plan.Check, c observe.Controls, e Evidence) Result {
	// QO1, QH1, QH6, QH9; E-15 includes checks with no invocation.
	failed := !c.ConfinementApplied || !c.SelfTestPassed || !c.SystemNamed
	for _, invocation := range e.Invocations {
		if invocation.Key["computation"] != nil {
			continue
		}
		r := invocation.Run
		if !r.Markers.Confinement {
			failed = true
		}
	}
	// E-53: a captured invocation carries the same confinement requirement.
	for _, invocation := range e.Captures {
		failed = failed || !invocation.Run.Markers.Confinement
	}
	if failed {
		return Result{Outcome: NotObserved, Reason: NetworkUnverifiable, Level: 1}
	}
	return Result{}
}

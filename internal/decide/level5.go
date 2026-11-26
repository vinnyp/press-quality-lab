package decide

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
)

func level5(ch plan.Check, c observe.Controls, e Evidence) Result {
	// QH8: a check without a copy use is not decided here.
	changed := len(ch.Keys) > 0 && !c.PrivateCopyUnchanged
	for _, invocation := range e.Invocations {
		changed = changed || invocation.Run.CopyChanged
	}
	if changed {
		return Result{Outcome: NotObserved, Reason: "harness_fault", Level: 5}
	}
	return Result{}
}

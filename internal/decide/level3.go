package decide

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
)

func level3(ch plan.Check, c observe.Controls, e Evidence) Result {
	_, known := ch.Preimage["probe"]
	if !known {
		panic("QE2: preimage lacks probe")
	}
	if ch.Preimage["probe"] != nil {
		if e.Discovery == nil {
			return sentinel(3, "QD4")
		}
		d := e.Discovery
		// QO1 blocks QD4, but this is a level-3 precondition query, not an
		// invocation reaching level 7. Other checks still serve their own keys.
		for _, invocation := range d.Invocations {
			if !invocation.Run.AfterMarker() {
				return Result{}
			}
		}
		if d.Ran && !d.Bound && !d.Unreadable && d.invocationResult(ch, c).Level == 0 && d.missing(ch) {
			return Result{Level: 3, Outcome: NotObserved, Reason: "precondition_unmet"}
		}
	}
	return Result{}
}

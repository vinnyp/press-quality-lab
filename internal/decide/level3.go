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
		if d.Ran && !d.Bound && !d.Unreadable && d.invocationResult(ch, c).Level == 0 && d.missing(ch) {
			return Result{Level: 3, Outcome: NotObserved, Reason: "precondition_unmet"}
		}
	}
	return Result{}
}

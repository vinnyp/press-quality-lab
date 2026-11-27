package decide

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
)

// plantedSecret is QO7's single ordered combination. Level 7 defers when
// the deciding step belongs to level 8, including leaks and control faults.
func plantedSecret(ch plan.Check, c observe.Controls, e Evidence) Result {
	fault := Result{Level: 8, Outcome: NotObserved, Reason: "harness_fault"}
	for _, inv := range e.Invocations {
		if len(inv.Run.Details.LeakScan.Leaks) > 0 {
			return Result{Level: 8, Outcome: "not_held", FailureKind: "predicate"}
		}
	}
	for _, inv := range e.Invocations {
		a, err := observe.Attribute(inv.Run)
		if err != nil {
			panic(err)
		}
		if a == observe.EngineKill {
			fault.Level = 7
			return fault
		}
	}
	proved := false
	if c.ReadCalibrationMoved {
		for _, inv := range e.Invocations {
			r := inv.Run
			p := r.Details.Planted
			a, err := observe.Attribute(r)
			if err != nil {
				panic(err)
			}
			if a != observe.Exited || !AcceptedInvocationCode(ch, inv.Key, r.End.Code) || p == nil || !p.AtimeMoved || !p.SameFile {
				continue
			}
			if p.ControlEnd == nil {
				panic("QP11: missing read control")
			}
			control, err := observe.Attribute(observe.Run{End: p.ControlEnd.End, Markers: p.ControlEnd.Markers})
			if err != nil {
				panic(err)
			}
			if control != observe.ConfinementKill {
				return fault
			}
			proved = true
		}
	}
	for _, inv := range e.Invocations {
		if r := DiscoveryInvocation(ch, c, inv.Run); r.Level != 0 {
			return r
		}
	}
	for _, inv := range e.Invocations {
		r := inv.Run
		if r.Details.LeakScan.BoundReached || r.Details.LeakScan.Unreadable || r.StdoutCut || r.StderrCut {
			return fault
		}
	}
	if proved {
		return Result{Level: 8, Outcome: "held"}
	}
	return Result{Level: 8, Outcome: NotObserved, Reason: "input_missing"}
}

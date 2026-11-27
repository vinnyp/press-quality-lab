package decide

import (
	"strconv"

	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

// Discovery contains facts accumulated by the walk, shared by generated checks.
type Discovery struct {
	Ran           bool
	FoundCount    int
	DeclaredFound []bool
	Missing       bool
	Undeclared    bool
	Names         []plan.Array
	Bound         bool
	Unreadable    bool
	Invocations   []observe.Invocation
}

func (d *Discovery) missing(ch plan.Check) bool {
	return d.Missing
}

// DiscoveryInvocation decides only QO1 to QO4, leaving every other end to QD5.
func DiscoveryInvocation(ch plan.Check, c observe.Controls, run observe.Run) Result {
	a, err := observe.Attribute(run)
	if err != nil {
		panic(err)
	}
	switch a {
	case observe.Unconfined, observe.CouldNotStart, observe.ConfinementKill, observe.BudgetKill:
		return InvocationOutcome(ch, c, run)
	case observe.Exited, observe.EngineKill, observe.Crashed:
		return Result{}
	default:
		panic("unknown discovery attribution")
	}
}

func (d *Discovery) invocationResult(ch plan.Check, c observe.Controls) Result {
	for _, invocation := range d.Invocations {
		if r := DiscoveryInvocation(ch, c, invocation.Run); r.Level != 0 {
			return r
		}
	}
	return Result{}
}

// DiscoveryOverride applies QS5's QD3 trigger after all checks have resolved.
func DiscoveryOverride(ch plan.Check, result Result, d *Discovery) Result {
	if d == nil || !d.Ran || d.missing(ch) {
		return result
	}
	if !d.Undeclared {
		return result
	}

	return finalOverride(ch, result, true)
}

func discoveryLevel8(ch plan.Check, d *Discovery) Result {
	if d == nil {
		return sentinel(8, "QD2")
	}
	if !d.Ran {
		panic("QD2: discovery reached level 8 without a walk")
	}
	if d.Bound || d.Unreadable || d.missing(ch) {
		return Result{Level: 8, Outcome: NotObserved, Reason: "precondition_unmet"}
	}
	count, ok := ch.Subject["declared_count"].(strictjson.Number)
	if !ok {
		panic("discovery count is not a number")
	}
	if !strictjson.EqualJSON(count, strictjson.Number(strconv.Itoa(d.FoundCount))) {
		return Result{Level: 8, Outcome: "not_held", FailureKind: "predicate"}
	}
	return Result{Level: 8, Outcome: "held"}
}

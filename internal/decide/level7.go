package decide

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"strconv"
)

// InvocationLevelBuilt leaves the planted-secret combination to U37.
func InvocationLevelBuilt(ch plan.Check) bool {
	return ch.Preimage != nil && ch.Preimage["probe"] != "planted_secret"
}

func level7(ch plan.Check, c observe.Controls, e Evidence) Result {
	if ch.Preimage["probe"] == "discovery" {
		if e.Discovery == nil {
			return sentinel(7, "QO7")
		}
		return e.Discovery.invocationResult(ch, c)
	}
	if ch.Preimage["probe"] == "planted_secret" && len(ch.Keys) == 0 {
		return Result{}
	}
	if !InvocationLevelBuilt(ch) {
		rule := "QO7"
		if ch.Preimage == nil {
			rule = "QO2"
		}
		return sentinel(7, rule)
	}
	for _, invocation := range e.Invocations {
		if invocation.Key["computation"] != nil {
			continue
		}
		if r := InvocationOutcome(ch, c, invocation.Run); r.Level != 0 {
			return r
		}
	}
	return Result{}
}

// InvocationOutcome decides QO1 to QO5 for a single subject invocation.
// Discovery limits this helper to QO1 to QO4 in its QO7 combination.
func InvocationOutcome(ch plan.Check, c observe.Controls, run observe.Run) Result {
	a, err := observe.Attribute(run)
	if err != nil {
		panic(err)
	}
	switch a {
	case observe.Unconfined:
		return Result{Outcome: NotObserved, Reason: NetworkUnverifiable, Level: 1}
	case observe.CouldNotStart:
		return Result{Outcome: NotObserved, Reason: "could_not_start", Level: 7}
	case observe.ConfinementKill:
		return Result{Outcome: NotObserved, Reason: NetworkUnverifiable, Level: 7}
	case observe.BudgetKill:
		if run.End.RunningAtBudget && c.KnownFastWithin {
			return Result{Outcome: "not_held", FailureKind: "over_budget", Level: 7}
		}
		return Result{Outcome: NotObserved, Reason: "harness_fault", Level: 7}
	case observe.EngineKill:
		return Result{Outcome: NotObserved, Reason: "harness_fault", Level: 7}
	case observe.Crashed:
		return Result{Outcome: "not_held", FailureKind: "crashed", Level: 7}
	case observe.Exited:
		if !acceptedCode(ch, run.End.Code) {
			return Result{Outcome: "not_held", FailureKind: "wrong_exit", Level: 7}
		}
		return Result{}
	default:
		panic("QO5: unknown attribution")
	}
}

func acceptedCode(ch plan.Check, code int) bool {
	p := ch.Preimage
	var codes strictjson.Array
	switch {
	case p["predicate"] == "exit_code":
		codes = strictjson.Array{object(p["expected"])["literal"]}
	case p["probe"] == "bare" || p["probe"] == "exit_codes":
		codes = array(object(p["parameters"])["exit_codes"])
	case p["probe"] == nil && object(p["parameters"])["exit_codes"] != nil:
		codes = array(object(p["parameters"])["exit_codes"])
	default:
		words := array(p["words"])
		best := -1
		for _, v := range array(ch.Subject["commands"]) {
			cmd := object(v)
			prefix := array(cmd["words"])
			if len(prefix) > len(words) || len(prefix) <= best {
				continue
			}
			match := true
			for i, w := range prefix {
				if w != words[i] {
					match = false
					break
				}
			}
			if match {
				best = len(prefix)
				codes = array(cmd["exit_codes"])
			}
		}
	}
	if codes == nil {
		return code == 0
	}
	for _, v := range codes {
		if strictjson.EqualJSON(v, strictjson.Number(strconv.Itoa(code))) {
			return true
		}
	}
	return false
}

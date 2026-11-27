package decide

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"math/big"
)

func level8(ch plan.Check, c observe.Controls, e Evidence) Result {
	rule := "QA9"
	switch ch.Preimage["probe"] {
	case "discovery":
		return discoveryLevel8(ch, e.Discovery)
	case "bare":
		return basicProbe(ch, e)
	case "byte_bound":
		return basicProbe(ch, e)
	case "exit_codes":
		return basicProbe(ch, e)
	case "store_unchanged":
		return storeUnchanged(ch, e)
	case "planted_secret":
		return plantedSecret(ch, c, e)
	}
	return sentinel(8, rule)
}

func basicProbe(ch plan.Check, e Evidence) Result {
	if len(ch.Keys) == 0 {
		return Result{Outcome: NotObserved, Reason: "empty_target", Level: 8}
	}
	if len(e.Invocations) != 1 {
		panic("QP1: missing probe invocation")
	}
	run := e.Invocations[0].Run
	r := Result{Outcome: "held", Level: 8}
	if ch.Preimage["probe"] == "byte_bound" {
		if err := observe.ValidateCounts(run, true); err != nil {
			panic(err)
		}
		sum := run.StdoutSeen + run.StderrSeen
		spelling, numeric := object(ch.Preimage["parameters"])["bound"].(strictjson.Number)
		if !numeric {
			panic("QP3: missing bound")
		}
		bound, ok := new(big.Rat).SetString(string(spelling))
		if !ok {
			panic("QP3: invalid bound")
		}
		if new(big.Rat).SetInt64(sum).Cmp(bound) > 0 {
			r.Outcome = "not_held"
			r.FailureKind = "predicate"
		}
	}
	return r
}

package decide

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"reflect"
	"strconv"
)

func computations(ch plan.Check) []plan.Object {
	ex := object(ch.Preimage["expected"])
	var result []plan.Object
	for _, v := range []any{object(ex["precondition"])["computation"], ex["computation"]} {
		if c := object(v); c != nil {
			result = append(result, c)
		}
	}
	return result
}
func computationRun(ch plan.Check, c plan.Object, e Evidence) observe.Run {
	for _, inv := range e.Invocations {
		if inv.Key["runbook"] == ch.Preimage["runbook"] && inv.Key["step"] == ch.Preimage["step"] && reflect.DeepEqual(inv.Key["computation"], c["argv"]) {
			return inv.Run
		}
	}
	panic("QA6: missing computation run")
}
func capturedRun(ch plan.Check, name any, e Evidence) observe.Run {
	for _, inv := range e.Captures {
		if inv.Key["runbook"] == ch.Preimage["runbook"] && inv.Key["step"] == name {
			return inv.Run
		}
	}
	panic("QA4: missing captured run")
}
func level6(ch plan.Check, _ observe.Controls, e Evidence) Result {
	if _, known := ch.Preimage["expected"]; !known {
		panic("QA2: missing expected context")
	}
	comps := computations(ch)
	if r := cutCaptures(ch, e); r.Level != 0 {
		return r
	}
	// Fetch all runs before judging any: an absent served run is an engine fault.
	runs := make([]observe.Run, len(comps))
	for i, c := range comps {
		runs[i] = computationRun(ch, c, e)
	}
	failed := func() Result { return Result{Outcome: NotObserved, Reason: "computation_failed", Level: 6} }
	for _, run := range runs {
		a, err := observe.Attribute(run)
		if err != nil {
			panic(err)
		}
		if a != observe.Exited {
			return failed()
		}
	}
	for _, run := range runs {
		if run.End.Code != 0 {
			return failed()
		}
	}
	for _, run := range runs {
		if run.Details.Printed == nil {
			return failed()
		}
	}
	if object(object(ch.Preimage["expected"])["computation"]) != nil {
		v := Expected(ch, e)
		switch ch.Preimage["predicate"] {
		case "exit_code", "stdout_bytes_at_most", "row_count":
			if !whole(v) {
				return failed()
			}
		case "stdout_equals", "stdout_contains", "stdout_not_contains", "stderr_contains":
			s, ok := v.(string)
			if !ok || s == "" {
				return failed()
			}
		}
	}
	return Result{}
}
func whole(v any) bool {
	n, ok := v.(strictjson.Number)
	if !ok {
		return false
	}
	i, err := strconv.ParseInt(string(n), 10, 64)
	return err == nil && i >= 0 && i <= 9007199254740991
}

// Expected implements QA2 for both invocation exit codes and predicates.
func Expected(ch plan.Check, e Evidence) any {
	ex := object(ch.Preimage["expected"])
	if c := object(ex["computation"]); c != nil {
		r := computationRun(ch, c, e)
		if r.Details.Printed == nil {
			panic("QA6: invalid computed expected value")
		}
		return r.Details.Printed.Value
	}
	return ex["literal"]
}
func authoredFields(ch plan.Check, e Evidence, r Result) Result {
	if ch.Preimage["probe"] != nil || r.Unbuilt != nil {
		return r
	}
	if r.Outcome == "held" || r.Outcome == "not_held" {
		r.Expected = Expected(ch, e)
		if c := object(object(object(ch.Preimage["expected"])["precondition"])["computation"]); c != nil {
			run := computationRun(ch, c, e)
			if run.Details.Printed == nil {
				panic("QA6: invalid precondition")
			}
			b := run.Details.Printed.Value == true
			r.PreconditionObserved = &b
		}
	}
	return r
}

// BeforeComputations retries levels 1 through 5 with the step run, then decides
// level 6's cut captures without requiring computation runs that are not served yet.
func BeforeComputations(ch plan.Check, c observe.Controls, e Evidence) Result {
	if r := BeforeInvocationsWithEvidence(ch, c, e); r.Level != 0 {
		return r
	}
	return cutCaptures(ch, e)
}
func cutCaptures(ch plan.Check, e Evidence) Result {
	cut := false
	for _, comp := range computations(ch) {
		for _, name := range array(comp["captures"]) {
			run := capturedRun(ch, name, e)
			cut = cut || run.StdoutCut
		}
	}
	if cut {
		return Result{Outcome: NotObserved, Reason: "harness_fault", Level: 6}
	}
	return Result{}
}

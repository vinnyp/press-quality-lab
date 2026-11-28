package collect

import (
	"errors"
	"reflect"
	"strconv"

	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

// Serve replays a key, or its entire runbook, through grade home preparation.
// Context supplies the subject and planned computations for keys absent from
// the ordinary served trace. MayServe still enforces subject safety and scope.
// Runbook replay deliberately continues after an earlier predicate fails.
func Serve(p plan.Plan, source observe.Source, context plan.Check, key plan.Object, options Options) (runs []observe.Invocation, err error) {
	if err := decide.MayServe(context, key, p.Scopes); err != nil {
		return nil, err
	}
	if key["runbook"] != nil {
		present := false
		for _, book := range p.Runbooks {
			for _, step := range book.Steps {
				present = present || sameReplayInvocation(step.Key, key)
				for _, i := range step.Required {
					for _, candidate := range p.Checks[i].Keys {
						present = present || sameReplayInvocation(candidate, key)
					}
				}
			}
		}
		if !present {
			return nil, &plan.Fault{Rule: "QA7", Cause: "replay key absent from runbook"}
		}
	}
	prepared := newHomeSource(p, source, options)
	defer func() { err = errors.Join(err, prepared.close()) }()
	if _, err := prepared.Controls(); err != nil {
		return nil, err
	}
	serve := func(ch plan.Check, key plan.Object) error {
		if err := decide.MayServe(ch, key, p.Scopes); err != nil {
			return err
		}
		budget := ch.Preimage["budget_seconds"]
		if ch.Preimage["probe"] == "discovery" {
			budget = strictjson.Number("10")
		}
		if key["computation"] != nil {
			found := false
			for _, comp := range decide.Computations(ch) {
				if reflect.DeepEqual(comp["argv"], key["computation"]) {
					budget = comp["budget_seconds"]
					found = true
					break
				}
			}
			if !found {
				return &plan.Fault{Rule: "QA6", Cause: "unmatched replay computation"}
			}
		}
		seconds := 0
		if n, ok := budget.(strictjson.Number); ok {
			var err error
			seconds, err = strconv.Atoi(string(n))
			if err != nil {
				return &plan.Fault{Rule: "QA7", Cause: "invalid replay budget"}
			}
		}
		prepared.checks[ch.ID] = ch
		run, err := prepared.Observe(map[string]any(key), observe.Spec{BudgetSeconds: seconds, CheckID: ch.ID, Preimage: ch.Preimage})
		if err != nil {
			return err
		}
		if err := observe.ValidateCounts(run, ch.Preimage["probe"] == "byte_bound"); err != nil {
			return err
		}
		runs = append(runs, observe.Invocation{Key: key, Run: run})
		return nil
	}
	if key["runbook"] == nil {
		if err := serve(context, key); err != nil {
			return nil, err
		}
		return runs, nil
	}
	found := false
	for _, book := range p.Runbooks {
		if len(book.Steps) == 0 || book.Steps[0].Key["runbook"] != key["runbook"] {
			continue
		}
		for _, step := range book.Steps {
			ch := context
			if len(step.Required) > 0 {
				ch = p.Checks[step.Required[0]]
			}
			pre := plan.Object{}
			for k, v := range ch.Preimage {
				pre[k] = v
			}
			pre["runbook"], pre["step"], pre["words"] = step.Key["runbook"], step.Key["step"], step.Key["words"]
			pre["probe"], pre["budget_seconds"] = nil, step.BudgetSeconds
			ch.Preimage = pre
			stepKey := step.Key
			if sameReplayInvocation(step.Key, key) {
				stepKey = key
			}
			if err := serve(ch, stepKey); err != nil {
				return nil, err
			}
			if sameReplayInvocation(step.Key, key) {
				found = true
			}
			served := []plan.Object{}
			for _, i := range step.Required {
				for _, k := range p.Checks[i].Keys {
					if k["computation"] == nil {
						continue
					}
					duplicate := false
					for _, old := range served {
						duplicate = duplicate || reflect.DeepEqual(old, k)
					}
					if duplicate {
						continue
					}
					computationKey := k
					if sameReplayInvocation(k, key) {
						computationKey = key
					}
					if err := serve(p.Checks[i], computationKey); err != nil {
						return nil, err
					}
					served = append(served, k)
					if sameReplayInvocation(k, key) {
						found = true
					}
				}
			}
		}
	}
	if !found {
		return nil, &plan.Fault{Rule: "QA7", Cause: "replay key absent from runbook"}
	}
	return runs, nil
}

// Offline arguments and environment names come from the replayed recording.
// The authored invocation still determines its words, home, step and computation.
func sameReplayInvocation(a, b plan.Object) bool {
	for _, field := range []string{"runbook", "step", "words", "home", "computation"} {
		if !reflect.DeepEqual(a[field], b[field]) {
			return false
		}
	}
	return true
}

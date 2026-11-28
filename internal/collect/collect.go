// Package collect owns the untagged collection sequence.
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

// Check pairs a planned check with its resolution, including an unbuilt value.
type Check struct {
	Plan        plan.Check
	Result      decide.Result
	Invocations []observe.Invocation
	Discovery   *decide.Discovery
}

// Collect resolves every check, retaining sentinels as values for inspection.
// Discovery runs first so level 3 and the final override share its facts.
// Runbook checks share each step run and resolve before the final overrides.
func Collect(p plan.Plan, source observe.Source, options ...Options) ([]Check, error) {
	var o Options
	if len(options) > 0 {
		o = options[0]
	}
	return collectWithOptions(p, source, o)
}

func collect(p plan.Plan, source observe.Source, resolve func(plan.Check, observe.Controls, decide.Evidence) decide.Result) (checks []Check, err error) {
	prepared := newHomeSource(p, source, Options{})
	defer func() { err = errors.Join(err, prepared.close()) }()
	return collectPrepared(p, prepared, resolve)
}
func collectPrepared(p plan.Plan, source *homeSource, resolve func(plan.Check, observe.Controls, decide.Evidence) decide.Result) ([]Check, error) {
	p.Checks = append([]plan.Check(nil), p.Checks...)
	// QE3, QA7: validate the entire schedule before discovery can serve a key.
	scheduledAtStart := map[int]bool{}
	for _, book := range p.Runbooks {
		var context plan.Check
		contextSet := false
		for _, step := range book.Steps {
			for _, i := range step.Required {
				scheduledAtStart[i] = true
				if !contextSet {
					context = p.Checks[i]
					contextSet = true
				}
			}
		}
		for _, step := range book.Steps {
			if err := decide.MayPlanServe(context, step.Key, p.Scopes); err != nil {
				return nil, err
			}
		}
	}
	for i, ch := range p.Checks {
		if ch.Preimage["probe"] == nil && !scheduledAtStart[i] {
			return nil, &plan.Fault{Rule: "QA7", Cause: "unscheduled authored check"}
		}
		for _, key := range ch.Keys {
			if err := decide.MayPlanServe(ch, key, p.Scopes); err != nil {
				return nil, err
			}
		}
	}

	controls, err := source.Controls()
	if err != nil {
		return nil, err
	}
	var discovery *decide.Discovery
	for i := range p.Checks {
		if p.Checks[i].Preimage["probe"] == "discovery" {
			var keys []plan.Object
			discovery, keys, err = walk(p.Checks[i], controls, source, p.Scopes)
			if err != nil {
				return nil, err
			}
			p.Checks[i].Keys = keys
			break
		}
	}
	checks := make([]Check, len(p.Checks))
	membership := map[string]bool{}
	earlier := map[string]decide.Result{}
	scheduled := map[int]bool{}
	for _, book := range p.Runbooks {
		for _, step := range book.Steps {
			for _, i := range step.Required {
				membership[p.Checks[i].ID] = true
				scheduled[i] = true
			}
			for _, id := range step.Outside {
				membership[id] = false
			}
		}
	}
	serve := func(key plan.Object, ch plan.Check) (observe.Invocation, error) {
		if err := decide.MayServe(ch, key, p.Scopes); err != nil {
			return observe.Invocation{}, err
		}
		budget := ch.Preimage["budget_seconds"]
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
				panic("QA6: unmatched computation key")
			}
		}
		seconds := 0
		if n, ok := budget.(strictjson.Number); ok {
			var err error
			seconds, err = strconv.Atoi(string(n))
			if err != nil {
				panic("QA7: invalid planned budget")
			}
		}
		source.checks[ch.ID] = ch
		run, err := source.Observe(map[string]any(key), observe.Spec{BudgetSeconds: seconds, CheckID: ch.ID, Preimage: map[string]any(ch.Preimage)})
		if err != nil {
			return observe.Invocation{}, err
		}
		if err := observe.ValidateCounts(run, ch.Preimage["probe"] == "byte_bound"); err != nil {
			return observe.Invocation{}, err
		}
		return observe.Invocation{Key: map[string]any(key), Run: run}, nil
	}
	finish := func(i int, facts decide.Evidence) {
		ch := p.Checks[i]
		result := resolve(ch, controls, facts)
		checks[i] = Check{Plan: ch, Result: result, Invocations: facts.Invocations}
		if ch.Preimage["probe"] == "discovery" {
			checks[i].Discovery = discovery
		}
		earlier[ch.ID] = result
	}
	// QP1's probes keep their existing serving procedure and order.
	for i, ch := range p.Checks {
		if scheduled[i] {
			continue
		}
		facts := decide.Evidence{Discovery: discovery, Membership: membership, Earlier: earlier}
		if ch.Preimage["probe"] == "discovery" && discovery != nil {
			facts.Invocations = discovery.Invocations
		}
		if ch.Preimage["probe"] != "discovery" && decide.BeforeInvocationsWithEvidence(ch, controls, facts).Level == 0 && decide.InvocationLevelBuilt(ch) {
			for _, key := range ch.Keys {
				inv, err := serve(key, ch)
				if err != nil {
					return nil, err
				}
				facts.Invocations = append(facts.Invocations, inv)
			}
		}
		finish(i, facts)
	}
	for _, book := range p.Runbooks {
		var captures []observe.Invocation
		var previous []string
		var context plan.Check
		for _, step := range book.Steps {
			if len(step.Required) > 0 {
				context = p.Checks[step.Required[0]]
				break
			}
		}
		for _, step := range book.Steps {
			stopped := decide.Stopped(previous, earlier, membership)
			facts := decide.Evidence{Discovery: discovery, Membership: membership, Earlier: earlier, Captures: captures}
			eligible := map[int]bool{}
			for _, i := range step.Required {
				eligible[i] = decide.BeforeInvocationsWithEvidence(p.Checks[i], controls, facts).Level == 0
			}
			var stepRun *observe.Invocation
			if len(step.Required) == 0 {
				bookName, bookOK := step.Key["runbook"].(string)
				stepName, stepOK := step.Key["step"].(string)
				if !bookOK || !stepOK {
					panic("QA7: invalid step key")
				}
				ch := plan.Check{ID: "runbook:" + bookName + ":step:" + stepName, Subject: context.Subject, Scopes: p.Scopes, Keys: []plan.Object{step.Key}, Preimage: plan.Object{"probe": nil, "runbook": step.Key["runbook"], "step": step.Key["step"], "words": step.Key["words"], "earlier": plan.Array{}, "expected": nil, "budget_seconds": step.BudgetSeconds, "offline_args": step.OfflineArgs, "env": step.Env}}
				if decide.StepRuns(decide.StepFacts{Stopped: stopped, Controls: controls, Check: &ch}) {
					inv, err := serve(step.Key, ch)
					if err != nil {
						return nil, err
					}
					stepRun = &inv
				}
			} else {
				for _, i := range step.Required {
					if eligible[i] {
						inv, err := serve(step.Key, p.Checks[i])
						if err != nil {
							return nil, err
						}
						stepRun = &inv
						break
					}
				}
			}
			var computations []observe.Invocation
			for _, i := range step.Required {
				ch := p.Checks[i]
				own := facts
				if stepRun != nil {
					own.Invocations = []observe.Invocation{*stepRun}
					if eligible[i] && decide.BeforeComputations(ch, controls, own).Level == 0 {
						for _, comp := range decide.Computations(ch) {
							var key plan.Object
							for _, k := range ch.Keys {
								if reflect.DeepEqual(k["computation"], comp["argv"]) {
									key = k
									break
								}
							}
							if key == nil {
								panic("QA6: missing computation key")
							}
							var found *observe.Invocation
							for j := range computations {
								if reflect.DeepEqual(computations[j].Key, map[string]any(key)) {
									found = &computations[j]
									break
								}
							}
							if found == nil {
								inv, err := serve(key, ch)
								if err != nil {
									return nil, err
								}
								computations = append(computations, inv)
								found = &computations[len(computations)-1]
							}
							// Equal precondition and expected computations remain one own invocation.
							duplicate := false
							for _, inv := range own.Invocations {
								duplicate = duplicate || reflect.DeepEqual(inv.Key, found.Key)
							}
							if !duplicate {
								own.Invocations = append(own.Invocations, *found)
							}
						}
					}
				}
				finish(i, own)
			}
			if stepRun != nil {
				captures = append(captures, *stepRun)
			}
			for _, i := range step.Required {
				previous = append(previous, p.Checks[i].ID)
			}
			previous = append(previous, step.Outside...)
		}
		if len(book.Steps) > 0 {
			if err := source.finishBook(homeText(book.Steps[0].Key["runbook"])); err != nil {
				return nil, err
			}
		}
	}
	for i := range checks {
		checks[i].Result = decide.FinalOverride(checks[i].Plan, checks[i].Result)
		checks[i].Result = decide.DiscoveryOverride(checks[i].Plan, checks[i].Result, discovery)
	}
	return checks, nil
}

// Build fixes the complete plan before asking the source for any facts.
func Build(in plan.Input, source observe.Source) ([]Check, error) {
	p, err := plan.Build(in)
	if err != nil {
		return nil, err
	}
	return Collect(p, source)
}

// Package collect owns the untagged collection sequence.
package collect

import (
	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
)

// Check pairs a planned check with its resolution, including an unbuilt value.
type Check struct {
	Plan   plan.Check
	Result decide.Result
}

// Collect resolves every check, retaining sentinels as values for inspection.
// Discovery runs first so level 3 and the final override share its facts.
// Other checks serve keys only after levels 1 through 6 and only with built rules.
func Collect(p plan.Plan, source observe.Source) ([]Check, error) {
	return collect(p, source, decide.Resolve)
}

func collect(p plan.Plan, source observe.Source, resolve func(plan.Check, observe.Controls, decide.Evidence) decide.Result) ([]Check, error) {
	controls, err := source.Controls()
	if err != nil {
		return nil, err
	}
	var discovery *decide.Discovery
	for i := range p.Checks {
		if p.Checks[i].Preimage["probe"] == "discovery" {
			var keys []plan.Object
			discovery, keys, err = walk(p.Checks[i], controls, source)
			if err != nil {
				return nil, err
			}
			p.Checks[i].Keys = keys
			break
		}
	}
	checks := make([]Check, 0, len(p.Checks))
	for _, ch := range p.Checks {
		facts := decide.Evidence{Discovery: discovery}
		if ch.Preimage["probe"] == "discovery" && discovery != nil {
			facts.Invocations = discovery.Invocations
		}
		if ch.Preimage["probe"] != "discovery" && decide.BeforeInvocationsWithEvidence(ch, controls, facts).Level == 0 && decide.InvocationLevelBuilt(ch) {
			for _, key := range ch.Keys {
				run, err := source.Observe(map[string]any(key), observe.Spec{CheckID: ch.ID, Preimage: map[string]any(ch.Preimage)})
				if err != nil {
					return nil, err
				}
				facts.Invocations = append(facts.Invocations, observe.Invocation{Key: map[string]any(key), Run: run})
			}
		}
		checks = append(checks, Check{Plan: ch, Result: resolve(ch, controls, facts)})
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

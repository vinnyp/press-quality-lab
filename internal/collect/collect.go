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
// Grade-wide QO1 and levels 2 through 6 run before serving any key.
// Only a check with built invocation rules serves its planned keys.
func Collect(p plan.Plan, source observe.Source) ([]Check, error) {
	return collect(p, source, decide.Resolve)
}

func collect(p plan.Plan, source observe.Source, resolve func(plan.Check, observe.Controls, decide.Evidence) decide.Result) ([]Check, error) {
	controls, err := source.Controls()
	if err != nil {
		return nil, err
	}
	checks := make([]Check, 0, len(p.Checks))
	for _, ch := range p.Checks {
		facts := decide.Evidence{}
		if decide.BeforeInvocations(ch, controls).Level == 0 && decide.InvocationLevelBuilt(ch) {
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

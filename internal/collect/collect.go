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
// Grade-wide QO1 runs before serving any key. In this slice all later paths
// stop at an unbuilt level before an invocation can be served. Later units add
// sequencing at those levels; no source call may jump over an unbuilt level.
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
		checks = append(checks, Check{Plan: ch, Result: resolve(ch, controls, decide.Evidence{})})
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

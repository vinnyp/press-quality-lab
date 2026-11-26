// Package decide assigns per-check evidence resolutions without I/O or verdicts.
package decide

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

type Outcome string
type Reason string

const NotObserved Outcome = "not_observed"
const NetworkUnverifiable Reason = "network_unverifiable"

// Unbuilt is a value for the exit funnel to turn into exit 70.
type Unbuilt struct {
	Level int
	Rule  string
}
type Result struct {
	Outcome Outcome
	Reason  Reason
	Level   int
	Unbuilt *Unbuilt
}

// Evidence contains only runs actually made for this check, never planned keys.
type Evidence struct{ Invocations []observe.Invocation }
type level func(plan.Check, observe.Controls, Evidence) Result

var levels = [...]level{level1, level2, level3, level4, level5, level6, level7, level8}

// Resolve tries every level in specification order until one decides.
func Resolve(ch plan.Check, controls observe.Controls, facts Evidence) Result {
	for _, step := range levels {
		r := step(ch, controls, facts)
		if r.Level != 0 {
			return FinalOverride(ch, r)
		}
	}
	panic("precedence table has no terminal level")
}
func sentinel(n int, rule string) Result {
	return Result{Level: n, Unbuilt: &Unbuilt{Level: n, Rule: rule}}
}
func object(v any) strictjson.Object {
	o, ok := v.(strictjson.Object)
	if !ok {
		return nil
	}
	return o
}

// dependency finds any placeholder for QS5's final override.
func dependency(v any) bool {
	switch x := v.(type) {
	case strictjson.Object:
		if _, ok := x["placeholder"]; ok {
			return true
		}
		for _, v := range x {
			if dependency(v) {
				return true
			}
		}
	case strictjson.Array:
		for _, v := range x {
			if dependency(v) {
				return true
			}
		}
	}
	return false
}

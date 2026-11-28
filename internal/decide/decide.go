// Package decide assigns per-check evidence resolutions without I/O or verdicts.
package decide

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"reflect"
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
	Expected             any
	TargetSize           *int64
	PreconditionObserved *bool
	FailureKind          string
	Outcome              Outcome
	Reason               Reason
	Level                int
	Unbuilt              *Unbuilt
}

// Evidence contains only runs actually made for this check, never planned keys.
type Evidence struct {
	Captures    []observe.Invocation
	Earlier     map[string]Result
	Membership  map[string]bool
	Invocations []observe.Invocation
	Discovery   *Discovery
}
type level func(plan.Check, observe.Controls, Evidence) Result

var levels = [...]level{level1, level2, level3, level4, level5, level6, level7, level8}

// Resolve tries every level in specification order until one decides.
func Resolve(ch plan.Check, controls observe.Controls, facts Evidence) Result {
	for _, inv := range append(append([]observe.Invocation(nil), facts.Invocations...), facts.Captures...) {
		if inv.Run.CopyChanged && !reflect.DeepEqual(inv.Run, observe.Run{CopyChanged: true}) {
			panic(&plan.Fault{Rule: "QH8", Cause: "changed copy carries launch facts"})
		}
	}
	for _, step := range levels {
		r := step(ch, controls, facts)
		if r.Level != 0 {
			return authoredFields(ch, facts, r)
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

// dependency inspects only the positions QS5 permits, excluding literal data.
func dependency(v any) bool {
	s := object(v)
	for _, key := range []string{"binary", "declared_count", "seed", "store", "secret"} {
		if placeholder(s[key]) {
			return true
		}
	}
	for _, value := range object(s["facts"]) {
		if placeholder(value) {
			return true
		}
	}
	for _, value := range array(s["commands"]) {
		c := object(value)
		if placeholder(c["class"]) || placeholder(c["writes"]) {
			return true
		}
	}
	return false
}

// BeforeInvocations tries levels 1 through 5 before the loop serves any key.
func BeforeInvocations(ch plan.Check, c observe.Controls) Result {
	return BeforeInvocationsWithEvidence(ch, c, Evidence{})
}

// BeforeInvocationsWithEvidence includes the already completed discovery walk.
func BeforeInvocationsWithEvidence(ch plan.Check, c observe.Controls, e Evidence) Result {
	for _, step := range levels[:5] {
		if r := step(ch, c, e); r.Level != 0 {
			return r
		}
	}
	return Result{}
}

package decide

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
)

func level6(ch plan.Check, _ observe.Controls, _ Evidence) Result {
	expected := object(ch.Preimage["expected"])
	value, known := ch.Preimage["expected"]
	unknown := !known || (value != nil && expected == nil)
	if unknown || expected["computation"] != nil || expected["precondition"] != nil || dependencyCapture(expected) {
		return sentinel(6, "QO6")
	}
	return Result{}
}

// Captures can occur in either the expected value or a precondition computation.
func dependencyCapture(v any) bool {
	switch x := v.(type) {
	case plan.Object:
		if captures, ok := x["captures"].(plan.Array); ok && len(captures) > 0 {
			return true
		}
		for _, v := range x {
			if dependencyCapture(v) {
				return true
			}
		}
	case plan.Array:
		for _, v := range x {
			if dependencyCapture(v) {
				return true
			}
		}
	}
	return false
}

package decide

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
)

func level3(ch plan.Check, _ observe.Controls, _ Evidence) Result {
	_, known := ch.Preimage["probe"]
	if !known || ch.Preimage["probe"] != nil {
		return sentinel(3, "QD4")
	}
	return Result{}
}

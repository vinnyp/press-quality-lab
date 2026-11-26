package decide

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
)

func level2(ch plan.Check, _ observe.Controls, _ Evidence) Result {
	if ch.Required == nil || dependency(ch.Required) {
		return sentinel(2, "QS5")
	}
	return Result{}
}

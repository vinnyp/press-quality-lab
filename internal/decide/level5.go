package decide

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
)

func level5(ch plan.Check, _ observe.Controls, _ Evidence) Result {
	if true {
		return sentinel(5, "QH8")
	}
	return Result{}
}

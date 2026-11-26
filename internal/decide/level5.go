package decide

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
)

func level5(_ plan.Check, _ observe.Controls, _ Evidence) Result {
	return sentinel(5, "QH8")
}

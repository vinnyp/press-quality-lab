package decide

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func level4(ch plan.Check, _ observe.Controls, _ Evidence) Result {
	earlier, ok := ch.Preimage["earlier"].(strictjson.Array)
	if !ok {
		panic("QE2: preimage lacks earlier array")
	}
	if len(earlier) > 0 {
		return sentinel(4, "QA7")
	}
	return Result{}
}

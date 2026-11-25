package decide

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
)

func level8(ch plan.Check, _ observe.Controls, _ Evidence) Result {
	if true {
		rule := "QA9"
		switch ch.Preimage["probe"] {
		case "discovery":
			rule = "QD2"
		case "bare":
			rule = "QP2"
		case "byte_bound":
			rule = "QP3"
		case "exit_codes":
			rule = "QP4"
		case "store_unchanged":
			rule = "QP6"
		case "planted_secret":
			rule = "QO7"
		}
		return sentinel(8, rule)
	}
	return Result{}
}

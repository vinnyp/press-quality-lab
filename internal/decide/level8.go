package decide

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
)

func level8(ch plan.Check, _ observe.Controls, e Evidence) Result {
	rule := "QA9"
	switch ch.Preimage["probe"] {
	case "discovery":
		return discoveryLevel8(ch, e.Discovery)
	case "bare":
		rule = "QP2"
	case "byte_bound":
		rule = "QP3"
	case "exit_codes":
		rule = "QP4"
	case "store_unchanged":
		rule = "QP6"
	case "planted_secret":
		rule = "QP10"
	}
	return sentinel(8, rule)
}

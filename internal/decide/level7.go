package decide

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func level7(ch plan.Check, _ observe.Controls, _ Evidence) Result {
	commands, ok := object(ch.Preimage["parameters"])["commands"].(strictjson.Array)
	_, present := object(ch.Preimage["parameters"])["commands"]
	commandsUnknown := object(ch.Preimage["parameters"]) == nil || (present && !ok)
	if ch.Preimage == nil || ch.Preimage["words"] != nil || ch.Preimage["probe"] == "discovery" || (ch.Preimage["probe"] == "planted_secret" && (commandsUnknown || len(commands) > 0)) {
		return sentinel(7, "QO2")
	}
	return Result{}
}

package decide

import (
	"reflect"

	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/rulebook"
)

// MayServe independently checks the source command before a key is served.
func MayServe(ch plan.Check, key plan.Object, scopes []rulebook.Scope) error {
	if err := MayPlanServe(ch, key, scopes); err != nil {
		return err
	}
	if exemptKey(ch, key) {
		return nil
	}
	cmd := servingCommand(ch, array(key["words"]))
	if placeholder(cmd["class"]) || placeholder(cmd["writes"]) {
		return &plan.Fault{Rule: "QS5", Cause: "placeholder invocation"}
	}
	return nil
}

// MayPlanServe applies only the destructive brake, before any discovery runs.
// Placeholder candidates remain planned and are stopped by level 2 or MayServe.
func MayPlanServe(ch plan.Check, key plan.Object, scopes []rulebook.Scope) error {
	if exemptKey(ch, key) {
		return nil
	}
	destructive := servingCommand(ch, array(key["words"]))["class"] == "destructive"
	if key["runbook"] != nil {
		for _, v := range array(ch.Subject["runbooks"]) {
			book := object(v)
			if book["name"] != key["runbook"] {
				continue
			}
			for _, step := range array(book["steps"]) {
				destructive = destructive || servingCommand(ch, array(object(step)["words"]))["class"] == "destructive"
			}
		}
	}
	if !destructive {
		return nil
	}
	if key["runbook"] != nil {
		for _, token := range scopes {
			if token == "destructive" {
				return nil
			}
		}
	}
	return &plan.Fault{Rule: "QS6", Cause: "destructive invocation outside allowed runbook scope"}
}

func exemptKey(ch plan.Check, key plan.Object) bool {
	return ch.Preimage["probe"] == "discovery" || key["words"] == nil && key["home"] == "scratch"
}

func servingCommand(ch plan.Check, words plan.Array) plan.Object {
	var selected plan.Object
	best := -1
	for _, v := range array(ch.Subject["commands"]) {
		cmd := object(v)
		prefix := array(cmd["words"])
		if len(prefix) > best && len(prefix) <= len(words) && reflect.DeepEqual(prefix, words[:len(prefix)]) {
			selected, best = cmd, len(prefix)
		}
	}
	return selected
}

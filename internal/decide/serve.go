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
	if placeholderRunbook(ch, key["runbook"], scopes) {
		return &plan.Fault{Rule: "QS5", Cause: "placeholder runbook"}
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
	// collection-rules.md makes the bare binary the command of any invocation
	// no declared command prefixes. Null words are no invocation: only the
	// computation keys exempted above carry them, so any other is a corrupt plan.
	if key["words"] == nil {
		return &plan.Fault{Rule: "QS6", Cause: "null words outside a computation"}
	}
	if ch.Subject == nil {
		return &plan.Fault{Rule: "QS6", Cause: "missing subject"}
	}
	if _, ok := ch.Subject["commands"].(plan.Array); !ok {
		return &plan.Fault{Rule: "QS6", Cause: "invalid commands"}
	}
	if _, ok := ch.Subject["runbooks"].(plan.Array); !ok {
		return &plan.Fault{Rule: "QA7", Cause: "invalid runbooks"}
	}
	if key["runbook"] != nil {
		matches, words := 0, false
		for _, v := range array(ch.Subject["runbooks"]) {
			b := object(v)
			if b["name"] != key["runbook"] {
				continue
			}
			matches++
			for _, st := range array(b["steps"]) {
				words = words || reflect.DeepEqual(object(st)["words"], key["words"])
			}
		}
		if matches != 1 || !words {
			return &plan.Fault{Rule: "QA7", Cause: "unresolved runbook key"}
		}
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
	return ch.Preimage["probe"] == "discovery" || key["computation"] != nil && key["words"] == nil && key["home"] == "scratch"
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

// placeholderRunbook applies E-54 using raw scope tokens and source classes.
func placeholderRunbook(ch plan.Check, name any, scopes []rulebook.Scope) bool {
	if name == nil {
		return false
	}
	for _, scope := range scopes {
		if scope == "destructive" {
			return false
		}
	}
	for _, v := range array(ch.Subject["runbooks"]) {
		b := object(v)
		if b["name"] != name {
			continue
		}
		for _, st := range array(b["steps"]) {
			if placeholder(servingCommand(ch, array(object(st)["words"]))["class"]) {
				return true
			}
		}
	}
	return false
}

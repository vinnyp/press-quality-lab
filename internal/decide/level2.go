package decide

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"reflect"
)

func placeholder(v any) bool { o := object(v); _, ok := o["placeholder"]; return len(o) == 1 && ok }
func array(v any) strictjson.Array {
	a, ok := v.(strictjson.Array)
	if !ok {
		return nil
	}
	return a
}

func level2(ch plan.Check, _ observe.Controls, _ Evidence) Result {
	if ch.Subject == nil {
		panic("QS5: check lacks subject")
	}
	s := ch.Subject
	blocked := placeholder(s["binary"])
	probe := ch.Preimage["probe"]
	if probe == "discovery" {
		blocked = blocked || placeholder(s["declared_count"])
	} else {
		blocked = blocked || placeholder(s["seed"]) || placeholder(s["store"])
		words := []strictjson.Array{}
		if probe == "planted_secret" {
			blocked = blocked || placeholder(s["secret"])
			for _, w := range array(object(s["secret"])["commands"]) {
				words = append(words, array(w))
			}
		} else if ch.Preimage["words"] != nil {
			words = append(words, array(ch.Preimage["words"]))
		}
		for _, comp := range computations(ch) {
			for _, name := range array(comp["captures"]) {
				for _, b := range array(s["runbooks"]) {
					book := object(b)
					if book["name"] != ch.Preimage["runbook"] {
						continue
					}
					for _, step := range array(book["steps"]) {
						st := object(step)
						if st["name"] == name {
							words = append(words, array(st["words"]))
						}
					}
				}
			}
		}
		for _, w := range words {
			longest := -1
			var selected strictjson.Object
			for _, v := range array(s["commands"]) {
				c := object(v)
				prefix := array(c["words"])
				if len(prefix) > longest && len(prefix) <= len(w) && (len(prefix) == 0 || reflect.DeepEqual(prefix, w[:len(prefix)])) {
					selected = c
					longest = len(prefix)
				}
			}
			blocked = blocked || placeholder(selected["class"]) || placeholder(selected["writes"])
		}
	}
	if blocked {
		return Result{Outcome: NotObserved, Reason: "placeholder_unfilled", Level: 2}
	}
	return Result{}
}

// FinalOverride applies QS5 after the otherwise final result. It must not be used
// to decide whether a runbook continues (QA7).
func FinalOverride(ch plan.Check, result Result) Result {
	return finalOverride(ch, result, dependency(ch.Subject))
}

func finalOverride(ch plan.Check, result Result, trigger bool) Result {
	if result.Unbuilt != nil || result.Outcome != "held" || !trigger {
		return result
	}
	switch ch.Preimage["dimension"] {
	case "bounding", "idempotency", "destructive-guard", "side-effect-integrity", "flag-conformance":
		return result
	}
	return Result{Outcome: NotObserved, Reason: "placeholder_unfilled", Level: result.Level}
}

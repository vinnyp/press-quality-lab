package decide

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/rulebook"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"testing"
)

func TestMayServe(t *testing.T) {
	// QS5, QS6, QA7, QA8, QD2: resolve source commands independently, by longest prefix.
	sub := plan.Object{"commands": plan.Array{
		plan.Object{"words": plan.Array{"remove"}, "class": "destructive", "writes": true},
		plan.Object{"words": plan.Array{"remove", "preview"}, "class": "safe", "writes": false},
		plan.Object{"words": plan.Array{"config"}, "class": "safe", "writes": false},
		plan.Object{"words": plan.Array{"config", "reset"}, "class": "destructive", "writes": true},
		plan.Object{"words": plan.Array{"unknown"}, "class": plan.Object{"placeholder": "class"}},
		plan.Object{"words": plan.Array{"pending"}, "class": "safe", "writes": plan.Object{"placeholder": "writes"}},
	}, "runbooks": plan.Array{plan.Object{"name": "mixed", "steps": plan.Array{plan.Object{"words": plan.Array{"config"}}, plan.Object{"words": plan.Array{"remove", "gear"}}}}}}
	for _, row := range []struct {
		name, probe, book, home string
		words                   plan.Array
		scopes                  []rulebook.Scope
		allowed                 bool
	}{
		{"SafeChild", "bare", "", "seed", plan.Array{"remove", "preview"}, nil, true},
		{"DestructiveChild", "bare", "", "seed", plan.Array{"config", "reset"}, []rulebook.Scope{"destructive"}, false},
		{"Probe", "bare", "", "seed", plan.Array{"remove"}, nil, false},
		{"Outside", "", "mixed", "runbook", plan.Array{"config"}, []rulebook.Scope{"conformance"}, false},
		{"Inside", "", "mixed", "runbook", plan.Array{"config"}, []rulebook.Scope{"conformance", "destructive"}, true},
		{"UnknownToken", "", "mixed", "runbook", plan.Array{"config"}, []rulebook.Scope{"destructive-guard"}, false},
		{"Discovery", "discovery", "", "empty", plan.Array{"remove", "--help"}, nil, true},
		{"RunbookHelp", "", "mixed", "runbook", plan.Array{"remove", "--help"}, nil, false},
		{"Computation", "", "mixed", "scratch", nil, nil, true},
		{"ClassPlaceholder", "bare", "", "seed", plan.Array{"unknown"}, nil, false},
		{"WritesPlaceholder", "bare", "", "seed", plan.Array{"pending"}, nil, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			pre := plan.Object{"probe": nil}
			if row.probe != "" {
				pre["probe"] = row.probe
			}
			key := plan.Object{"words": row.words, "home": row.home, "runbook": nil}
			if row.words == nil {
				key["words"] = nil
			}
			if row.book != "" {
				key["runbook"] = row.book
			}
			ch := plan.Check{Subject: sub, Preimage: pre}
			if (MayServe(ch, key, row.scopes) == nil) != row.allowed {
				t.Fatal(row.name)
			}
		})
	}
}

func TestDestructiveGuardPredicate(t *testing.T) {
	// QA8, QA9, QO5: refusal requires both the authored exit and confirmation message.
	for _, row := range []struct {
		name    string
		code    int
		stderr  string
		outcome Outcome
		kind    string
	}{
		{"Refused", 3, "use --confirm", "held", ""},
		{"ZeroExit", 0, "use --confirm", "not_held", "wrong_exit"},
		{"NoMessage", 3, "refused", "not_held", "predicate"},
	} {
		t.Run(row.name, func(t *testing.T) {
			ch := plan.Check{Subject: plan.Object{}, Preimage: plan.Object{"probe": nil, "earlier": plan.Array{}, "predicate": "stderr_contains", "dimension": "destructive-guard", "expected": plan.Object{"literal": "--confirm"}, "parameters": plan.Object{"exit_codes": plan.Array{strictjson.Number("3")}}}}
			r := Resolve(ch, passing(), Evidence{Invocations: []observe.Invocation{{Run: observe.Run{Markers: observe.Markers{Confinement: true}, End: observe.End{Code: row.code}, Stderr: []byte(row.stderr), StderrSeen: int64(len(row.stderr))}}}})
			if r.Outcome != row.outcome || r.FailureKind != row.kind {
				t.Fatal(r)
			}
		})
	}
}

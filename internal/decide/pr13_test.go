package decide

import (
	"errors"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/rulebook"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
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
	}, "runbooks": plan.Array{plan.Object{"name": "mixed", "steps": plan.Array{plan.Object{"words": plan.Array{"config"}}, plan.Object{"words": plan.Array{"remove", "gear"}}, plan.Object{"words": plan.Array{"remove", "--help"}}}}}}
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
		{"RunbookHelp", "", "mixed", "runbook", plan.Array{"remove", "--help"}, []rulebook.Scope{"conformance"}, false},
		{"Computation", "", "mixed", "scratch", nil, nil, true},
		{"ClassPlaceholder", "bare", "", "seed", plan.Array{"unknown"}, nil, false},
		{"WritesPlaceholder", "bare", "", "seed", plan.Array{"pending"}, nil, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			pre := plan.Object{"probe": nil}
			if row.probe != "" {
				pre["probe"] = row.probe
			}
			key := plan.Object{"computation": nil, "words": row.words, "home": row.home, "runbook": nil}
			if row.name == "Computation" {
				key["words"] = nil
				key["computation"] = "compute"
			}
			if row.book != "" {
				key["runbook"] = row.book
			}
			ch := plan.Check{Subject: sub, Preimage: pre}
			err := MayServe(ch, key, row.scopes)
			if row.allowed {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			want := "QS6"
			if row.name == "ClassPlaceholder" || row.name == "WritesPlaceholder" {
				want = "QS5"
			}
			var fault *plan.Fault
			if !errors.As(err, &fault) || fault.Rule != want {
				t.Fatalf("got %v, want %s", err, want)
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

func TestMayPlanServeMissingFacts(t *testing.T) {
	// QS6, QA7, X3: absent safety facts are engine faults.
	for _, name := range []string{"Held", "ProbeHeld", "Subject", "Commands", "Runbooks", "MissingBook", "DuplicateBook", "Words"} {
		t.Run(name, func(t *testing.T) {
			book := plan.Object{"name": "book", "steps": plan.Array{plan.Object{"words": plan.Array{"read"}}}}
			sub := plan.Object{"commands": plan.Array{plan.Object{"words": plan.Array{"read"}, "class": "safe"}}, "runbooks": plan.Array{book}}
			key := plan.Object{"runbook": "book", "words": plan.Array{"read"}}
			switch name {
			case "ProbeHeld":
				key["runbook"] = nil
			case "Subject":
				// Cause distinguishes the nil-subject guard from the commands type guard.
				sub = nil
			case "Commands":
				sub["commands"] = "wrong"
			case "Runbooks":
				key["runbook"] = nil
				sub["runbooks"] = "wrong"
			case "MissingBook":
				key["runbook"] = "missing"
			case "DuplicateBook":
				sub["runbooks"] = plan.Array{book, book}
			case "Words":
				key["words"] = plan.Array{"other"}
			}
			err := MayPlanServe(plan.Check{Subject: sub}, key, nil)
			if name == "Held" || name == "ProbeHeld" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			want, cause := "QA7", "unresolved runbook key"
			switch name {
			case "Subject":
				want, cause = "QS6", "missing subject"
			case "Commands":
				want, cause = "QS6", "invalid commands"
			case "Runbooks":
				cause = "invalid runbooks"
			}
			var fault *plan.Fault
			if !errors.As(err, &fault) || fault.Rule != want || fault.Cause != cause {
				t.Fatalf("got %v, want %s: %s", err, want, cause)
			}
		})
	}
}
func TestComputationExemption(t *testing.T) {
	// QS6, QA7, X3: scratch and null words alone do not identify a computation.
	ch := plan.Check{Subject: plan.Object{"commands": plan.Array{}, "runbooks": plan.Array{}}}
	for _, name := range []string{"Exempt", "Home", "NoComputation", "Unknown"} {
		t.Run(name, func(t *testing.T) {
			key := plan.Object{"home": "scratch", "words": nil, "computation": "compute"}
			if name == "Home" {
				key["home"] = "seed"
			}
			if name == "NoComputation" {
				key["computation"] = nil
			}
			if name == "Unknown" {
				key["words"] = plan.Array{"unknown"}
				key["computation"] = nil
			}
			err := MayPlanServe(ch, key, []rulebook.Scope{"conformance"})
			if name == "Exempt" || name == "Unknown" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var fault *plan.Fault
			if !errors.As(err, &fault) || fault.Rule != "QS6" || fault.Cause != "null words outside a computation" {
				t.Fatal(err)
			}
		})
	}
}

func TestPlaceholderRunbook(t *testing.T) {
	// QS5, QA7, E-49, E-54: whole-book safety depends on class, never writes alone.
	for _, name := range []string{"Outside", "Inside", "Writes"} {
		t.Run(name, func(t *testing.T) {
			pending := plan.Object{"words": plan.Array{"pending"}, "class": plan.Object{"placeholder": "class"}, "writes": false}
			sub := plan.Object{"commands": plan.Array{plan.Object{"words": plan.Array{"read"}, "class": "safe", "writes": false}, pending}, "runbooks": plan.Array{plan.Object{"name": "book", "steps": plan.Array{plan.Object{"words": plan.Array{"read"}}, plan.Object{"words": plan.Array{"pending"}}}}}}
			ch := plan.Check{Subject: sub, Preimage: plan.Object{"probe": nil, "earlier": plan.Array{}, "runbook": "book", "words": plan.Array{"read"}}, Scopes: []rulebook.Scope{"conformance"}}
			if name == "Inside" {
				ch.Scopes = append(ch.Scopes, "destructive")
			}
			if name == "Writes" {
				pending["class"] = "safe"
				pending["writes"] = plan.Object{"placeholder": "writes"}
			}
			blocked := name == "Outside"
			if r := level2(ch, passing(), Evidence{}); (r.Level == 2 && r.Reason == "placeholder_unfilled") != blocked {
				t.Fatal(r)
			}
			if got := StepRuns(StepFacts{Check: &ch, Controls: passing()}); got == blocked {
				t.Fatal("StepRuns", got)
			}
			var fault *plan.Fault
			key := plan.Object{"runbook": "book", "words": plan.Array{"read"}}
			if err := MayServe(ch, key, ch.Scopes); (err != nil) != blocked || blocked && (!errors.As(err, &fault) || fault.Rule != "QS5") {
				t.Fatal(err)
			}
			key["words"], key["home"], key["computation"] = nil, "scratch", "compute"
			if err := MayServe(ch, key, ch.Scopes); (err != nil) != blocked || blocked && (!errors.As(err, &fault) || fault.Rule != "QS5") {
				t.Fatal("computation", err)
			}
		})
	}
}

package decide

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"testing"
)

func TestPR10ProbePredicates(t *testing.T) {
	// QP1, QP2, QP3, QP4, QE8: predicate outcomes and exact drained target size.
	for _, row := range []struct {
		name, probe string
		codes       plan.Array
		code        int
		out, err    int64
		keys        bool
		outcome     Outcome
		reason      Reason
		failure     string
	}{
		{"StoreEmpty", "store_unchanged", nil, 0, 0, 0, false, NotObserved, "empty_target", ""},
		{"ExitEmpty", "exit_codes", nil, 0, 0, 0, false, NotObserved, "empty_target", ""},
		{"Empty", "byte_bound", nil, 0, 0, 0, false, NotObserved, "empty_target", ""},
		{"BareZero", "bare", nil, 0, 0, 0, true, "held", "", ""},
		{"BareOther", "bare", nil, 3, 0, 0, true, "not_held", "", "wrong_exit"},
		{"BareDeclared", "bare", plan.Array{strictjson.Number("3")}, 3, 0, 0, true, "held", "", ""},
		{"Below", "byte_bound", nil, 0, 3, 4, true, "held", "", ""},
		{"At", "byte_bound", nil, 0, 4, 4, true, "held", "", ""},
		{"Past", "byte_bound", nil, 0, 5, 4, true, "not_held", "", "predicate"},
		{"DeclaredExit", "exit_codes", plan.Array{strictjson.Number("3")}, 3, 0, 0, true, "held", "", ""},
		{"UndeclaredExit", "exit_codes", plan.Array{strictjson.Number("3")}, 0, 0, 0, true, "not_held", "", "wrong_exit"},
	} {
		t.Run(row.name, func(t *testing.T) {
			ch := plan.Check{Preimage: plan.Object{"probe": row.probe, "parameters": plan.Object{"bound": strictjson.Number("8"), "exit_codes": row.codes}}}
			facts := Evidence{}
			if row.keys {
				ch.Keys = []plan.Object{{}}
				facts.Invocations = []observe.Invocation{{Run: observe.Run{End: observe.End{Code: row.code}, Markers: observe.Markers{Confinement: true}, StdoutSeen: row.out, StderrSeen: row.err}}}
			}
			r := level7(ch, passing(), facts)
			if r.Level == 0 {
				r = level8(ch, passing(), facts)
			}
			if r.Outcome != row.outcome || r.Reason != row.reason || r.FailureKind != row.failure || r.Unbuilt != nil {
				t.Fatalf("result %+v", r)
			}
		})
	}
}

func TestPR10PlantedAcceptedCode(t *testing.T) {
	// QP11, QO7: each invocation uses its own listed command's successful codes.
	ch := plan.Check{Subject: plan.Object{"commands": plan.Array{
		plan.Object{"words": plan.Array{}, "exit_codes": plan.Array{strictjson.Number("9")}},
		plan.Object{"words": plan.Array{"read"}, "exit_codes": plan.Array{strictjson.Number("3")}},
		plan.Object{"words": plan.Array{"other"}, "exit_codes": plan.Array{strictjson.Number("4")}},
	}}, Preimage: plan.Object{"probe": "planted_secret"}}
	for _, row := range []struct {
		word string
		code int
		want bool
	}{{"read", 3, true}, {"read", 9, false}, {"other", 4, true}, {"other", 3, false}} {
		if got := AcceptedInvocationCode(ch, observe.Key{"words": plan.Array{row.word}}, row.code); got != row.want {
			t.Fatalf("%s code %d: %v", row.word, row.code, got)
		}
	}
}

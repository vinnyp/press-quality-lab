package decide

import (
	"fmt"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func TestCouldNotStartCodes(t *testing.T) {
	// QO2, QH7: every possible program exit code, including 0, is independent of the marker.
	for code := 0; code <= 255; code++ {
		t.Run(fmt.Sprintf("Code%d", code), func(t *testing.T) {
			ch := plan.Check{Preimage: plan.Object{"predicate": "exit_code", "expected": plan.Object{"literal": strictjson.Number("0")}}}
			for _, marker := range []bool{false, true} {
				e := Evidence{Invocations: []observe.Invocation{{Run: observe.Run{Markers: observe.Markers{Confinement: true, CouldNotStart: marker}, End: observe.End{Code: code}}}}}
				got := level7(ch, passing(), e)
				if marker {
					if got != (Result{Outcome: "not_observed", Reason: "could_not_start", Level: 7}) {
						t.Fatalf("code %d: %+v", code, got)
					}
				} else {
					want := Result{}
					if code != 0 {
						want = Result{Outcome: "not_held", FailureKind: "wrong_exit", Level: 7}
					}
					if got != want {
						t.Fatalf("program exit %d got %+v want %+v", code, got, want)
					}
				}
			}
		})
	}
}

func TestBudgetControl(t *testing.T) {
	// QO4, QH24: both running-at-budget and the known-fast control are required.
	for _, running := range []bool{false, true} {
		for _, fast := range []bool{false, true} {
			t.Run(fmt.Sprintf("Running%tFast%t", running, fast), func(t *testing.T) {
				c := passing()
				c.KnownFastWithin = fast
				e := Evidence{Invocations: []observe.Invocation{{Run: observe.Run{Markers: observe.Markers{Confinement: true}, End: observe.End{Signal: 9, EngineKilled: true, BudgetExpired: true, RunningAtBudget: running}}}}}
				want := Result{Outcome: "not_observed", Reason: "harness_fault", Level: 7}
				if running && fast {
					want = Result{Outcome: "not_held", FailureKind: "over_budget", Level: 7}
				}
				if got := level7(plan.Check{Preimage: plan.Object{"probe": nil}}, c, e); got != want {
					t.Fatalf("got %+v want %+v", got, want)
				}
			})
		}
	}
}

func TestChangedCopy(t *testing.T) {
	// QH8: any invocation using a changed copy fails; a check without invocations passes this level.
	for _, n := range []int{0, 1, 2} {
		for _, unchanged := range []bool{false, true} {
			t.Run(fmt.Sprintf("Keys%dUnchanged%t", n, unchanged), func(t *testing.T) {
				ch := plan.Check{Keys: make([]plan.Object, n)}
				c := passing()
				c.PrivateCopyUnchanged = unchanged
				want := Result{}
				if n > 0 && !unchanged {
					want = Result{Outcome: "not_observed", Reason: "harness_fault", Level: 5}
				}
				if got := level5(ch, c, Evidence{}); got != want {
					t.Fatalf("got %+v want %+v", got, want)
				}
			})
		}
	}
	for _, index := range []int{0, 1} {
		t.Run(fmt.Sprintf("ChangedInvocation%d", index), func(t *testing.T) {
			c := passing()
			e := Evidence{Invocations: []observe.Invocation{{}, {}}}
			e.Invocations[index].Run.CopyChanged = true
			got := level5(plan.Check{Keys: []plan.Object{{}, {}}}, c, e)
			if got != (Result{Outcome: "not_observed", Reason: "harness_fault", Level: 5}) {
				t.Fatal(got)
			}
		})
	}

}

func TestAcceptedExitCodes(t *testing.T) {
	// QO5: all four bullets, with QP2, QP4 and QS6 supplying their accepted codes.
	for _, row := range []struct {
		name    string
		pre     plan.Object
		subject plan.Object
		accept  int
	}{
		{"Predicate", plan.Object{"predicate": "exit_code", "expected": plan.Object{"literal": strictjson.Number("2")}}, nil, 2},
		{"Bare", plan.Object{"probe": "bare", "parameters": plan.Object{"exit_codes": plan.Array{strictjson.Number("3")}}}, nil, 3},
		{"DeclaredExitProbe", plan.Object{"probe": "exit_codes", "parameters": plan.Object{"exit_codes": plan.Array{strictjson.Number("4")}}}, nil, 4},
		{"AuthoredOwn", plan.Object{"probe": nil, "parameters": plan.Object{"exit_codes": plan.Array{strictjson.Number("5")}}}, nil, 5},
		{"Command", plan.Object{"probe": nil, "words": plan.Array{"read", "all"}}, plan.Object{"commands": plan.Array{plan.Object{"words": plan.Array{}, "exit_codes": plan.Array{strictjson.Number("8")}}, plan.Object{"words": plan.Array{"read"}, "exit_codes": plan.Array{strictjson.Number("6")}}}}, 6},
		{"CommandReverse", plan.Object{"probe": nil, "words": plan.Array{"read", "all"}}, plan.Object{"commands": plan.Array{plan.Object{"words": plan.Array{"read"}, "exit_codes": plan.Array{strictjson.Number("6")}}, plan.Object{"words": plan.Array{}, "exit_codes": plan.Array{strictjson.Number("8")}}}}, 6},
		{"NullCommandCodes", plan.Object{"probe": nil, "words": plan.Array{"read"}}, plan.Object{"commands": plan.Array{plan.Object{"words": plan.Array{"read"}, "exit_codes": nil}}}, 0},
		{"UndeclaredBare", plan.Object{"probe": "bare", "parameters": plan.Object{"exit_codes": nil}}, nil, 0},
	} {
		t.Run(row.name, func(t *testing.T) {
			ch := plan.Check{Preimage: row.pre, Subject: row.subject}
			for _, code := range []int{row.accept, 99} {
				got := InvocationOutcome(ch, passing(), observe.Run{Markers: observe.Markers{Confinement: true}, End: observe.End{Code: code}})
				want := Result{}
				if code != row.accept {
					want = Result{Outcome: "not_held", FailureKind: "wrong_exit", Level: 7}
				}
				if got != want {
					t.Fatalf("code %d got %+v want %+v", code, got, want)
				}
			}
		})
	}
}

func TestOtherInvocationEnds(t *testing.T) {
	// QO3, QO5: confinement kills are not crashes; other signals and unsuccessful exits are.
	for _, row := range []struct {
		name   string
		end    observe.End
		out    Outcome
		reason Reason
	}{
		{"Confinement", observe.End{Signal: 9}, "not_observed", "network_unverifiable"},
		{"Crash", observe.End{Signal: 11}, "not_held", "crashed"},
		{"Memory", observe.End{Signal: 9, ExitDetail: 0x20000}, "not_held", "crashed"},
		{"Signing", observe.End{Signal: 9, ExitDetail: 0x40000}, "not_held", "crashed"},
		{"EngineOutsideBudget", observe.End{Signal: 9, EngineKilled: true}, "not_observed", "harness_fault"},
		{"PanicExit", observe.End{Code: 2}, "not_held", "wrong_exit"},
	} {
		t.Run(row.name, func(t *testing.T) {
			got := InvocationOutcome(plan.Check{Preimage: plan.Object{}}, passing(), observe.Run{Markers: observe.Markers{Confinement: true}, End: row.end})
			want := Result{Outcome: row.out, Level: 7}
			if row.out == "not_held" {
				want.FailureKind = string(row.reason)
			} else {
				want.Reason = row.reason
			}
			if got != want {
				t.Fatal(got)
			}
		})
	}
}

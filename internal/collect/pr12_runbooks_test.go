package collect

import (
	"reflect"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/rulebook"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func loopCheck() plan.Object {
	return plan.Object{"dimension": "response-correctness", "predicate": "stdout_equals", "expected": plan.Object{"literal": "value"}, "path": nil, "precondition": nil, "exit_codes": nil}
}
func loopComp(name string) plan.Object {
	return plan.Object{"argv": plan.Array{"/bin/" + name}, "budget_seconds": strictjson.Number("1"), "captures": plan.Array{}, "declared_files": plan.Array{}}
}
func loopStep(name string, checks ...plan.Object) plan.Object {
	a := plan.Array{}
	for _, ch := range checks {
		a = append(a, ch)
	}
	return plan.Object{"name": name, "words": plan.Array{name}, "budget_seconds": strictjson.Number("1"), "checks": a}
}
func loopPlan(t *testing.T, steps plan.Array) plan.Plan {
	t.Helper()
	s := plan.Object{"commands": plan.Array{plan.Object{"words": plan.Array{}, "class": "safe", "writes": false, "exit_codes": plan.Array{strictjson.Number("0")}}}, "runbooks": plan.Array{plan.Object{"name": "book", "steps": steps}}, "offline": plan.Array{}}
	p, err := plan.Build(plan.Input{Subject: s, Scopes: []rulebook.Scope{rulebook.ConformanceScope}})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for n < len(p.Checks) && p.Checks[n].Preimage["probe"] != nil {
		n++
	}
	p.Checks = p.Checks[n:]
	for _, book := range p.Runbooks {
		for _, step := range book.Steps {
			for i := range step.Required {
				step.Required[i] -= n
			}
		}
	}
	return p
}

type loopSource struct {
	keys     []observe.Key
	controls observe.Controls
	run      func(observe.Key) observe.Run
}

func (s *loopSource) Controls() (observe.Controls, error) { return s.controls, nil }
func (s *loopSource) Observe(k observe.Key, _ observe.Spec) (observe.Run, error) {
	s.keys = append(s.keys, k)
	return s.run(k), nil
}
func cleanLoopRun() observe.Run {
	return observe.Run{Markers: observe.Markers{Confinement: true}, Stdout: []byte("value"), StdoutSeen: 5}
}
func loopSourceWith(run func(observe.Key) observe.Run) *loopSource {
	return &loopSource{controls: goodControls(), run: run}
}
func stepsServed(s *loopSource) []any {
	var names []any
	for _, k := range s.keys {
		if k["computation"] == nil {
			names = append(names, k["step"])
		}
	}
	return names
}

func TestRunbookStops(t *testing.T) {
	// QA7: every non-held pre-override step stops later invocations.
	for _, name := range []string{"Predicate", "Crashed", "OverBudget", "CouldNotStart", "ComputationFailed", "LevelTwo"} {
		t.Run(name, func(t *testing.T) {
			first := loopCheck()
			if name == "ComputationFailed" {
				first["expected"] = plan.Object{"computation": loopComp("compute")}
			}
			p := loopPlan(t, plan.Array{loopStep("first", first), loopStep("second", loopCheck())})
			if name == "LevelTwo" {
				p.Checks[0].Subject["commands"] = append(discoveryArray(p.Checks[0].Subject["commands"]), plan.Object{"words": plan.Array{"first"}, "class": plan.Object{"placeholder": "supply class"}})
			}
			source := loopSourceWith(func(k observe.Key) observe.Run {
				r := cleanLoopRun()
				if k["step"] == "first" {
					switch name {
					case "Predicate":
						r.Stdout = []byte("other")
					case "Crashed":
						r.End.Signal = 11
					case "OverBudget":
						r.End = observe.End{Signal: 9, EngineKilled: true, BudgetExpired: true, RunningAtBudget: true}
					case "CouldNotStart":
						r.Markers.CouldNotStart = true
					case "ComputationFailed":
						if k["computation"] != nil {
							r.End.Code = 1
						}
					}
				}
				return r
			})
			source.controls.KnownFastWithin = true
			got, err := Collect(p, source)
			want := []any{"first"}
			if name == "LevelTwo" {
				want = nil
			}
			if err != nil || len(got) != 2 || got[1].Result.Level != 4 || got[1].Result.Reason != "earlier_step_failed" || !reflect.DeepEqual(stepsServed(source), want) {
				t.Fatal(got, err, stepsServed(source))
			}
		})
	}
}

func TestOutsideStepRuns(t *testing.T) {
	// QA9, QA7, E-49: an outside check is not evaluated and its step never stops the book.
	for _, name := range []string{"Run", "Stopped", "Confinement", "SelfTest", "System", "Copy", "Placeholder", "Destructive"} {
		t.Run(name, func(t *testing.T) {
			outside := loopCheck()
			outside["dimension"] = "destructive-guard"
			p := loopPlan(t, plan.Array{loopStep("first", loopCheck()), loopStep("outside", outside), loopStep("last", loopCheck())})
			if len(p.Runbooks) != 1 || len(p.Runbooks[0].Steps[1].Outside) != 1 || len(p.Runbooks[0].Steps[1].Required) != 0 {
				t.Fatal(p.Runbooks)
			}
			source := loopSourceWith(func(k observe.Key) observe.Run {
				r := cleanLoopRun()
				if k["step"] == "outside" || name == "Stopped" {
					r.End.Code = 1
				}
				return r
			})
			want := []any{"first", "outside", "last"}
			switch name {
			case "Stopped":
				want = []any{"first"}
			case "Confinement":
				source.controls.ConfinementApplied = false
				want = nil
			case "SelfTest":
				source.controls.SelfTestPassed = false
				want = nil
			case "System":
				source.controls.SystemNamed = false
				want = nil
			case "Copy":
				source.controls.PrivateCopyUnchanged = false
				want = nil
			case "Placeholder":
				p.Checks[0].Subject["commands"] = append(discoveryArray(p.Checks[0].Subject["commands"]), plan.Object{"words": plan.Array{"outside"}, "writes": plan.Object{"placeholder": "supply writes"}})
				want = []any{"first", "last"}
			case "Destructive":
				discoveryObject(discoveryArray(p.Checks[0].Subject["commands"])[0])["class"] = "destructive"
				want = nil
			}
			got, err := Collect(p, source)
			if err != nil || len(got) != 2 || !reflect.DeepEqual(stepsServed(source), want) {
				t.Fatal(got, err, stepsServed(source), want)
			}
			if name == "Run" && got[1].Result.Outcome != "held" {
				t.Fatal(got)
			}
		})
	}
}

func TestCapturePlaceholderLoop(t *testing.T) {
	// QS5, QA7, E-51: captures depend on placeholders with and without a required earlier check.
	for _, required := range []bool{false, true} {
		first := loopCheck()
		if !required {
			first["dimension"] = "destructive-guard"
		}
		last := loopCheck()
		comp := loopComp("compute")
		comp["captures"] = plan.Array{"first"}
		last["expected"] = plan.Object{"computation": comp}
		for _, field := range []string{"class", "writes"} {
			t.Run(map[bool]string{false: "Outside", true: "Required"}[required]+"_"+field, func(t *testing.T) {
				p := loopPlan(t, plan.Array{loopStep("first", first), loopStep("last", last)})
				control := loopSourceWith(func(k observe.Key) observe.Run {
					r := cleanLoopRun()
					if k["computation"] != nil {
						r.Details.Printed = &observe.PrintedValue{Value: "value"}
					}
					return r
				})
				positive, err := Collect(p, control)
				if err != nil || len(control.keys) != 3 {
					t.Fatal(positive, err, control.keys)
				}
				for _, ch := range positive {
					if ch.Result.Outcome != "held" {
						t.Fatal(ch)
					}
				}
				p.Checks[0].Subject["commands"] = append(discoveryArray(p.Checks[0].Subject["commands"]), plan.Object{"words": plan.Array{"first"}, field: plan.Object{"placeholder": "supply value"}})
				source := loopSourceWith(func(observe.Key) observe.Run { t.Fatal("placeholder served"); return observe.Run{} })
				got, err := Collect(p, source)
				if err != nil || len(got) == 0 {
					t.Fatal(got, err)
				}
				for _, ch := range got {
					if ch.Result.Level != 2 || ch.Result.Reason != "placeholder_unfilled" {
						t.Fatal(ch)
					}
				}
			})
		}
	}
}

func TestSharedComputations(t *testing.T) {
	// QA7, QA4, E-52: one step, precondition first, and equal computations run once.
	a, b := loopCheck(), loopCheck()
	for _, ch := range []plan.Object{a, b} {
		ch["predicate"] = "row_count"
		ch["path"] = ""
		ch["expected"] = plan.Object{"computation": loopComp("expected")}
		ch["precondition"] = plan.Object{"computation": loopComp("precondition")}
	}
	p := loopPlan(t, plan.Array{loopStep("first", a, b)})
	source := loopSourceWith(func(k observe.Key) observe.Run {
		r := cleanLoopRun()
		r.Stdout = []byte("[]")
		r.StdoutSeen = 2
		if k["computation"] != nil {
			var value any = strictjson.Number("0")
			if reflect.DeepEqual(k["computation"], plan.Array{"/bin/precondition"}) {
				value = true
			}
			r.Details.Printed = &observe.PrintedValue{Value: value}
		}
		return r
	})
	got, err := Collect(p, source)
	if err != nil || len(source.keys) != 3 || !reflect.DeepEqual(source.keys[1]["computation"], plan.Array{"/bin/precondition"}) || !reflect.DeepEqual(source.keys[2]["computation"], plan.Array{"/bin/expected"}) {
		t.Fatal(got, err, source.keys)
	}
	for _, ch := range got {
		if ch.Result.Outcome != "held" || ch.Result.PreconditionObserved == nil || !*ch.Result.PreconditionObserved || len(ch.Invocations) != 3 {
			t.Fatal(ch)
		}
	}
}

func TestCaptureOwnRuns(t *testing.T) {
	// QA4, QO1, QO5, QA7: captured exit codes are not this check's exit; own QO1 precedes a cut capture.
	for _, name := range []string{"CapturedWrongExit", "Cut", "CutUnconfined", "OwnCopyChanged"} {
		t.Run(name, func(t *testing.T) {
			first := loopCheck()
			first["dimension"] = "destructive-guard"
			last := loopCheck()
			comp := loopComp("compute")
			comp["captures"] = plan.Array{"first"}
			last["expected"] = plan.Object{"computation": comp}
			p := loopPlan(t, plan.Array{loopStep("first", first), loopStep("last", last)})
			source := loopSourceWith(func(k observe.Key) observe.Run {
				r := cleanLoopRun()
				if k["step"] == "first" {
					r.End.Code = 17
					if name != "CapturedWrongExit" {
						r.StdoutCut = true
						r.StdoutSeen = (1 << 20) + 1
					}
				}
				if k["step"] == "last" && name == "CutUnconfined" {
					r.Markers.Confinement = false
				}
				if k["step"] == "last" && name == "OwnCopyChanged" {
					r.CopyChanged = true
				}
				if k["computation"] != nil {
					r.Details.Printed = &observe.PrintedValue{Value: "value"}
				}
				return r
			})
			got, err := Collect(p, source)
			level := 6
			outcome := "not_observed"
			reason := "harness_fault"
			runs := 1
			switch name {
			case "CapturedWrongExit":
				level = 8
				outcome = "held"
				reason = ""
				runs = 2
			case "CutUnconfined":
				level = 1
				reason = "network_unverifiable"
			case "OwnCopyChanged":
				level = 5
			}
			if err != nil || len(got) != 1 || got[0].Result.Level != level || string(got[0].Result.Outcome) != outcome || string(got[0].Result.Reason) != reason || len(got[0].Invocations) != runs {
				t.Fatal(got, err)
			}
			if len(source.keys) != runs+1 {
				t.Fatal(source.keys)
			}
		})
	}
}

func TestOverridesDoNotStopRunbook(t *testing.T) {
	// QS5, QA7: a grade-wide placeholder applies only after all runbooks have run.
	p := loopPlan(t, plan.Array{loopStep("first", loopCheck()), loopStep("last", loopCheck())})
	p.Checks[0].Subject["facts"] = plan.Object{"other": plan.Object{"placeholder": "supply fact"}}
	source := loopSourceWith(func(observe.Key) observe.Run { return cleanLoopRun() })
	got, err := Collect(p, source)
	if err != nil || !reflect.DeepEqual(stepsServed(source), []any{"first", "last"}) {
		t.Fatal(got, err, source.keys)
	}
	for _, ch := range got {
		if ch.Result.Outcome != "not_observed" || ch.Result.Reason != "placeholder_unfilled" {
			t.Fatal(ch)
		}
	}
}

func TestSharedStepBeforeComputations(t *testing.T) {
	// QA7, QO1, QS5, E-51: every check judges its shared invocation, including a sibling blocked before serving.
	outside := loopCheck()
	outside["dimension"] = "destructive-guard"
	capture := loopCheck()
	comp := loopComp("compute")
	comp["captures"] = plan.Array{"outside"}
	capture["expected"] = plan.Object{"computation": comp}
	p := loopPlan(t, plan.Array{loopStep("outside", outside), loopStep("last", capture, loopCheck())})
	p.Checks[0].Subject["commands"] = append(discoveryArray(p.Checks[0].Subject["commands"]), plan.Object{"words": plan.Array{"outside"}, "class": plan.Object{"placeholder": "supply class"}})
	source := loopSourceWith(func(k observe.Key) observe.Run {
		if k["step"] != "last" || k["computation"] != nil {
			t.Fatal("unexpected run", k)
		}
		return observe.Run{}
	})
	got, err := Collect(p, source)
	if err != nil || len(got) != 2 || len(source.keys) != 1 {
		t.Fatal(got, err, source.keys)
	}
	for _, ch := range got {
		if ch.Result.Level != 1 || ch.Result.Reason != "network_unverifiable" || len(ch.Invocations) != 1 {
			t.Fatal(ch)
		}
	}
}

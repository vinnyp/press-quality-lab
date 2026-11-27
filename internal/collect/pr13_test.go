package collect

import (
	"errors"
	"reflect"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/rulebook"
)

func authoredPlan(t *testing.T, sub plan.Object, scopes []rulebook.Scope) plan.Plan {
	t.Helper()
	p, err := plan.Build(plan.Input{Subject: sub, Scopes: scopes})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for n < len(p.Checks) && p.Checks[n].Preimage["probe"] != nil {
		n++
	}
	p.Checks = p.Checks[n:]
	for _, b := range p.Runbooks {
		for _, st := range b.Steps {
			for i := range st.Required {
				st.Required[i] -= n
			}
		}
	}
	return p
}

func TestDestructiveRunbooks(t *testing.T) {
	// QA7, QA8, QS5, QS6, QO1, E-15, E-54: scope gates the whole book; precedence still decides each check.
	for _, name := range []string{"Inside", "Outside", "Stopped", "GradeWide", "WritesPlaceholder", "FactFalse", "FactPlaceholder", "ClassPlaceholder", "ClassPlaceholderInside", "SafeWritesPlaceholder"} {
		t.Run(name, func(t *testing.T) {
			base := loopPlan(t, plan.Array{loopStep("first", loopCheck()), loopStep("remove", loopCheck()), loopStep("last", loopCheck())})
			sub := base.Checks[0].Subject
			class := any("destructive")
			if name == "ClassPlaceholder" || name == "ClassPlaceholderInside" {
				class = plan.Object{"placeholder": "supply class"}
			}
			command := plan.Object{"words": plan.Array{"remove"}, "class": class, "writes": true}
			sub["commands"] = append(discoveryArray(sub["commands"]), command)
			sub["facts"] = plan.Object{"has_destructive_ops": name != "FactFalse"}
			if name == "FactPlaceholder" {
				discoveryObject(sub["facts"])["has_destructive_ops"] = plan.Object{"placeholder": "supply fact"}
			}
			if name == "SafeWritesPlaceholder" {
				command["class"] = "safe"
			}
			if name == "WritesPlaceholder" || name == "SafeWritesPlaceholder" {
				command["writes"] = plan.Object{"placeholder": "supply writes"}
			}
			scopes := []rulebook.Scope{"conformance", "destructive"}
			if name == "Outside" || name == "ClassPlaceholder" || name == "SafeWritesPlaceholder" {
				scopes = scopes[:1]
			}
			p := authoredPlan(t, sub, scopes)
			source := loopSourceWith(func(k observe.Key) observe.Run {
				r := cleanLoopRun()
				if name == "Stopped" && k["step"] == "first" {
					r.Stdout = []byte("other")
				}
				return r
			})
			if name == "GradeWide" {
				source.controls.SelfTestPassed = false
			}
			got, err := Collect(p, source)
			if err != nil {
				t.Fatal(err)
			}
			want := []any{"first", "remove", "last"}
			switch name {
			case "Outside", "GradeWide", "ClassPlaceholder":
				want = nil
			case "Stopped", "WritesPlaceholder", "ClassPlaceholderInside", "SafeWritesPlaceholder":
				want = want[:1]
			}
			if !reflect.DeepEqual(stepsServed(source), want) {
				t.Fatal(stepsServed(source), want)
			}
			if name == "Outside" {
				if len(got) != 0 || len(p.Runbooks) != 0 {
					t.Fatal(got, p.Runbooks)
				}
				return
			}
			if len(got) != 3 {
				t.Fatal(got)
			}
			if name == "Stopped" && got[1].Result.Reason != "earlier_step_failed" {
				t.Fatal(got)
			}
			if name == "GradeWide" {
				for _, ch := range got {
					if ch.Result.Level != 1 || ch.Result.Reason != "network_unverifiable" {
						t.Fatal(ch)
					}
				}
			}
			if name == "ClassPlaceholder" {
				for _, ch := range got {
					if ch.Result.Level != 2 || ch.Result.Reason != "placeholder_unfilled" {
						t.Fatal(ch)
					}
				}
			}
			if name == "WritesPlaceholder" || name == "ClassPlaceholderInside" || name == "SafeWritesPlaceholder" {
				if got[1].Result.Level != 2 || got[1].Result.Reason != "placeholder_unfilled" || got[2].Result.Reason != "earlier_step_failed" {
					t.Fatal(got)
				}
			}
			if name == "Inside" || name == "FactFalse" {
				for _, ch := range got {
					if ch.Result.Outcome != "held" {
						t.Fatal(ch)
					}
				}
			}
		})
	}
}

func TestDestructivePreflight(t *testing.T) {
	// QS6, QE3, QA8, E-49: a corrupt plan is refused before discovery, including unchecked steps.
	for _, outside := range []bool{false, true} {
		check := loopCheck()
		if outside {
			check["dimension"] = "destructive-guard"
		}
		p := loopPlan(t, plan.Array{loopStep("first", loopCheck()), loopStep("remove", check)})
		p.Checks = append(p.Checks, discoveryCheck())
		source := loopSourceWith(func(observe.Key) observe.Run { return cleanLoopRun() })
		positive, err := Collect(p, source)
		if err != nil || len(source.keys) != 3 {
			t.Fatal(positive, err, source.keys)
		}
		for _, ch := range positive {
			if ch.Result.Outcome != "held" {
				t.Fatal(ch)
			}
		}
		source.keys = nil
		p.Checks[0].Subject["commands"] = append(discoveryArray(p.Checks[0].Subject["commands"]), plan.Object{"words": plan.Array{"remove"}, "class": "destructive"})
		got, err := Collect(p, source)
		var fault *plan.Fault
		if !errors.As(err, &fault) || fault.Rule != "QS6" || got != nil || len(source.keys) != 0 {
			t.Fatal(got, err, source.keys)
		}
	}
}

type changingControls struct {
	*loopSource
	change func()
}

func (s changingControls) Controls() (observe.Controls, error) { s.change(); return s.controls, nil }

func TestServeSafetyBrakes(t *testing.T) {
	// QS5, QS6, QA8: each serve checks source safety again after startup preflight.
	for _, name := range []string{"Destructive", "ClassPlaceholder", "WritesPlaceholder"} {
		t.Run(name, func(t *testing.T) {
			p := loopPlan(t, plan.Array{loopStep("first", loopCheck())})
			source := loopSourceWith(func(observe.Key) observe.Run { return cleanLoopRun() })
			sub := p.Checks[0].Subject
			cmd := plan.Object{"words": plan.Array{"different"}, "class": "safe", "writes": false}
			sub["commands"] = append(discoveryArray(sub["commands"]), cmd)
			if name != "Destructive" {
				// E-54 must allow the book so only the serve placeholder brake refuses it.
				p.Scopes = append(p.Scopes, "destructive")
				p.Checks[0].Scopes = p.Scopes
				// Preimage dependencies remain safe; a corrupt schedule must still never
				// serve a placeholder key even when level 2 cannot see that corruption.
				p.Runbooks[0].Steps[0].Key["words"] = plan.Array{"different"}
				p.Checks[0].Keys[0]["words"] = plan.Array{"different"}
				book := discoveryObject(discoveryArray(sub["runbooks"])[0])
				discoveryObject(discoveryArray(book["steps"])[0])["words"] = plan.Array{"different"}
			}
			positive, err := Collect(p, source)
			if err != nil || len(positive) != 1 || positive[0].Result.Outcome != "held" || len(source.keys) != 1 {
				t.Fatal(positive, err, source.keys)
			}
			source.keys = nil
			changed := changingControls{loopSource: source, change: func() {
				switch name {
				case "Destructive":
					discoveryObject(discoveryArray(sub["commands"])[0])["class"] = "destructive"
				case "ClassPlaceholder":
					cmd["class"] = plan.Object{"placeholder": "class"}
				case "WritesPlaceholder":
					cmd["writes"] = plan.Object{"placeholder": "writes"}
				}
			}}
			got, err := Collect(p, changed)
			want := "QS5"
			if name == "Destructive" {
				want = "QS6"
			}
			var fault *plan.Fault
			if !errors.As(err, &fault) || fault.Rule != want || got != nil || len(source.keys) != 0 {
				t.Fatal(got, err, source.keys)
			}
		})
	}
}

func TestPlaceholderUncheckedStep(t *testing.T) {
	// QS5, QA7, E-49, E-54: even an unchecked safe step waits for whole-book safety.
	outside := loopCheck()
	outside["dimension"] = "destructive-guard"
	p := loopPlan(t, plan.Array{loopStep("first", outside), loopStep("last", loopCheck())})
	sub := p.Checks[0].Subject
	sub["commands"] = append(discoveryArray(sub["commands"]), plan.Object{"words": plan.Array{"last"}, "class": plan.Object{"placeholder": "class"}})
	source := loopSourceWith(func(observe.Key) observe.Run { return cleanLoopRun() })
	got, err := Collect(p, source)
	if err != nil || len(got) != 1 || got[0].Result.Level != 2 || got[0].Result.Reason != "placeholder_unfilled" || len(source.keys) != 0 {
		t.Fatal(got, err, source.keys)
	}
}

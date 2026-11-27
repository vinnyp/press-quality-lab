package collect

import (
	"errors"
	"reflect"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

type servingSource struct {
	controls observe.Controls
	keys     []observe.Key
	run      observe.Run
	err      error
}

func (s *servingSource) Controls() (observe.Controls, error) { return s.controls, nil }
func (s *servingSource) Observe(k observe.Key, _ observe.Spec) (observe.Run, error) {
	s.keys = append(s.keys, k)
	return s.run, s.err
}

func TestInvocationServing(t *testing.T) {
	// QE8, QO1, QH8, QA7, QO6, QO7: steps serve once; probes retain their own keys.
	for _, name := range []string{"Ordered", "UnconfinedInvocation", "GradeWide", "Placeholder", "DiscoveryWalk", "Destructive", "ChangedCopy", "Computation", "NoInvocation", "SourceFault"} {
		t.Run(name, func(t *testing.T) {
			ch := plan.Check{ID: "synthetic", Subject: plan.Object{}, Preimage: plan.Object{"probe": nil, "predicate": "stdout_equals", "earlier": plan.Array{}, "words": plan.Array{"read"}, "expected": plan.Object{"literal": "value"}}, Keys: []plan.Object{{"words": plan.Array{"read"}, "computation": nil}}}
			p := plan.Plan{Checks: []plan.Check{ch}}
			source := &servingSource{controls: goodControls(), run: cleanLoopRun()}
			want := []observe.Key{map[string]any(ch.Keys[0])}
			level := 8
			switch name {
			case "UnconfinedInvocation":
				source.run = observe.Run{End: observe.End{Signal: 11}}
				level = 1
			case "GradeWide":
				source.controls.SelfTestPassed = false
				want = nil
				level = 1
			case "Placeholder":
				ch.Subject = plan.Object{"binary": plan.Object{"placeholder": "supply binary"}}
				want = nil
				level = 2
			case "DiscoveryWalk":
				ch.Preimage["probe"] = "discovery"
				ch.Subject["declared_count"] = strictjson.Number("0")
				want = []observe.Key{map[string]any(plan.DiscoveryKey(plan.Array{}))}
			case "Destructive":
				p = loopPlan(t, plan.Array{loopStep("first", loopCheck())})
				discoveryObject(discoveryArray(p.Checks[0].Subject["commands"])[0])["class"] = "destructive"
				want = nil
				level = 4
			case "ChangedCopy":
				source.controls.PrivateCopyUnchanged = false
				want = nil
				level = 5
			case "Computation":
				c := loopCheck()
				c["expected"] = plan.Object{"computation": loopComp("compute")}
				p = loopPlan(t, plan.Array{loopStep("first", c)})
				source.run.Details.Printed = &observe.PrintedValue{Value: "value"}
				want = []observe.Key{map[string]any(p.Runbooks[0].Steps[0].Key), map[string]any(p.Checks[0].Keys[1])}
			case "NoInvocation":
				ch.Keys = nil
				defer func() {
					if recover() == nil {
						t.Fatal("missing own invocation accepted")
					}
				}()
			case "SourceFault":
				source.err = errors.New("observation failed")
			}
			if name != "Destructive" && name != "Computation" {
				p.Checks[0] = ch
			}
			got, err := Collect(p, source)
			if name == "SourceFault" {
				if !errors.Is(err, source.err) || got != nil {
					t.Fatal(got, err)
				}
				return
			}
			if err != nil || len(got) != 1 || got[0].Result.Level != level || !reflect.DeepEqual(source.keys, want) {
				t.Fatalf("result %+v err %v keys %+v want %+v level %d", got, err, source.keys, want, level)
			}
		})
	}
}

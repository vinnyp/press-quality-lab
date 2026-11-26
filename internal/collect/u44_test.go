package collect

import (
	"errors"
	"reflect"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
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
	// QE8, QO1, QH8, QA7, QO6, QO7: only built checks reaching level 7 serve their keys, in target order.
	base := plan.Check{ID: "synthetic", Subject: plan.Object{}, Preimage: plan.Object{"probe": nil, "earlier": plan.Array{}, "words": plan.Array{"read"}, "expected": plan.Object{"literal": "value"}}, Keys: []plan.Object{{"words": plan.Array{"first"}}, {"words": plan.Array{"second"}}}}
	for _, name := range []string{"Ordered", "UnconfinedInvocation", "GradeWide", "Placeholder", "DiscoverySentinel", "RunbookSentinel", "ChangedCopy", "ComputationSentinel", "NoInvocation", "SourceFault"} {
		t.Run(name, func(t *testing.T) {
			ch := base
			ch.Preimage = plan.Object{}
			for k, v := range base.Preimage {
				ch.Preimage[k] = v
			}
			source := &servingSource{controls: observe.Controls{ConfinementApplied: true, SelfTestPassed: true, SystemNamed: true, PrivateCopyUnchanged: true}, run: observe.Run{Markers: observe.Markers{Confinement: true}}}
			want := []observe.Key{map[string]any(base.Keys[0]), map[string]any(base.Keys[1])}
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
				ch.Subject = plan.Object{"binary": plan.Object{"placeholder": "Supply binary."}}
				want = nil
				level = 2
			case "DiscoverySentinel":
				ch.Preimage["probe"] = "discovery"
				want = nil
				level = 3
			case "RunbookSentinel":
				ch.Preimage["earlier"] = plan.Array{"prior"}
				want = nil
				level = 4
			case "ChangedCopy":
				source.controls.PrivateCopyUnchanged = false
				want = nil
				level = 5
			case "ComputationSentinel":
				ch.Preimage["expected"] = plan.Object{"computation": plan.Object{"argv": plan.Array{"compute"}}}
				want = nil
				level = 6
			case "NoInvocation":
				ch.Keys = nil
				want = nil
			case "SourceFault":
				source.err = errors.New("observation failed")
			}
			got, err := Collect(plan.Plan{Checks: []plan.Check{ch}}, source)
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

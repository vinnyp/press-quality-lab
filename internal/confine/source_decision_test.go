package confine_test

import (
	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"testing"
)

// Keep the decision and its types untagged; Darwin tests supply raw runs only.
func assertSourceReadDecision(t *testing.T, key observe.Key, run, control observe.Run, held bool) {
	t.Helper()
	run.Details.Planted.ControlEnd = &observe.InvocationProtocol{End: control.End, Markers: control.Markers}
	ch := plan.Check{Subject: plan.Object{"commands": plan.Array{plan.Object{"words": plan.Array{}, "exit_codes": nil}}}, Preimage: plan.Object{"probe": "planted_secret", "expected": nil, "earlier": plan.Array{}, "parameters": plan.Object{"kind": "token"}}}
	c := observe.Controls{ConfinementApplied: true, SelfTestPassed: true, SystemNamed: true, KnownFastWithin: true, ReadCalibrationMoved: true, PrivateCopyUnchanged: true}
	result := decide.Resolve(ch, c, decide.Evidence{Discovery: &decide.Discovery{}, Invocations: []observe.Invocation{{Key: key, Run: run}}})
	if held {
		if result.Outcome != "held" {
			t.Fatalf("read decision %+v", result)
		}
	} else if result.Outcome != "not_observed" || result.Reason != "harness_fault" {
		t.Fatalf("harness-read decision %+v", result)
	}
}

func TestSourceReadDecisionOracle(t *testing.T) {
	// QH27: the raw-fact oracle distinguishes a confinement-killed control from QH28's clean twin.
	run := observe.Run{Markers: observe.Markers{Confinement: true}, Details: observe.RunDetails{Planted: &observe.Planted{AtimeMoved: true, SameFile: true}}}
	killed := observe.Run{Markers: observe.Markers{Confinement: true}, End: observe.End{Code: -1, Signal: 9}}
	clean := observe.Run{Markers: observe.Markers{Confinement: true}}
	assertSourceReadDecision(t, observe.Key{"words": plan.Array{}}, run, killed, true)
	assertSourceReadDecision(t, observe.Key{"words": plan.Array{}}, run, clean, false)
}

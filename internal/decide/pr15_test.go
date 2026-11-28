package decide

import (
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func TestChangedCopyPrecedence(t *testing.T) {
	// QH8, QO1, E-53: a refused copy use has no confinement attempt.
	for _, row := range []struct {
		name             string
		changed, capture bool
		level            int
		reason           Reason
	}{
		{"Changed", true, false, 5, "harness_fault"},
		{"Unconfined", false, false, 1, NetworkUnverifiable},
		{"ChangedCapture", true, true, 5, "harness_fault"},
		{"UnconfinedCapture", false, true, 1, NetworkUnverifiable},
	} {
		t.Run(row.name, func(t *testing.T) {
			ch, facts := computedCheck("exit_code", strictjson.Number("0"))
			ch.Preimage["runbook"] = "book"
			ch.Preimage["step"] = "next"
			ch.Keys = []plan.Object{{"words": plan.Array{}}}
			comp := object(object(ch.Preimage["expected"])["computation"])
			comp["captures"] = plan.Array{"first"}
			facts.Invocations[0].Key["runbook"] = "book"
			facts.Invocations[0].Key["step"] = "next"
			facts.Invocations = append(facts.Invocations, observe.Invocation{Key: observe.Key{"runbook": "book", "step": "next"}, Run: observe.Run{Markers: observe.Markers{Confinement: true}}})
			facts.Captures = []observe.Invocation{{Key: observe.Key{"runbook": "book", "step": "first"}, Run: observe.Run{Markers: observe.Markers{Confinement: true}}}}
			if healthy := Resolve(ch, passing(), facts); healthy.Outcome != "held" {
				t.Fatalf("healthy fixture: %+v", healthy)
			}
			if row.capture {
				facts.Captures[0].Run = observe.Run{CopyChanged: row.changed}
			} else {
				facts.Invocations[1].Run = observe.Run{CopyChanged: row.changed}
			}

			got := Resolve(ch, passing(), facts)
			if got.Level != row.level || got.Outcome != NotObserved || got.Reason != row.reason {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestChangedCopyProtocolFault(t *testing.T) {
	// QH8, QH6: a changed-copy refusal cannot also carry launch facts.
	for _, row := range []struct {
		name string
		run  observe.Run
	}{
		{"Marker", observe.Run{CopyChanged: true, Markers: observe.Markers{Confinement: true}}},
		{"FailureMarker", observe.Run{CopyChanged: true, Markers: observe.Markers{CouldNotStart: true}}},
		{"End", observe.Run{CopyChanged: true, End: observe.End{Code: 1}}},
		{"ZeroExitStream", observe.Run{CopyChanged: true, End: observe.End{Code: 0}, Stdout: []byte("x")}},
		{"Seen", observe.Run{CopyChanged: true, StdoutSeen: 1}},
		{"Details", observe.Run{CopyChanged: true, Details: observe.RunDetails{Planted: &observe.Planted{}}}},
	} {
		t.Run(row.name, func(t *testing.T) {
			ch := plan.Check{Subject: plan.Object{}, Preimage: plan.Object{"probe": "bare"}}
			twin := row.run
			twin.CopyChanged = false
			got := Resolve(ch, passing(), Evidence{Invocations: []observe.Invocation{{Run: twin}}})
			want := 1
			if twin.Markers.Confinement {
				want = 3
			}
			if got.Level != want {
				t.Fatalf("unchanged twin: %+v", got)
			}
			defer func() {
				fault, ok := recover().(*plan.Fault)
				if !ok || fault.Rule != "QH8" || fault.Cause != "changed copy carries launch facts" {
					t.Fatalf("wrong engine fault: %v", fault)
				}
			}()
			Resolve(ch, passing(), Evidence{Invocations: []observe.Invocation{{Run: row.run}}})
		})
	}
}

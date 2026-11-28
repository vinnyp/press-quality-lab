//go:build scripted

package scripted

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/observe"
)

func TestProbeControlFacts(t *testing.T) {
	// QT4, QP9, QP11: host controls replace recorded facts before decisions; E-37 uses attribution.
	recording, err := os.ReadFile("../../docs/specs/conformance-collection-1.0/behaviour/known-good.json")
	if err != nil {
		t.Fatal(err)
	}
	var rec map[string]any
	if err := json.Unmarshal(recording, &rec); err != nil {
		t.Fatal(err)
	}
	invs, ok := rec["invocations"].([]any)
	if !ok {
		t.Fatal("missing invocations")
	}
	for _, detected := range []bool{true, false} {
		host := map[string]any{"confinement": map[string]any{"applied": true, "sandbox_exec_exit": nil}, "self_test": "passed", "known_fast": "within", "read_calibration": "moved", "store_control": "not_detected", "read_control": "not_killed", "private_copy": "unchanged", "kill_attribution": []any{}}
		if detected {
			host["store_control"] = "detected"
			host["read_control"] = "killed"
		}
		h, err := json.Marshal(host)
		if err != nil {
			t.Fatal(err)
		}
		source, err := Read(h, recording, nil)
		if err != nil {
			t.Fatal(err)
		}
		homeFound, plantedFound := false, false
		for _, v := range invs {
			inv, ok := v.(map[string]any)
			if !ok {
				t.Fatal("invocation shape")
			}
			key, ok := inv["key"].(map[string]any)
			if !ok {
				t.Fatal("key shape")
			}
			id, err := externalKeyID(key)
			if err != nil {
				t.Fatal(err)
			}
			original := source.runs[id]
			if original.Details.Home.ControlDetected != nil {
				opposite := !detected
				original.Details.Home.ControlDetected = &opposite
				source.runs[id] = original
				r, err := source.Observe(key, observe.Spec{StoreControl: &opposite})
				if err != nil {
					t.Fatal(err)
				}
				if r.Details.Home.ControlDetected == nil || *r.Details.Home.ControlDetected != detected {
					t.Fatal("recording defeated host store control")
				}
				r, err = source.Observe(key, observe.Spec{})
				if err != nil || r.Details.Home.ControlDetected != nil {
					t.Fatalf("nil store control %+v %v", r.Details.Home, err)
				}
				homeFound = true
			}
			if original.Details.Planted != nil && original.Details.Planted.ControlEnd != nil {
				end := observe.End{}
				if !detected {
					end.Signal = 9
				}
				original.Details.Planted.ControlEnd = &observe.InvocationProtocol{End: end, Markers: observe.Markers{Confinement: true}}
				source.runs[id] = original
				r, err := source.Observe(key, observe.Spec{})
				if err != nil {
					t.Fatal(err)
				}
				var trace bytes.Buffer
				source.trace = NewTrace(&trace)
				if err := source.Required(array{}); err != nil {
					t.Fatal(err)
				}
				before := trace.String()
				if r.Details.Planted.ControlEnd != nil {
					t.Fatal("ordinary run retained control")
				}
				r, err = source.Observe(key, observe.Spec{Control: true})
				if err != nil {
					t.Fatal(err)
				}
				if trace.String() != before {
					t.Fatal("control counted as served")
				}
				if _, err := source.Observe(key, observe.Spec{}); err != nil {
					t.Fatal(err)
				}
				if trace.String() == before {
					t.Fatal("ordinary run missing served event")
				}
				source.trace = NewTrace(nil)
				by := "exit"
				if detected {
					by = "confinement"
				}
				CheckAttribution(t, r, by)
				plantedFound = true
			}
		}
		if !homeFound || !plantedFound {
			t.Fatalf("fixture missing control subjects: %v %v", homeFound, plantedFound)
		}
	}
}

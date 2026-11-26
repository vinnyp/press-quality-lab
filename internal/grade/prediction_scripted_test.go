//go:build scripted

package grade

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/caseassembly"
	"github.com/vinnyp/press-quality-lab/internal/collect"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/scripted"
	"github.com/vinnyp/press-quality-lab/internal/subject"
)

type predictedEnding struct {
	Phase string   `json:"phase"`
	Rules []string `json:"rules"`
}

func allowedRule(e predictedEnding, rule string) bool {
	for _, r := range e.Rules {
		base, _, _ := strings.Cut(r, ".")
		if base == rule {
			return true
		}
	}
	return false
}
func TestPredictedCheckEndings(t *testing.T) {
	// QO1, QT6: independent predictions check every resolution, including pending checks.
	const kit = "../../docs/specs/conformance-collection-1.0"
	var predictions struct {
		Cases []struct {
			Case   string          `json:"case"`
			Kind   string          `json:"kind"`
			Ending predictedEnding `json:"ending"`
			Checks []struct {
				ID     string          `json:"id"`
				Ending predictedEnding `json:"ending"`
			} `json:"checks"`
		} `json:"cases"`
	}
	if err := caseassembly.Read("../../scripts/ratchet/pr6-prediction.json", &predictions); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, prediction := range predictions.Cases {
		if prediction.Ending.Phase == "excluded" {
			continue
		}
		t.Run(strings.ReplaceAll(prediction.Case, "-", "_"), func(t *testing.T) {
			source := filepath.Join(kit, "cases", prediction.Case)
			var c caseassembly.Case
			if err := caseassembly.Read(filepath.Join(source, "case.json"), &c); err != nil {
				t.Fatal(err)
			}
			layout := caseassembly.Assemble(t, kit, source, c)
			data, err := os.ReadFile(filepath.Join(layout.Subject, c.Input.SubjectFile))
			if err != nil {
				t.Fatal(err)
			}
			s, refusal := subject.Parse(data)
			if refusal != nil {
				if prediction.Kind != "derivation-only" || prediction.Ending.Phase == "unrefused" || !allowedRule(prediction.Ending, refusal.Rule) {
					t.Fatalf("subject refusal %v; want %+v", refusal, prediction.Ending)
				}
				counts["derivation"]++
				return
			}
			o := Options{SubjectFile: filepath.Join(layout.Subject, c.Input.SubjectFile), Scope: c.Input.Scope, Baseline: layout.Baseline, Anchor: c.Input.Request == "anchor", Scripted: true}
			in, err := planningInput(o, s)
			if err != nil {
				var fault *Error
				if prediction.Ending.Phase != "safe_read" || !errors.As(err, &fault) || fault.Code != 70 || !allowedRule(prediction.Ending, fault.Rule) {
					t.Fatalf("snapshot %v; want %+v", err, prediction.Ending)
				}
				counts["safe_read"]++
				return
			}
			if prediction.Ending.Phase == "safe_read" {
				t.Fatal("predicted safe-read refusal reached planning")
			}
			p, err := plan.Build(in.Input)
			if err != nil {
				var gap *plan.Gap
				if !errors.As(err, &gap) || !allowedRule(prediction.Ending, gap.Rule) {
					t.Fatalf("planning %v; want %+v", err, prediction.Ending)
				}
				if prediction.Kind == "derivation-only" {
					counts["derivation"]++
				} else {
					counts["planning"]++
				}
				return
			}
			if prediction.Ending.Phase == "planning" {
				t.Fatal("predicted planning gap reached collection")
			}
			host, err := json.Marshal(c.Input.Host)
			if err != nil {
				t.Fatal(err)
			}
			recording, err := os.ReadFile(filepath.Join(kit, "behaviour", c.Input.Form+".json"))
			if err != nil {
				t.Fatal(err)
			}
			raw, err := scripted.Read(host, recording, nil)
			if err != nil {
				t.Fatal(err)
			}
			if prediction.Kind == "derivation-only" {
				// QE6: predicted refusals stop at the pre-check; pending cases follow the real loop.
				var output strings.Builder
				dependencies := Dependencies{
					IDs:       func() (int, int) { return 1000, 1000 },
					StoreRoot: func() (string, error) { t.Fatal("derivation reached store"); return "", nil },
					Source:    func() (observe.Source, func() error, error) { return raw, func() error { return nil }, nil },
				}
				code, err := Run(o, dependencies, &output)
				var refusal *Error
				wantCode := 2
				if prediction.Ending.Phase == "unrefused" {
					wantCode = 70
				}
				if code != wantCode || !errors.As(err, &refusal) || !allowedRule(prediction.Ending, refusal.Rule) {
					t.Fatalf("derivation code=%d err=%v; want %+v", code, err, prediction.Ending)
				}
				counts["derivation"]++
				return
			}
			checks, err := gather(collect.Collect, p, raw)
			if err != nil {
				t.Fatal(err)
			}
			gotIDs, wantIDs := []string{}, []string{}
			for _, check := range checks {
				gotIDs = append(gotIDs, check.Plan.ID)
			}
			for _, check := range prediction.Checks {
				wantIDs = append(wantIDs, check.ID)
			}
			slices.Sort(gotIDs)
			slices.Sort(wantIDs)
			if !reflect.DeepEqual(gotIDs, wantIDs) {
				t.Fatalf("check ids got %v want %v", gotIDs, wantIDs)
			}
			for _, check := range checks {
				for _, want := range prediction.Checks {
					if check.Plan.ID != want.ID {
						continue
					}
					if want.Ending.Phase == "deciding" {
						if check.Result.Unbuilt != nil || check.Result.Level != 1 || check.Result.Outcome != "not_observed" || check.Result.Reason != "network_unverifiable" {
							t.Errorf("check %s: %+v; want QO1", want.ID, check.Result)
						}
					} else if check.Result.Unbuilt == nil || !allowedRule(want.Ending, check.Result.Unbuilt.Rule) {
						t.Errorf("check %s: %+v; want %+v", want.ID, check.Result, want.Ending)
					}
				}
			}
			counts["decide"]++
		})
	}
	if !reflect.DeepEqual(counts, map[string]int{"decide": 94, "planning": 24, "derivation": 29, "safe_read": 2}) {
		t.Fatalf("phase counts %v", counts)
	}
}

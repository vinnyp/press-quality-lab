//go:build scripted

package grade

import (
	"bytes"
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
	"github.com/vinnyp/press-quality-lab/internal/throwaway"
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
				Level  int             `json:"deciding_level"`
				ID     string          `json:"id"`
				Ending predictedEnding `json:"ending"`
			} `json:"checks"`
		} `json:"cases"`
	}
	if err := caseassembly.Read("../../scripts/ratchet/pr8-prediction.json", &predictions); err != nil {
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
			// U9 moves all subject-file derivation refusals ahead of planning.
			// Judge-checked endings follow the PR8 prediction.
			expectedRule := stringValue(c.Expect["rule"])
			if prediction.Kind == "derivation-only" && (strings.HasPrefix(expectedRule, "QS") || expectedRule == "QP7" || expectedRule == "QP10" || expectedRule == "QA4" || expectedRule == "QA9") {
				options := Options{SubjectFile: filepath.Join(layout.Subject, c.Input.SubjectFile), Scope: c.Input.Scope, Scripted: true}
				var output bytes.Buffer
				code, err := Run(options, Dependencies{IDs: func() (int, int) { return 1000, 1000 }}, &output)
				var refusal *Error
				if code != 2 || !errors.As(err, &refusal) || refusal.Rule != expectedRule || output.Len() != 0 {
					t.Fatalf("U9: code=%d error=%v output=%q; want exit 2 %s", code, err, output.String(), expectedRule)
				}
				counts["derivation"]++
				return
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
				workspaceParent := t.TempDir()
				dependencies := Dependencies{
					Workspace: func() (*throwaway.Workspace, error) {
						return throwaway.BeginAt(filepath.Join(workspaceParent, "workspaces"))
					},
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
			checks, _, err := gatherSignals(collect.Collect, p, raw, nil, 0)
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
					if len(want.Ending.Rules) == 1 && want.Ending.Rules[0] == "QH8" {
						// QH8, QO2 to QO5: U44 replaces PR8's level-5 sentinel.
						// Keep the PR8 reader until U11; take changed expectations from the sealed case.
						var document struct {
							Evidence []struct {
								ID      string `json:"check_id"`
								Outcome string `json:"outcome"`
								Reason  string `json:"reason"`
								Failure string `json:"failure_kind"`
							} `json:"evidence"`
						}
						if err := caseassembly.Read(filepath.Join(source, "input.json"), &document); err != nil {
							t.Fatal(err)
						}
						expectedLevel, expectedRule := 8, "QA9"
						expected, expectedObject := check.Plan.Preimage["expected"].(plan.Object)
						if check.Plan.Preimage["expected"] != nil && !expectedObject {
							t.Fatal("expected value context is not an object")
						}
						if expected["computation"] != nil || expected["precondition"] != nil {
							expectedLevel, expectedRule = 6, "QO6"
						}
						foundEvidence := false
						for _, e := range document.Evidence {
							if e.ID == want.ID {
								foundEvidence = true
								invocationEnd := e.Reason == "could_not_start" || e.Reason == "network_unverifiable" || (e.Reason == "harness_fault" && recordedEngineOrBudget(t, recording, c.Input.Host, check.Plan.Keys)) || e.Failure == "over_budget" || e.Failure == "crashed" || e.Failure == "wrong_exit"
								changed := c.Input.Host["private_copy"] == "changed" && len(check.Plan.Keys) > 0
								if changed || expectedLevel != 6 && invocationEnd {
									level := 7
									if changed {
										level = 5
									}
									if check.Result.Unbuilt != nil || check.Result.Level != level || string(check.Result.Outcome) != e.Outcome || string(check.Result.Reason) != e.Reason || check.Result.FailureKind != e.Failure {
										t.Errorf("U44 check %s got %+v want level %d %s %s %s", want.ID, check.Result, level, e.Outcome, e.Reason, e.Failure)
									}
									expectedRule = ""
								}
							}
						}
						if !foundEvidence {
							t.Fatal("expected evidence absent", want.ID)
						}
						if expectedRule != "" && (check.Result.Unbuilt == nil || check.Result.Level != expectedLevel || check.Result.Unbuilt.Rule != expectedRule) {
							t.Errorf("U44 check %s got %+v want level %d %s", want.ID, check.Result, expectedLevel, expectedRule)
						}
						continue
					}
					if want.Ending.Phase == "deciding" {
						if check.Result.Unbuilt != nil || check.Result.Level != want.Level || check.Result.Outcome != "not_observed" || (want.Level == 1 && check.Result.Reason != "network_unverifiable") || (want.Level == 2 && check.Result.Reason != "placeholder_unfilled") {
							t.Errorf("check %s: %+v; want level %d", want.ID, check.Result, want.Level)
						}
					} else if check.Result.Unbuilt == nil || !allowedRule(want.Ending, check.Result.Unbuilt.Rule) {
						t.Errorf("check %s: %+v; want %+v", want.ID, check.Result, want.Ending)
					}
				}
			}
			counts["decide"]++
		})
	}
	if !reflect.DeepEqual(counts, map[string]int{"decide": 111, "planning": 7, "derivation": 31}) {
		t.Fatalf("phase counts %v", counts)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func recordedEngineOrBudget(t *testing.T, data []byte, host map[string]any, keys []plan.Object) bool {
	t.Helper()
	var recorded struct {
		Invocations []struct {
			Key      any `json:"key"`
			Confined []struct {
				End struct {
					By string `json:"by"`
				} `json:"end"`
			} `json:"confined"`
		} `json:"invocations"`
	}
	if err := json.Unmarshal(data, &recorded); err != nil {
		t.Fatal(err)
	}
	var overrides []struct {
		Key any    `json:"invocation"`
		By  string `json:"by"`
	}
	if err := json.Unmarshal(mustJSON(t, host["kill_attribution"]), &overrides); err != nil {
		t.Fatal(err)
	}
	for _, key := range keys {
		for _, i := range recorded.Invocations {
			if bytes.Equal(mustJSON(t, i.Key), mustJSON(t, key)) && len(i.Confined) > 0 && i.Confined[0].End.By == "budget" {
				return true
			}
		}
		for _, o := range overrides {
			if bytes.Equal(mustJSON(t, o.Key), mustJSON(t, key)) && o.By == "engine" {
				return true
			}
		}
	}
	return false
}

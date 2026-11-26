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
	// QD5, QO2, QO4, QE19, QT5: compare pending case evidence and served keys.
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
	if err := caseassembly.Read("../../scripts/ratchet/pr9-prediction.json", &predictions); err != nil {
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
			// Judge-checked endings follow the PR9 prediction.
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
			var trace bytes.Buffer
			raw, err := scripted.Read(host, recording, &trace)
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
			if err := raw.Required(plan.Array{}); err != nil {
				t.Fatal(err)
			}
			checks, _, err := gatherSignals(collect.Collect, p, raw, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			var pres map[string]any
			if err := caseassembly.Read(filepath.Join(source, "preimages.json"), &pres); err != nil {
				t.Fatal(err)
			}
			predByID := map[string]struct {
				Level  int
				Ending predictedEnding
			}{}
			for _, pred := range prediction.Checks {
				predByID[pred.ID] = struct {
					Level  int
					Ending predictedEnding
				}{pred.Level, pred.Ending}
			}
			var subjectJSON map[string]any
			if err := json.Unmarshal(data, &subjectJSON); err != nil {
				t.Fatal(err)
			}
			compareTrace(t, trace.Bytes(), recording, c.Input.Host, subjectJSON, pres, predByID)
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
						if check.Result.Unbuilt != nil || check.Result.Level != want.Level {
							t.Errorf("check %s: %+v; want level %d", want.ID, check.Result, want.Level)
						}
						compareDecided(t, source, check)

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

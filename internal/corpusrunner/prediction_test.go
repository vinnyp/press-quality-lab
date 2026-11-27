//go:build scripted

package corpusrunner

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

type ending struct {
	Exit  int      `json:"exit_code"`
	Phase string   `json:"phase"`
	Rules []string `json:"rules"`
}
type prediction struct {
	Case   string `json:"case"`
	Ending ending `json:"ending"`
}

func readPredictions(t *testing.T) map[string]prediction {
	t.Helper()
	var document struct {
		Cases []prediction `json:"cases"`
	}
	read(t, "../../scripts/ratchet/pr13-prediction.json", &document)
	out := map[string]prediction{}
	for _, p := range document.Cases {
		out[p.Case] = p
	}
	if len(out) != 152 {
		t.Fatalf("prediction count %d", len(out))
	}
	return out
}
func checkEnding(want ending, exit int, rule string) error {
	for _, r := range want.Rules {
		base, _, _ := strings.Cut(r, ".")
		if exit == want.Exit && base == rule {
			return nil
		}
	}
	return fmt.Errorf("ending exit=%d rule=%s; want exit=%d rules=%v", exit, rule, want.Exit, want.Rules)
}

func checkRequired(tr trace, input map[string]any) error {
	want, ok := input["required"].([]any)
	if !ok || tr.Required == nil || !reflect.DeepEqual(unordered(tr.Required), unordered(want)) {
		return errors.New("traced required set differs")
	}
	return nil
}

func TestPredictedEndingControls(t *testing.T) {
	// QO1, QT6: a pending ending requires both the predicted rule and engine-fault exit.
	want := ending{Exit: 70, Rules: []string{
		"QE2.placeholders",
		"QS5",
	}}
	for _, row := range []struct {
		name    string
		exit    int
		rule    string
		message string
	}{
		{"Exact", 70, "QS5", ""},
		{"Suffix", 70, "QE2", ""},
		{"WrongRule", 70, "QE16", "ending exit=70 rule=QE16; want exit=70 rules=[QE2.placeholders QS5]"},
		{"Refusal", 2, "QS5", "ending exit=2 rule=QS5; want exit=70 rules=[QE2.placeholders QS5]"},
		{"Verdict", 0, "", "ending exit=0 rule=; want exit=70 rules=[QE2.placeholders QS5]"},
		{"MissingRule", 70, "", "ending exit=70 rule=; want exit=70 rules=[QE2.placeholders QS5]"},
	} {
		t.Run(row.name, func(t *testing.T) { assertError(t, checkEnding(want, row.exit, row.rule), row.message) })
	}
}

func TestRequiredTraceControls(t *testing.T) {
	// QE3, QT6: compare independent case requirements with the trace, including missing events.
	for _, id := range []string{"case-003", "case-004"} {
		t.Run(strings.ReplaceAll(id, "-", "_"), func(t *testing.T) {
			_, input, _, _ := fixture(t, id)
			required, ok := input["required"].([]any)
			if !ok || len(required) == 0 {
				t.Fatal("fixture has no required checks")
			}
			assertError(t, checkRequired(trace{Required: required}, input), "")
			for _, required := range [][]any{nil, {}, required[1:], append(append([]any{}, required...), required[0])} {
				assertError(t, checkRequired(trace{Required: required}, input), "traced required set differs")
			}
			assertError(t, checkRequired(trace{Required: required}, map[string]any{}), "traced required set differs")
		})
	}
}

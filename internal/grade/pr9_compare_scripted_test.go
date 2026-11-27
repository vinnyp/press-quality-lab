//go:build scripted

package grade

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/caseassembly"
	"github.com/vinnyp/press-quality-lab/internal/collect"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/record"
)

func compareDecided(t *testing.T, source string, check collect.Check) {
	t.Helper()
	// QE19: compare every specified evidence field, including all target fields.
	var expected struct {
		Evidence []map[string]any `json:"evidence"`
	}
	if err := caseassembly.Read(filepath.Join(source, "input.json"), &expected); err != nil {
		t.Fatal(err)
	}
	resolution, err := checkResolution(check, nil)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := record.Build(record.Input{Plan: plan.Plan{Checks: []plan.Check{check.Plan}}, Results: map[string]record.Resolution{check.Plan.ID: resolution}, Scripted: true})
	if err != nil {
		t.Fatal(err)
	}
	evidence, ok := doc["evidence"].(plan.Array)
	if !ok || len(evidence) != 1 {
		t.Fatal("missing actual evidence")
	}
	actual, ok := evidence[0].(plan.Object)
	if !ok {
		t.Fatal("invalid actual evidence")
	}
	for _, want := range expected.Evidence {
		if want["check_id"] != check.Plan.ID {
			continue
		}
		for _, field := range []string{"outcome", "reason", "failure_kind", "target_size", "precondition_observed", "tier", "targets"} {
			got := normalizedJSON(t, actual[field])
			w := normalizedJSON(t, want[field])
			if !reflect.DeepEqual(got, w) {
				t.Errorf("check %s %s got %v want %v", check.Plan.ID, field, got, w)
			}
		}
		return
	}
	t.Fatal("missing expected evidence", check.Plan.ID)
}

func normalizedJSON(t *testing.T, v any) any {
	t.Helper()
	var out any
	if err := json.Unmarshal(mustJSON(t, v), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

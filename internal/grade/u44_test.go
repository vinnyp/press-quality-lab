package grade

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/collect"
	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
)

func TestInvocationFailureField(t *testing.T) {
	// QO4, QE8: an invocation failure is stored in failure_kind; reason remains null.
	o, d, saved := fixture(t)
	id := ""
	d.Collect = func(p plan.Plan, _ observe.Source) ([]collect.Check, error) {
		checks := make([]collect.Check, 0, len(p.Checks))
		for _, ch := range p.Checks {
			result := decide.Result{Outcome: "not_observed", Reason: "network_unverifiable", Level: 1}
			if ch.Preimage["probe"] == "bare" {
				id = ch.ID
				result = decide.InvocationOutcome(ch, observe.Controls{KnownFastWithin: true}, observe.Run{Markers: observe.Markers{Confinement: true}, End: observe.End{Signal: 9, EngineKilled: true, BudgetExpired: true, RunningAtBudget: true}})
			}
			checks = append(checks, collect.Check{Plan: ch, Result: result})
		}
		return checks, nil
	}
	_, err := Run(o, d, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Evidence []map[string]any `json:"evidence"`
	}
	data, marshalErr := json.Marshal(saved.JudgeInput)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if id == "" || len(doc.Evidence) == 0 {
		t.Fatal("no subject evidence")
	}
	found := false
	for _, e := range doc.Evidence {
		if e["check_id"] == id {
			found = true
			kind, present := e["failure_kind"]
			reason, reasonPresent := e["reason"]
			if !present || kind != "over_budget" || !reasonPresent || reason != nil || e["outcome"] != "not_held" || e["target_size"] != nil {
				t.Fatal(e)
			}
		}
	}
	if !found {
		t.Fatal("bare evidence absent")
	}
}

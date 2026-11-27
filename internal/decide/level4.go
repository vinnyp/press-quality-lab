package decide

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

// Stopped judges only pre-override resolutions. Membership maps every known id
// to whether it is required; false explicitly denotes an outside check.
func Stopped(earlier []string, resolutions map[string]Result, membership map[string]bool) bool {
	stopped := false
	for _, id := range earlier {
		required, known := membership[id]
		if !known {
			panic("QA7: unknown earlier check")
		}
		if !required {
			continue
		}
		r, ok := resolutions[id]
		if !ok || r.Outcome == "" && r.Unbuilt == nil {
			panic("QA7: missing earlier resolution")
		}
		stopped = stopped || r.Outcome != "held"
	}
	return stopped
}

// StepFacts contains facts used to decide E-49's steps without required checks.
type StepFacts struct {
	Stopped  bool
	Controls observe.Controls
	Check    *plan.Check
}

func StepRuns(f StepFacts) bool {
	if f.Check == nil {
		panic("E-49: missing step context")
	}
	// E-54: this is a second brake behind level 2.
	if placeholderRunbook(*f.Check, f.Check.Preimage["runbook"], f.Check.Scopes) {
		return false
	}
	if BeforeInvocationsWithEvidence(*f.Check, f.Controls, Evidence{}).Level != 0 {
		return false
	}
	return !f.Stopped
}

func level4(ch plan.Check, _ observe.Controls, e Evidence) Result {
	earlier, ok := ch.Preimage["earlier"].(strictjson.Array)
	if !ok {
		panic("QE2: preimage lacks earlier array")
	}
	ids := make([]string, len(earlier))
	for i, id := range earlier {
		name, ok := id.(string)
		if !ok {
			panic("QE2: invalid earlier id")
		}
		ids[i] = name
	}
	if Stopped(ids, e.Earlier, e.Membership) {
		return Result{Outcome: NotObserved, Reason: "earlier_step_failed", Level: 4}
	}
	return Result{}
}

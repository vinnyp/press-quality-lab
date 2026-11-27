package record

import (
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/plan"
)

func TestReasonPairing(t *testing.T) {
	// QE13, QO3, X3: only collector reasons accompany not_observed, and no others.
	for _, row := range []struct {
		name    string
		outcome decide.Outcome
		reason  decide.Reason
		valid   bool
	}{
		{"Held", "held", "", true},
		{"UnknownOutcome", "unknown", "", false},
		{"NotHeld", "not_held", "", true},
		{"Network", decide.NotObserved, "network_unverifiable", true},
		{"Attempted", decide.NotObserved, "network_attempted", false},
		{"NoEvidence", decide.NotObserved, "no_evidence", false},
		{"Scope", decide.NotObserved, "scope_not_requested", false},
		{"Empty", decide.NotObserved, "", false},
		{"HeldReason", "held", "network_unverifiable", false},
		{"NotHeldReason", "not_held", "network_unverifiable", false},
	} {
		t.Run(row.name, func(t *testing.T) {
			in := Input{Plan: plan.Plan{Checks: []plan.Check{{ID: "check"}}}, Results: map[string]Resolution{"check": {Result: decide.Result{Outcome: row.outcome, Reason: row.reason}}}}
			doc, err := Build(in)
			if (err == nil) != row.valid || !row.valid && doc != nil {
				t.Fatal(doc, err)
			}
		})
	}
	for _, reason := range decide.Reasons() {
		_, err := Build(Input{Plan: plan.Plan{Checks: []plan.Check{{ID: "check"}}}, Results: map[string]Resolution{"check": {Result: decide.Result{Outcome: decide.NotObserved, Reason: reason}}}})
		if err != nil {
			t.Fatal(reason, err)
		}
	}
}

func TestNextActionsSealed(t *testing.T) {
	// QE12, QE13: independent readings pin every action and both reason sets.
	read := func(path string) string {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	contract := read("../../docs/specs/conformance-collection-1.0/collection-contract.md")
	table := strings.Split(strings.Split(contract, "| Reason | Next action |")[1], "- **QE13.")[0]
	want := map[string]string{}
	for _, line := range strings.Split(table, "\n") {
		columns := strings.Split(strings.TrimSpace(line), "|")
		if len(columns) == 4 && strings.HasPrefix(strings.TrimSpace(columns[1]), "`") {
			want[strings.Trim(strings.TrimSpace(columns[1]), "`")] = strings.TrimSpace(columns[2])
		}
	}
	gotActions := NextActions()
	if len(want) != 11 || !reflect.DeepEqual(gotActions, want) {
		t.Fatalf("table got %v want %v", gotActions, want)
	}
	delete(want, "no_evidence")
	delete(want, "scope_not_requested")
	got := map[string]bool{}
	for _, reason := range decide.Reasons() {
		if got[string(reason)] {
			t.Fatal("duplicate", reason)
		}
		got[string(reason)] = true
	}
	fromContract := map[string]bool{}
	for reason := range want {
		fromContract[reason] = true
	}
	if !reflect.DeepEqual(got, fromContract) {
		t.Fatal(got, fromContract)
	}
	rulebook := read("../../docs/specs/judge-1.0/rulebook.md")
	list := strings.Split(strings.Split(rulebook, "**Reasons a family may report with `not_observed`:**")[1], "\n")[0]
	fromJudge := map[string]bool{}
	for _, token := range regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(list, -1) {
		fromJudge[token[1]] = true
	}
	delete(fromJudge, "network_attempted")
	delete(fromJudge, "live_not_run")
	delete(fromJudge, "judgement_unverified")
	if !reflect.DeepEqual(got, fromJudge) {
		t.Fatal(got, fromJudge)
	}
}

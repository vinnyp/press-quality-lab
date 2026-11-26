package decide

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/rulebook"
	"github.com/vinnyp/press-quality-lab/internal/subject"
)

func corpusPlan(t *testing.T, name string) plan.Plan {
	t.Helper()
	root := "../../docs/specs/conformance-collection-1.0"
	b, err := os.ReadFile(filepath.Join(root, "cases", name, "subject.json"))
	if err != nil {
		t.Fatal(err)
	}
	s, parseErr := subject.Parse(b)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	in := plan.Input{Subject: s, Scopes: []rulebook.Scope{rulebook.ConformanceScope}, Files: map[string][]byte{}}
	// The corpus subject names computation files; their contents affect ids only.
	err = filepath.WalkDir(filepath.Join(root, "subjects", "known-good", "computations"), func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			b, e := os.ReadFile(p)
			if e != nil {
				return e
			}
			in.Files["computations/"+d.Name()] = b
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := plan.Build(in)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func passing() observe.Controls {
	return observe.Controls{ConfinementApplied: true, SelfTestPassed: true, SystemNamed: true}
}
func first(t *testing.T, p plan.Plan, match func(plan.Check) bool) plan.Check {
	t.Helper()
	for _, ch := range p.Checks {
		if match(ch) {
			return ch
		}
	}
	t.Fatal("no matching planned check")
	return plan.Check{}
}
func TestTriggers(t *testing.T) {
	// QD4: generated checks only.
	// QA7: an earlier runbook step.
	// QH8: any private copy can change.
	// QO6: expected computation, precondition or capture.
	// QO2: invocation-level rules only for a check with invocations.
	// QA9: own rule is always reached when earlier levels pass.
	p := corpusPlan(t, "case-004")
	generated := first(t, p, func(c plan.Check) bool { return c.Preimage["probe"] != nil })
	authored := first(t, p, func(c plan.Check) bool { return c.Preimage["probe"] == nil })
	earlier := first(t, p, func(c plan.Check) bool {
		earlier, ok := c.Preimage["earlier"].(plan.Array)
		return ok && len(earlier) > 0
	})
	computation := first(t, p, func(c plan.Check) bool { return object(c.Preimage["expected"])["computation"] != nil })
	emptyPlan := corpusPlan(t, "case-144")
	empty := first(t, emptyPlan, func(c plan.Check) bool {
		return c.Preimage["words"] == nil && c.Preimage["probe"] != "discovery" && len(c.Keys) == 0
	})
	for _, row := range []struct {
		name string
		n    int
		yes  plan.Check
		no   *plan.Check
		rule string
	}{
		{"Generated", 3, generated, &authored, "QD4"},
		{"Earlier", 4, earlier, &generated, "QA7"},
		{"PrivateCopy", 5, authored, nil, "QH8"},
		{"Computation", 6, computation, &generated, "QO6"},
		{"Precondition", 6, plan.Check{Preimage: plan.Object{"expected": plan.Object{"precondition": plan.Object{"argv": plan.Array{"/bin/echo"}}}}}, &generated, "QO6"},
		{"Captures", 6, plan.Check{Preimage: plan.Object{"expected": plan.Object{"literal": plan.Object{"captures": plan.Array{"step"}}}}}, &generated, "QO6"},
		{"Invocation", 7, authored, &empty, "QO2"},
		{"OwnRule", 8, authored, nil, "QA9"},
	} {
		t.Run(row.name, func(t *testing.T) {
			r := levels[row.n-1](row.yes, passing(), Evidence{})
			if r.Unbuilt == nil || *r.Unbuilt != (Unbuilt{Level: row.n, Rule: row.rule}) || r.Level != row.n || r.Outcome != "" || r.Reason != "" {
				t.Fatalf("positive: %+v", r)
			}
			if row.no != nil {
				if r := levels[row.n-1](*row.no, passing(), Evidence{}); r != (Result{}) {
					t.Fatalf("negative: %+v", r)
				}
			}
		})
	}
}
func TestPassingHost(t *testing.T) {
	// QO1: a passing host must leave the check to the first applicable sentinel.
	p := corpusPlan(t, "case-004")
	for _, ch := range p.Checks {
		want, rule := 5, "QH8"
		if ch.Preimage["probe"] != nil {
			want, rule = 3, "QD4"
		} else if earlier, ok := ch.Preimage["earlier"].(plan.Array); ok && len(earlier) > 0 {
			want, rule = 4, "QA7"
		}
		r := Resolve(ch, passing(), Evidence{})
		if r.Level != want || r.Unbuilt == nil || r.Unbuilt.Rule != rule {
			t.Fatalf("check %s: %+v want %d %s", ch.ID, r, want, rule)
		}
	}
}
func TestGradeWide(t *testing.T) {
	// QO1: all three grade-wide conditions, including E-15's no-run check.
	// QH1: system fact is injected, never detected here.
	// QH9: failed self-test decides all 29 case-004 checks.
	p := corpusPlan(t, "case-004")
	if len(p.Checks) != 29 {
		t.Fatalf("checks %d", len(p.Checks))
	}
	p.Checks = append(p.Checks, plan.Check{Preimage: plan.Object{"probe": "byte_bound", "words": nil}, Required: plan.Object{}})
	for _, name := range []string{"Confinement", "SelfTest", "System"} {
		t.Run(name, func(t *testing.T) {
			c := passing()
			switch name {
			case "Confinement":
				c.ConfinementApplied = false
			case "SelfTest":
				c.SelfTestPassed = false
			case "System":
				c.SystemNamed = false
			}
			for _, ch := range p.Checks {
				r := Resolve(ch, c, Evidence{})
				if r != (Result{Outcome: "not_observed", Reason: "network_unverifiable", Level: 1}) {
					t.Fatalf("%s: %+v", ch.ID, r)
				}
			}
		})
	}
}
func TestInvocationConfinement(t *testing.T) {
	// QO1: QH6's failed confinement applies only to a subject run that happened.
	ch := corpusPlan(t, "case-004").Checks[0]
	for _, row := range []struct {
		name        string
		run         observe.Run
		computation any
		want        bool
	}{
		{"ExitWithoutMarker", observe.Run{}, nil, true},
		{"KilledBeforeMarker", observe.Run{End: observe.End{Signal: 9}}, nil, true},
		{"MarkedExit", observe.Run{Markers: observe.Markers{Confinement: true}}, nil, false},
		{"MarkedKill", observe.Run{Markers: observe.Markers{Confinement: true}, End: observe.End{Signal: 9, EngineKilled: true}}, nil, false},
		{"OtherSignal", observe.Run{End: observe.End{Signal: 11, EngineKilled: true}}, nil, false},
		{"Computation", observe.Run{}, plan.Array{"/bin/echo"}, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			e := Evidence{Invocations: []observe.Invocation{{Key: observe.Key{"computation": row.computation}, Run: row.run}}}
			r := level1(ch, passing(), e)
			if row.want {
				if r != (Result{Outcome: "not_observed", Reason: "network_unverifiable", Level: 1}) {
					t.Fatalf("%+v", r)
				}
			} else if r != (Result{}) {
				t.Fatalf("%+v", r)
			}
		})
	}
	if r := level1(ch, passing(), Evidence{}); r != (Result{}) {
		t.Fatalf("absent run %+v", r)
	}
}

func TestOwnRuleNames(t *testing.T) {
	// QD2: discovery's own rules.
	// QP2: bare invocation.
	// QP3: byte bound.
	// QP4: declared exits.
	// QP6: store integrity.
	// QO7: planted-secret combination.
	// QA9: authored predicates.
	want := map[any]string{
		nil:               "QA9",
		"discovery":       "QD2",
		"bare":            "QP2",
		"byte_bound":      "QP3",
		"exit_codes":      "QP4",
		"store_unchanged": "QP6",
		"planted_secret":  "QO7",
	}
	seen := map[any]bool{}
	for _, ch := range corpusPlan(t, "case-004").Checks {
		probe := ch.Preimage["probe"]
		seen[probe] = true
		r := level8(ch, passing(), Evidence{})
		if r.Unbuilt == nil || r.Unbuilt.Rule != want[probe] || r.Level != 8 {
			t.Fatalf("probe %v: %+v", probe, r)
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("covered %d kinds want %d", len(seen), len(want))
	}
}

func TestInvocationTriggers(t *testing.T) {
	// QO2: each subject invocation shape reaches the invocation rules.
	p := corpusPlan(t, "case-004")
	for _, ch := range p.Checks {
		// All checks in this subject have a subject invocation, including discovery
		// and the multi-command planted-secret probe, whose words are null.
		r := level7(ch, passing(), Evidence{})
		if r.Unbuilt == nil || r.Unbuilt.Rule != "QO2" || r.Level != 7 {
			t.Fatalf("probe %v: %+v", ch.Preimage["probe"], r)
		}
	}
}
func TestUnknownTriggers(t *testing.T) {
	// QO6: absent expected-value information cannot exclude a computation.
	// QO2: absent invocation information cannot exclude an invocation.
	for _, n := range []int{6, 7} {
		if r := levels[n-1](plan.Check{}, passing(), Evidence{}); r.Unbuilt == nil || r.Level != n {
			t.Fatalf("level %d: %+v", n, r)
		}
	}
}

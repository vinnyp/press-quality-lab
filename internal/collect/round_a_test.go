package collect

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
)

func TestRoundAHomeRemoval(t *testing.T) {
	// QE20, QA7: single homes disappear after observation; shared homes survive intermediate steps.
	for _, kind := range []string{"seed", "runbook"} {
		t.Run(kind, func(t *testing.T) {
			var home string
			f := &homeFake{run: func(_ observe.Key, spec observe.Spec) (observe.Run, error) {
				home = spec.HomePath
				return cleanLoopRun(), nil
			}}
			s, ch := homeFixture(t, f)
			key := homeKey(kind)
			key["runbook"] = "book"
			if _, err := s.Observe(key, observe.Spec{CheckID: ch.ID}); err != nil {
				t.Fatal(err)
			}
			_, err := os.Stat(home)
			if kind == "runbook" {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("home retained: %v", err)
			}
		})
	}
}

func TestRoundAAttribution(t *testing.T) {
	// QO7: an inconsistent end belongs to the decision path, not the engine fault path.
	for _, kind := range []string{"seed", "planted"} {
		t.Run(kind, func(t *testing.T) {
			f := &homeFake{run: func(_ observe.Key, spec observe.Spec) (observe.Run, error) {
				if spec.Control {
					t.Fatal("inconsistent end triggered read control")
				}
				r := cleanLoopRun()
				r.End.RunningAtBudget = true
				r.Details.Planted = &observe.Planted{AtimeMoved: true, SameFile: true}
				return r, nil
			}}
			source, ch := homeFixture(t, f)
			source.controls = goodControls()
			source.controls.ReadCalibrationMoved = true
			run, err := source.Observe(homeKey(kind), observe.Spec{CheckID: ch.ID})
			if err != nil {
				t.Fatalf("attribution escaped: %v", err)
			}
			ch.Preimage["probe"] = "planted_secret"
			ch.Preimage["parameters"] = plan.Object{"kind": "credential"}
			ch.Preimage["expected"] = nil
			ch.Preimage["earlier"] = plan.Array{}
			result := decide.Resolve(ch, goodControls(), decide.Evidence{Discovery: &decide.Discovery{}, Invocations: []observe.Invocation{{Key: homeKey(kind), Run: run}}})
			if result.Outcome != "not_observed" || result.Reason != "harness_fault" {
				t.Fatalf("decision %+v", result)
			}
		})
	}
}

func TestRoundASeedChanged(t *testing.T) {
	// QE2, QP5: copied bytes must still match the planned seed.
	f := &homeFake{run: func(_ observe.Key, _ observe.Spec) (observe.Run, error) { return cleanLoopRun(), nil }}
	s, ch := homeFixture(t, f)
	ch.Seed = []plan.Entry{{Path: "store", Data: []byte("data")}, {Path: "token", Data: []byte("secret-token")}}
	s.checks[ch.ID] = ch
	if _, err := s.Observe(homeKey("seed"), observe.Spec{CheckID: ch.ID}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.directory, "seed", "store"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := s.Observe(homeKey("seed"), observe.Spec{CheckID: ch.ID})
	var fault *plan.Fault
	if !errors.As(err, &fault) || fault.Rule != "QE2" || fault.Cause != "seed copy differs from plan" {
		t.Fatalf("seed fault: %v", err)
	}
}

func TestRoundAHomeWorkspaceAndHook(t *testing.T) {
	// QE20, QT5: every home is below the published run workspace, and the typed hook sees it before cleanup.
	f := &homeFake{run: func(_ observe.Key, _ observe.Spec) (observe.Run, error) { return cleanLoopRun(), nil }}
	s, ch := homeFixture(t, f)
	parent := t.TempDir()
	s.options.RunDirectory = parent
	called := 0
	var home string
	s.options.AfterObserve = func(_ observe.Key, spec observe.Spec, run observe.Run) error {
		called++
		home = spec.HomePath
		rel, err := filepath.Rel(parent, home)
		if err != nil || !filepath.IsLocal(rel) {
			t.Fatalf("home outside run directory: %s %v", rel, err)
		}
		if _, err := os.Stat(home); err != nil {
			t.Fatal(err)
		}
		if string(run.Stdout) != "value" {
			t.Fatal("hook lost run")
		}
		return nil
	}
	for _, kind := range []string{"seed", "empty", "planted", "scratch"} {
		if _, err := s.Observe(homeKey(kind), observe.Spec{CheckID: ch.ID}); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(home); !os.IsNotExist(err) {
			t.Fatal("hook home retained", err)
		}
	}
	if called != 4 {
		t.Fatal(called)
	}
}

func TestRoundAUnknownHome(t *testing.T) {
	// QT5: an unsupported home is an engine fault before the source is reached.
	f := &homeFake{run: func(_ observe.Key, _ observe.Spec) (observe.Run, error) {
		t.Fatal("source reached")
		return observe.Run{}, nil
	}}
	s, ch := homeFixture(t, f)
	for _, kind := range []string{"", "unknown"} {
		_, err := s.Observe(homeKey(kind), observe.Spec{CheckID: ch.ID})
		var fault *plan.Fault
		if !errors.As(err, &fault) || fault.Rule != "QT5" || fault.Cause != "unknown home kind" {
			t.Fatalf("%v", err)
		}
	}
}

func TestRoundABookCleanup(t *testing.T) {
	// QA7, QE20: a shared home survives intermediate steps and is removed after the last step.
	p := loopPlan(t, plan.Array{loopStep("first", loopCheck()), loopStep("last", loopCheck())})
	subject := p.Checks[0].Subject
	subject["runbooks"] = append(discoveryArray(subject["runbooks"]), plan.Object{"name": "next", "steps": plan.Array{loopStep("next", loopCheck())}})
	var buildErr error
	p, buildErr = plan.Build(plan.Input{Subject: subject, Scopes: p.Scopes})
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	n := 0
	for n < len(p.Checks) && p.Checks[n].Preimage["probe"] != nil {
		n++
	}
	p.Checks = p.Checks[n:]
	for _, book := range p.Runbooks {
		for _, step := range book.Steps {
			for i := range step.Required {
				step.Required[i] -= n
			}
		}
	}
	var home string
	calls := 0
	nextCalls := 0
	f := &homeFake{controls: goodControls(), run: func(key observe.Key, spec observe.Spec) (observe.Run, error) {
		if key["runbook"] == nil {
			return cleanLoopRun(), nil
		}
		if key["runbook"] == "next" {
			nextCalls++
			if _, err := os.Stat(home); !os.IsNotExist(err) {
				t.Fatal("earlier book home retained", err)
			}
			return cleanLoopRun(), nil
		}
		calls++
		if home != "" && home != spec.HomePath {
			t.Fatal("shared home replaced")
		}
		home = spec.HomePath
		if _, err := os.Stat(home); err != nil {
			t.Fatal(err)
		}
		return cleanLoopRun(), nil
	}}
	if _, err := Collect(p, f, Options{RunDirectory: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || nextCalls != 1 {
		t.Fatal(calls, nextCalls)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatal("last step home retained", err)
	}
}

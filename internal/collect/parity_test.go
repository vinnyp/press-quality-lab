//go:build necessity_test

package collect

import (
	"errors"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/rulebook"
	"github.com/vinnyp/press-quality-lab/internal/throwaway"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestHomeFilesBeforeSnapshot(t *testing.T) {
	// KTD20, QT5: parity files precede the snapshot; ordinary collection leaves the option unset.
	for _, enabled := range []bool{false, true} {
		f := &homeFake{run: func(k observe.Key, s observe.Spec) (observe.Run, error) {
			found := false
			for _, e := range s.Before.Entries {
				found = found || e.Path == "peer.pid"
			}
			if found != enabled {
				t.Fatalf("fixture presence %v", found)
			}
			return cleanLoopRun(), nil
		}}
		s, ch := homeFixture(t, f)
		if enabled {
			s.options.HomeFiles = []throwaway.NamedFile{{Name: "peer.pid", Data: []byte("123")}}
		}
		if _, err := s.Observe(homeKey("seed"), observe.Spec{CheckID: ch.ID}); err != nil {
			t.Fatal(err)
		}
	}
}
func TestReplayRunbook(t *testing.T) {
	// QA4, QA7, QS6: a replay includes earlier captures and continues after failed predicates.
	comp := loopComp("echo")
	comp["captures"] = plan.Array{"first"}
	last := loopCheck()
	last["expected"] = plan.Object{"computation": comp}
	p := loopPlan(t, plan.Array{loopStep("first", loopCheck()), loopStep("last", last)})
	var homes []string
	calls := 0
	f := &homeFake{controls: goodControls(), run: func(k observe.Key, s observe.Spec) (observe.Run, error) {
		calls++
		if k["computation"] == nil {
			homes = append(homes, s.HomePath)
		} else {
			b, err := os.ReadFile(s.CapturePaths["first"])
			if err != nil || string(b) != "wrong" {
				t.Fatalf("capture %q %v", b, err)
			}
			if s.HomeKind != "scratch" {
				t.Fatal("computation home")
			}
		}
		r := cleanLoopRun()
		r.Stdout = []byte("wrong")
		r.StdoutSeen = 5
		return r, nil
	}}
	ch := p.Checks[len(p.Checks)-1]
	key := ch.Keys[len(ch.Keys)-1]
	runs, err := Serve(p, f, ch, key, Options{RunDirectory: t.TempDir()})
	if err != nil || len(runs) != 3 || calls != 3 || len(homes) != 2 || homes[0] != homes[1] {
		t.Fatalf("replay %d %d %v %v", len(runs), calls, homes, err)
	}
	ch.Subject["commands"] = append(discoveryArray(ch.Subject["commands"]), plan.Object{"words": plan.Array{"first"}, "class": "destructive", "writes": true})
	p.Scopes = []rulebook.Scope{rulebook.ConformanceScope, rulebook.DestructiveScope}
	if _, err := Serve(p, f, ch, key, Options{RunDirectory: t.TempDir()}); err != nil {
		t.Fatal("scoped twin", err)
	}
	p.Scopes = []rulebook.Scope{rulebook.ConformanceScope}
	_, err = Serve(p, f, ch, key, Options{RunDirectory: t.TempDir()})
	var fault *plan.Fault
	if !errors.As(err, &fault) || fault.Rule != "QS6" || fault.Cause != "destructive invocation outside allowed runbook scope" {
		t.Fatalf("scope guard %v", err)
	}
}

func TestReplayRecordedSettings(t *testing.T) {
	// QT5, QA7: recorded offline arguments and environment names survive targeted replay.
	p := loopPlan(t, plan.Array{loopStep("first", loopCheck())})
	ch := p.Checks[0]
	key := plan.Object{}
	for name, value := range ch.Keys[0] {
		key[name] = value
	}
	key["offline_args"] = plan.Array{"--recorded"}
	key["env"] = plan.Array{"RECORDED_MODE"}
	calls := 0
	f := &homeFake{controls: goodControls(), run: func(k observe.Key, s observe.Spec) (observe.Run, error) {
		calls++
		if !reflect.DeepEqual(k["offline_args"], plan.Array{"--recorded"}) || !reflect.DeepEqual(k["env"], plan.Array{"RECORDED_MODE"}) {
			t.Fatalf("settings %v", k)
		}
		return cleanLoopRun(), nil
	}}
	runs, err := Serve(p, f, ch, key, Options{RunDirectory: t.TempDir()})
	if err != nil || len(runs) != 1 || calls != 1 {
		t.Fatalf("replay %v %d %v", runs, calls, err)
	}
}

func TestReplayHomeKind(t *testing.T) {
	// QT5: replay accepts the planned home kind and faults on empty or unknown kinds.
	ch := discoveryCheck()
	p := plan.Plan{Checks: []plan.Check{ch}}
	for _, kind := range []string{homeText(ch.Keys[0]["home"]), "", "unknown"} {
		key := plan.Object{}
		for name, value := range ch.Keys[0] {
			key[name] = value
		}
		key["home"] = kind
		calls := 0
		f := &homeFake{controls: goodControls(), run: func(_ observe.Key, _ observe.Spec) (observe.Run, error) {
			calls++
			return cleanLoopRun(), nil
		}}
		runs, err := Serve(p, f, ch, key, Options{RunDirectory: t.TempDir()})
		if kind == "" || kind == "unknown" {
			var fault *plan.Fault
			if !errors.As(err, &fault) || fault.Rule != "QT5" || fault.Cause != "unknown home kind" || calls != 0 {
				t.Fatalf("invalid home %q: %d %v", kind, calls, err)
			}
		} else if err != nil || calls != 1 || len(runs) != 1 {
			t.Fatalf("planned home %q: %d %d %v", kind, calls, len(runs), err)
		}
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

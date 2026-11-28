package collect

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/rulebook"
	"github.com/vinnyp/press-quality-lab/internal/throwaway"
)

type homeFake struct {
	run      func(observe.Key, observe.Spec) (observe.Run, error)
	controls observe.Controls
}

func (f *homeFake) Controls() (observe.Controls, error)                        { return f.controls, nil }
func (f *homeFake) Observe(k observe.Key, s observe.Spec) (observe.Run, error) { return f.run(k, s) }
func homeFixture(t *testing.T, f *homeFake) (*homeSource, plan.Check) {
	t.Helper()
	dir := t.TempDir()
	seed := filepath.Join(dir, "seed")
	if err := os.Mkdir(seed, 0700); err != nil {
		t.Fatal(err)
	}
	if err := throwaway.WriteHomeFiles(seed, []throwaway.NamedFile{{Name: "token", Data: []byte("secret-token")}, {Name: "store", Data: []byte("data")}}); err != nil {
		t.Fatal(err)
	}
	ch := plan.Check{Seed: []plan.Entry{{Path: "store", Data: []byte("data")}, {Path: "token", Data: []byte("secret-token")}}, ID: "home", Subject: plan.Object{"seed": "seed", "store": "store", "secret": plan.Object{"path": "token", "token": "secret-token"}, "commands": plan.Array{plan.Object{"words": plan.Array{}, "class": "safe", "writes": false, "exit_codes": nil}}}, Preimage: plan.Object{"probe": "bare"}}
	s := newHomeSource(plan.Plan{SubjectDirectory: dir, Checks: []plan.Check{ch}}, f, Options{})
	s.controls = f.controls
	t.Cleanup(func() {
		if err := s.close(); err != nil {
			t.Error(err)
		}
	})
	return s, ch
}
func homeKey(kind string) observe.Key {
	return observe.Key{"home": kind, "words": plan.Array{}, "runbook": nil, "step": nil, "computation": nil}
}

func TestHomeKindsAndFreshness(t *testing.T) {
	// QP5, QH14, QA4, QA7, QH12: probes get fresh homes; only runbook steps share.
	for _, kind := range []string{"seed", "empty", "planted", "runbook", "scratch"} {
		t.Run(kind, func(t *testing.T) {
			paths := []string{}
			f := &homeFake{run: func(k observe.Key, s observe.Spec) (observe.Run, error) {
				if s.HomeKind != kind {
					t.Fatalf("kind %q", s.HomeKind)
				}
				if _, err := os.Stat(filepath.Join(s.HomePath, ".pql-tmp")); err != nil {
					t.Fatal(err)
				}
				_, err := os.Stat(filepath.Join(s.HomePath, "store"))
				seeded := kind == "seed" || kind == "planted" || kind == "runbook"
				if seeded && err != nil || !seeded && !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("seed contents %v", err)
				}
				if len(paths) > 0 {
					_, err := os.Stat(filepath.Join(s.HomePath, "written"))
					if kind == "runbook" && err != nil || kind != "runbook" && !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("fresh state %v", err)
					}
					found := false
					for _, e := range s.Before.Entries {
						found = found || e.Path == "written"
					}
					if found != (kind == "runbook") {
						t.Fatal("before snapshot stale")
					}
				} else if err := throwaway.WriteHomeFiles(s.HomePath, []throwaway.NamedFile{{Name: "written", Data: []byte("x")}}); err != nil {
					t.Fatal(err)
				}
				paths = append(paths, s.HomePath)
				return cleanLoopRun(), nil
			}}
			s, ch := homeFixture(t, f)
			k := homeKey(kind)
			if kind == "runbook" {
				k["runbook"] = "book"
			}
			for range 2 {
				if _, err := s.Observe(k, observe.Spec{CheckID: ch.ID}); err != nil {
					t.Fatal(err)
				}
			}
			if (paths[0] == paths[1]) != (kind == "runbook") {
				t.Fatal("home reuse")
			}
		})
	}
}
func TestReadControlSequence(t *testing.T) {
	// QP10, QH14, QH27, QO7, E-38: control condition, fresh canaries and same-canary control.
	for _, name := range []string{"Read", "NoCalibration", "NoMove", "Swapped", "BadExit", "Unconfined", "CouldNotStart"} {
		t.Run(name, func(t *testing.T) {
			canaries := []string{}
			ordinaryHome := ""
			controls := 0
			f := &homeFake{controls: goodControls()}
			f.controls.ReadCalibrationMoved = name != "NoCalibration"
			f.run = func(k observe.Key, s observe.Spec) (observe.Run, error) {
				if s.PlantedBefore.Access.After(s.PlantedBefore.Modified.Add(-24 * time.Hour)) {
					t.Fatal("not backdated")
				}
				digest := ""
				for _, e := range s.Before.Entries {
					if e.Path == "token" {
						digest = e.Digest
					}
				}
				if digest == "" {
					t.Fatal("missing planted snapshot digest")
				}
				if s.Control {
					controls++
					if s.HomePath == ordinaryHome {
						t.Fatal("control reused ordinary home")
					}
					if s.Canary != canaries[len(canaries)-1] {
						t.Fatal("different control canary")
					}
					return observe.Run{Markers: observe.Markers{Confinement: true}, End: observe.End{Signal: 9}}, nil
				}
				if len(canaries) > 0 && s.Canary == canaries[len(canaries)-1] {
					t.Fatal("reused canary")
				}
				canaries = append(canaries, s.Canary)
				ordinaryHome = s.HomePath
				r := cleanLoopRun()
				r.Details.Planted = &observe.Planted{AtimeMoved: name != "NoMove", SameFile: name != "Swapped"}
				if name == "BadExit" {
					r.End.Code = 1
				}
				if name == "Unconfined" {
					r.Markers.Confinement = false
				}
				if name == "CouldNotStart" {
					r.Markers.CouldNotStart = true
				}
				return r, nil
			}
			s, ch := homeFixture(t, f)
			for range 2 {
				r, err := s.Observe(homeKey("planted"), observe.Spec{CheckID: ch.ID})
				if err != nil {
					t.Fatal(err)
				}
				if (r.Details.Planted.ControlEnd != nil) != (name == "Read") {
					t.Fatal("control fold")
				}
			}
			want := 0
			if name == "Read" {
				want = 2
			}
			if controls != want {
				t.Fatalf("controls %d want %d", controls, want)
			}
		})
	}
}
func TestStoreControlPassed(t *testing.T) {
	// QP9: the appended separate copy supplies StoreControl; the subject copy stays pristine.
	for _, name := range []string{"Detected", "NoStore", "EmptyStore"} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			f := &homeFake{run: func(k observe.Key, s observe.Spec) (observe.Run, error) {
				calls++
				want := name == "Detected"
				if s.StoreControl == nil || *s.StoreControl != want {
					t.Fatalf("store control %v want %v", s.StoreControl, want)
				}
				b, err := os.ReadFile(filepath.Join(s.HomePath, "store"))
				if err != nil {
					t.Fatal(err)
				}
				expected := "data"
				if name == "EmptyStore" {
					expected = ""
				}
				if string(b) != expected {
					t.Fatal("control changed served home")
				}
				return cleanLoopRun(), nil
			}}
			s, ch := homeFixture(t, f)
			ch.Preimage["probe"] = "store_unchanged"
			if name == "NoStore" {
				ch.Subject["store"] = nil
			}
			if name == "EmptyStore" {
				if err := os.WriteFile(filepath.Join(s.directory, "seed", "store"), nil, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(s.directory, "seed", "token"), nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if name == "EmptyStore" {
				ch.Seed = []plan.Entry{{Path: "store"}, {Path: "token"}}
			}
			s.checks[ch.ID] = ch
			for range 2 {
				if _, err := s.Observe(homeKey("seed"), observe.Spec{CheckID: ch.ID}); err != nil {
					t.Fatal(err)
				}
			}
			if calls != 2 {
				t.Fatal(calls)
			}
		})
	}
}
func TestServeTimeAssertion(t *testing.T) {
	// QH27: metadata read at serve catches an intervening access before any source call.
	f := &homeFake{}
	s, ch := homeFixture(t, f)
	spec, err := s.prepare(ch, homeKey("planted"), observe.Spec{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := assertBackdated(spec); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := os.Chtimes(spec.PlantedPath, now, now); err != nil {
		t.Fatal(err)
	}
	err = assertBackdated(spec)
	var fault *plan.Fault
	if !errors.As(err, &fault) || fault.Rule != "QH27" || fault.Cause != "planted access time moved before serve" {
		t.Fatalf("fault %v", err)
	}
}

func TestDefaultHomeOptions(t *testing.T) {
	// QT5, KTD20: Collect leaves parity-only home files unset.
	calls := 0
	f := &homeFake{controls: goodControls(), run: func(k observe.Key, s observe.Spec) (observe.Run, error) {
		calls++
		for _, e := range s.Before.Entries {
			if e.Path == "peer.pid" {
				t.Fatal("parity file in default collection")
			}
		}
		return cleanLoopRun(), nil
	}}
	s, ch := homeFixture(t, f)
	ch.Subject["runbooks"] = plan.Array{}
	ch.Keys = []plan.Object{homeKey("seed")}
	ch.Preimage["parameters"] = plan.Object{"exit_codes": nil}
	ch.Preimage["earlier"] = plan.Array{}
	ch.Preimage["words"] = plan.Array{}
	ch.Preimage["expected"] = nil
	p := plan.Plan{SubjectDirectory: s.directory, Checks: []plan.Check{discoveryCheck(), ch}, Scopes: []rulebook.Scope{rulebook.ConformanceScope}}
	if _, err := Collect(p, f); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal(calls)
	}
}

func TestPlantedPreparationOrder(t *testing.T) {
	// QP10, QH27: the six steps precede serving, with no snapshot after backdating.
	events := []string{}
	f := &homeFake{run: func(k observe.Key, s observe.Spec) (observe.Run, error) {
		events = append(events, "serve")
		if !reflect.DeepEqual(events, []string{"copy", "plant", "snapshot", "backdate", "facts", "serve"}) {
			t.Fatalf("order %v", events)
		}
		return cleanLoopRun(), nil
	}}
	s, ch := homeFixture(t, f)
	s.preparation.plant = func(home, name, token, canary string) (string, error) {
		b, err := os.ReadFile(filepath.Join(home, name))
		if err != nil || string(b) != "secret-token" {
			t.Fatalf("fresh seed %q %v", b, err)
		}
		events = append(events, "copy", "plant")
		return throwaway.PlantGivenCanary(home, name, token, canary)
	}
	backdated := false
	s.preparation.snapshot = func(home, name, digest string) (throwaway.HomeSnapshot, error) {
		if backdated {
			t.Fatal("snapshot after backdating")
		}
		events = append(events, "snapshot")
		return throwaway.SnapshotPlanted(home, name, digest)
	}
	s.preparation.backdate = func(home, name string) (throwaway.FileFacts, error) {
		backdated = true
		events = append(events, "backdate")
		facts, err := throwaway.BackdatePlanted(home, name)
		events = append(events, "facts")
		return facts, err
	}
	if _, err := s.Observe(homeKey("planted"), observe.Spec{CheckID: ch.ID}); err != nil {
		t.Fatal(err)
	}
}

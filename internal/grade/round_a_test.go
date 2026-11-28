package grade

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/collect"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/record"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"github.com/vinnyp/press-quality-lab/internal/throwaway"
)

type roundASource struct {
	text   string
	canary string
	t      *testing.T
	masker *record.Masker
}

func (s *roundASource) Controls() (observe.Controls, error) {
	return observe.Controls{ConfinementApplied: true, SelfTestPassed: true, SystemNamed: true, PrivateCopyUnchanged: true, KnownFastWithin: true}, nil
}
func (s *roundASource) Observe(_ observe.Key, spec observe.Spec) (observe.Run, error) {
	text := "Usage: subject\n"
	if spec.Canary != "" {
		s.canary = spec.Canary
		if got := s.masker.Mask(spec.Canary); got != "pqlcanary"+strings.Repeat("0", 32) {
			s.t.Fatalf("canary not registered before observation: %q", got)
		}
		text = s.text
		if text == "canary" {
			text = spec.Canary
		}
	}
	return observe.Run{Markers: observe.Markers{Confinement: true}, Stdout: []byte(text), StdoutSeen: int64(len(text))}, nil
}

func TestRoundACanaryMasking(t *testing.T) {
	// QE18: a freshly drawn canary is registered before any raw observation or durable output.
	for _, name := range []string{"Canary", "Clean"} {
		t.Run(name, func(t *testing.T) {
			o, d, saved := fixture(t)
			s := inputSubject(t)
			s["seed"] = "seed"
			s["secret"] = strictjson.Object{"path": "token", "kind": "credential", "reason": nil, "token": "secret-token", "commands": strictjson.Array{strictjson.Array{}}}
			writeInput(t, filepath.Join(filepath.Dir(o.SubjectFile), "seed", "token"), "secret-token")
			if err := os.WriteFile(o.SubjectFile, strictjson.Write(s), 0600); err != nil {
				t.Fatal(err)
			}
			d.Masker = record.NewMasker(nil, nil)
			source := &roundASource{text: "clean_value", t: t, masker: d.Masker}
			if name == "Canary" {
				source.text = "canary"
			}
			d.Source = func() (observe.Source, func() error, error) { return source, func() error { return nil }, nil }
			o.JSON = true
			var out bytes.Buffer
			code, err := Run(o, d, &out)
			if err != nil || code == 70 {
				t.Fatalf("%d %v", code, err)
			}
			if source.canary == "" {
				t.Fatal("no planted observation")
			}
			doc := string(strictjson.Write(saved.Document()))
			if !strings.Contains(doc, "stdout") || !strings.Contains(doc, "head") {
				t.Fatal("missing inspected stream fields")
			}
			if strings.Contains(doc, source.canary) || strings.Contains(out.String(), source.canary) {
				t.Fatal("canary leaked")
			}
			expected := "clean_value"
			if name == "Canary" {
				expected = "pqlcanary" + strings.Repeat("0", 32)
			}
			if !strings.Contains(doc, expected) {
				t.Fatalf("missing stream value %s", expected)
			}
			// JSON output carries the judge document; stream masking is checked in the record.
			if !strings.Contains(out.String(), "outcome") {
				t.Fatal("missing judge document")
			}
		})
	}
}

func TestRoundAReleasedBeforePreparation(t *testing.T) {
	// KTD21, QE20: forcing discovery to observe a released source refuses before preparing homes.
	parent := t.TempDir()
	ch := plan.Check{ID: "release", Preimage: plan.Object{"probe": "discovery"}, Subject: plan.Object{}}
	_, err := collect.Collect(plan.Plan{Checks: []plan.Check{ch}}, &releasedReady{parent: parent, t: t}, collect.Options{RunDirectory: parent})
	var refused *Error
	if !errors.As(err, &refused) || refused.Rule != "QO1" || refused.Cause != "released source must never observe" {
		t.Fatalf("release fault %v", err)
	}
	entries, readErr := os.ReadDir(parent)
	if readErr != nil || len(entries) != 0 {
		t.Fatal(entries, readErr)
	}
}

type releasedReady struct {
	ReleasedSource
	parent string
	t      *testing.T
}

func (s *releasedReady) Observe(key observe.Key, spec observe.Spec) (observe.Run, error) {
	entries, err := os.ReadDir(s.parent)
	if err != nil || len(entries) != 0 {
		s.t.Fatalf("released source prepared a home: %v %v", entries, err)
	}
	return s.ReleasedSource.Observe(key, spec)
}

func (releasedReady) Controls() (observe.Controls, error) {
	return observe.Controls{ConfinementApplied: true, SelfTestPassed: true, SystemNamed: true, PrivateCopyUnchanged: true}, nil
}

func TestRoundAGradeHomeWorkspace(t *testing.T) {
	// QE20: the default collector places homes beneath the grade workspace published for cleanup.
	o, d, _ := fixture(t)
	var runDirectory string
	original := d.Workspace
	d.Workspace = func() (*throwaway.Workspace, error) {
		w, err := original()
		if err == nil {
			runDirectory = w.Path()
		}
		return w, err
	}
	s := &workspaceSource{t: t, parent: &runDirectory}
	d.Source = func() (observe.Source, func() error, error) { return s, func() error { return nil }, nil }
	if _, err := Run(o, d, io.Discard); err != nil {
		t.Fatal(err)
	}
	if s.calls == 0 {
		t.Fatal("no observations")
	}
}

type workspaceSource struct {
	t      *testing.T
	parent *string
	calls  int
}

func (*workspaceSource) Controls() (observe.Controls, error) {
	return observe.Controls{ConfinementApplied: true, SelfTestPassed: true, SystemNamed: true, PrivateCopyUnchanged: true, KnownFastWithin: true}, nil
}
func (s *workspaceSource) Observe(_ observe.Key, spec observe.Spec) (observe.Run, error) {
	s.calls++
	rel, err := filepath.Rel(*s.parent, spec.HomePath)
	if err != nil || !filepath.IsLocal(rel) {
		s.t.Fatalf("home outside workspace: %s %v", rel, err)
	}
	return observe.Run{Markers: observe.Markers{Confinement: true}, Stdout: []byte("Usage: subject\n"), StdoutSeen: 15}, nil
}

func TestRoundAGradeSeedChanged(t *testing.T) {
	// QE2, QP5: changing a seed after planning faults the grade instead of grading different bytes.
	for _, changed := range []bool{false, true} {
		o, d, _ := fixture(t)
		s := inputSubject(t)
		s["seed"] = "seed"
		seedFile := filepath.Join(filepath.Dir(o.SubjectFile), "seed", "value")
		writeInput(t, seedFile, "original")
		if err := os.WriteFile(o.SubjectFile, strictjson.Write(s), 0600); err != nil {
			t.Fatal(err)
		}
		// This source supplies ordinary clean observations without a workspace assertion.
		d.Source = func() (observe.Source, func() error, error) {
			if changed {
				if err := os.WriteFile(seedFile, []byte("changed"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			return seedSource{}, func() error { return nil }, nil
		}
		code, err := Run(o, d, io.Discard)
		if changed {
			assertFailure(t, code, err, 70, "QE2", "engine source fault")
		} else if err != nil || code == 70 {
			t.Fatalf("unchanged seed %d %v", code, err)
		}
	}
}

type seedSource struct{}

func (seedSource) Controls() (observe.Controls, error) {
	return observe.Controls{ConfinementApplied: true, SelfTestPassed: true, SystemNamed: true, PrivateCopyUnchanged: true, KnownFastWithin: true}, nil
}
func (seedSource) Observe(observe.Key, observe.Spec) (observe.Run, error) {
	return observe.Run{Markers: observe.Markers{Confinement: true}, Stdout: []byte("Usage: subject\n"), StdoutSeen: 15}, nil
}

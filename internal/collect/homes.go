package collect

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"time"

	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/throwaway"
)

type preparation struct {
	random   io.Reader
	compare  func(throwaway.HomeSnapshot, throwaway.HomeSnapshot) ([]throwaway.HomeDelta, bool)
	plant    func(string, string, string, string) (string, error)
	snapshot func(string, string, string) (throwaway.HomeSnapshot, error)
	backdate func(string, string) (throwaway.FileFacts, error)
}

type homeSource struct {
	preparation preparation
	source      observe.Source
	controls    observe.Controls
	checks      map[string]plan.Check
	directory   string
	options     Options
	workspace   *throwaway.Workspace
	books       map[string]string
	homes       map[string]*throwaway.Workspace
	captures    map[string][]observe.Invocation
}

func newHomeSource(p plan.Plan, source observe.Source, options Options) *homeSource {
	s := &homeSource{preparation: preparation{random: rand.Reader, compare: throwaway.CompareHomes, plant: throwaway.PlantGivenCanary, snapshot: throwaway.SnapshotPlanted, backdate: throwaway.BackdatePlanted}, source: source, directory: p.SubjectDirectory, options: options, checks: map[string]plan.Check{}, books: map[string]string{}, homes: map[string]*throwaway.Workspace{}, captures: map[string][]observe.Invocation{}}
	for _, ch := range p.Checks {
		s.checks[ch.ID] = ch
	}
	return s
}
func (s *homeSource) Controls() (observe.Controls, error) {
	c, err := s.source.Controls()
	s.controls = c
	return c, err
}
func (s *homeSource) close() error {
	if s.workspace == nil {
		return nil
	}
	return s.workspace.Remove()
}
func (s *homeSource) fresh(ch plan.Check, kind string, fixtures bool) (string, error) {
	if s.workspace == nil {
		w, err := throwaway.Create(s.options.RunDirectory)
		if err != nil {
			return "", err
		}
		s.workspace = w
	}
	w, err := throwaway.Create(s.workspace.Path())
	if err != nil {
		return "", err
	}
	home := filepath.Join(w.Path(), "home")
	s.homes[home] = w
	seed := homeText(ch.Subject["seed"])
	if kind != "empty" && kind != "scratch" && seed != "" {
		if s.directory == "" {
			return "", &plan.Fault{Rule: "QP5", Cause: "seed directory context missing"}
		}
		if err := throwaway.CopySeed(filepath.Join(s.directory, seed), home); err != nil {
			return "", err
		}
		copied, err := throwaway.SnapshotHome(home)
		if err != nil {
			return "", err
		}
		var entries []plan.Entry
		for _, e := range copied.Entries {
			entries = append(entries, plan.Entry{Path: e.Path, Directory: e.Kind == "directory", Digest: e.Digest})
		}
		actual, err := plan.SeedDigest(entries)
		if err != nil {
			return "", err
		}
		expected, err := plan.SeedDigest(ch.Seed)
		if err != nil {
			return "", err
		}
		if copied.Unreadable || copied.Incomplete || actual != expected {
			return "", &plan.Fault{Rule: "QE2", Cause: "seed copy differs from plan"}
		}
	}
	if err := throwaway.EmptyHome(home); err != nil {
		return "", err
	}
	if fixtures {
		if err := applyHomeFixtures(home, s.options); err != nil {
			return "", err
		}
	}
	return home, nil
}
func (s *homeSource) prepare(ch plan.Check, key observe.Key, spec observe.Spec, canary string) (observe.Spec, error) {
	kind := homeText(key["home"])
	switch kind {
	case "seed", "empty", "planted", "runbook", "scratch":
	default:
		return spec, &plan.Fault{Rule: "QT5", Cause: "unknown home kind"}
	}
	home := ""
	book := homeText(key["runbook"])
	if kind == "runbook" {
		home = s.books[book]
	}
	if home == "" {
		var err error
		home, err = s.fresh(ch, kind, true)
		if err != nil {
			return spec, err
		}
		if kind == "runbook" {
			s.books[book] = home
		}
	}
	spec.HomePath, spec.HomeKind = home, kind
	if kind == "scratch" {
		spec.CapturePaths = map[string]string{}
		for _, comp := range decide.Computations(ch) {
			if !reflect.DeepEqual(comp["argv"], key["computation"]) {
				continue
			}
			for _, v := range discoveryArray(comp["captures"]) {
				name := homeText(v)
				for _, inv := range s.captures[book] {
					if inv.Key["step"] == name {
						file := "capture-" + fmt.Sprint(len(spec.CapturePaths))
						if err := throwaway.WriteHomeFiles(home, []throwaway.NamedFile{{Name: file, Data: inv.Run.Stdout}}); err != nil {
							return spec, err
						}
						spec.CapturePaths[name] = filepath.Join(home, file)
					}
				}
			}
		}
	}
	if kind == "planted" {
		if canary == "" {
			var err error
			canary, err = throwaway.DrawCanary(s.preparation.random)
			if err != nil {
				return spec, err
			}
			if s.options.CanaryDrawn != nil {
				s.options.CanaryDrawn(canary)
			}
		}
		secret := discoveryObject(ch.Subject["secret"])
		name := homeText(secret["path"])
		token := homeText(secret["token"])
		digest, err := s.preparation.plant(home, name, token, canary)
		if err != nil {
			return spec, err
		}
		spec.Before, err = s.preparation.snapshot(home, name, digest)
		if err != nil {
			return spec, err
		}
		spec.PlantedBefore, err = s.preparation.backdate(home, name)
		if err != nil {
			return spec, err
		}
		spec.Canary, spec.PlantedPath = canary, filepath.Join(home, name)
	} else {
		var err error
		spec.Before, err = throwaway.SnapshotHome(home)
		if err != nil {
			return spec, err
		}
	}
	return spec, nil
}
func (s *homeSource) storeControl(ch plan.Check) (result *bool, err error) {
	home, err := s.fresh(ch, "seed", false)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.removeHome(home)) }()
	before, err := throwaway.SnapshotHome(home)
	if err != nil {
		return nil, err
	}
	store := homeText(ch.Subject["store"])
	var bookkeeping []string
	for _, v := range discoveryArray(ch.Subject["bookkeeping"]) {
		if name, ok := v.(string); ok {
			bookkeeping = append(bookkeeping, name)
		}
	}
	detected := false
	if !throwaway.StoreTarget(before, store, bookkeeping) {
		return &detected, nil
	}
	if err := throwaway.AppendStoreControl(home, store); err != nil {
		return nil, err
	}
	after, err := throwaway.SnapshotHome(home)
	if err != nil {
		return nil, err
	}
	changes, unreadable := s.preparation.compare(before, after)
	if unreadable {
		return nil, &plan.Fault{Rule: "QP9", Cause: "control copy unreadable"}
	}
	for _, change := range changes {
		detected = detected || change.Path == store
	}
	return &detected, nil
}
func assertBackdated(spec observe.Spec) error {
	if spec.PlantedPath == "" {
		return nil
	}
	facts, err := throwaway.InspectFile(spec.PlantedPath)
	if err != nil {
		return err
	}
	if facts.Access.After(facts.Modified.Add(-24 * time.Hour)) {
		return &plan.Fault{Rule: "QH27", Cause: "planted access time moved before serve"}
	}
	return nil
}
func (s *homeSource) Observe(key observe.Key, spec observe.Spec) (result observe.Run, err error) {
	if refusal, ok := s.source.(interface{ BeforeHome() error }); ok {
		if err := refusal.BeforeHome(); err != nil {
			return observe.Run{}, err
		}
	}
	ch, ok := s.checks[spec.CheckID]
	if !ok {
		return observe.Run{}, &plan.Fault{Rule: "KTD13", Cause: "missing home context"}
	}
	prepared, err := s.prepare(ch, key, spec, "")
	if err != nil {
		return observe.Run{}, err
	}
	if prepared.HomeKind != "runbook" {
		defer func() { err = errors.Join(err, s.removeHome(prepared.HomePath)) }()
	}
	if ch.Preimage["probe"] == "store_unchanged" {
		prepared.StoreControl, err = s.storeControl(ch)
		if err != nil {
			return observe.Run{}, err
		}
	}
	if err := assertBackdated(prepared); err != nil {
		return observe.Run{}, err
	}
	run, err := s.source.Observe(key, prepared)
	if err != nil {
		return observe.Run{}, err
	}
	p := run.Details.Planted
	if s.options.AfterObserve != nil {
		if err := s.options.AfterObserve(key, prepared, run); err != nil {
			return observe.Run{}, err
		}
	}
	if prepared.HomeKind == "planted" && p != nil && p.AtimeMoved && p.SameFile && s.controls.ReadCalibrationMoved && acceptedRead(ch, key, run) {
		controlSpec, err := s.prepare(ch, key, spec, prepared.Canary)
		if err != nil {
			return observe.Run{}, err
		}
		defer func() { err = errors.Join(err, s.removeHome(controlSpec.HomePath)) }()
		controlSpec.Control = true
		if err := assertBackdated(controlSpec); err != nil {
			return observe.Run{}, err
		}
		control, err := s.source.Observe(key, controlSpec)
		if err != nil {
			return observe.Run{}, err
		}
		if s.options.AfterObserve != nil {
			if err := s.options.AfterObserve(key, controlSpec, control); err != nil {
				return observe.Run{}, err
			}
		}
		planted := *p
		planted.ControlEnd = &observe.InvocationProtocol{End: control.End, Markers: control.Markers}
		run.Details.Planted = &planted
	}
	if key["home"] == "runbook" {
		book := homeText(key["runbook"])
		s.captures[book] = append(s.captures[book], observe.Invocation{Key: key, Run: run})
	}
	return run, nil
}

// collectWithOptions owns home cleanup on every collection exit.
func collectWithOptions(p plan.Plan, source observe.Source, options Options) (checks []Check, err error) {
	prepared := newHomeSource(p, source, options)
	defer func() { err = errors.Join(err, prepared.close()) }()
	return collectPrepared(p, prepared, decide.Resolve)
}

func homeText(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}

func acceptedRead(ch plan.Check, key observe.Key, run observe.Run) bool {
	attribution, err := observe.Attribute(run)
	return err == nil && attribution == observe.Exited && decide.AcceptedInvocationCode(ch, key, run.End.Code)
}
func (s *homeSource) removeHome(home string) error {
	w := s.homes[home]
	if w == nil {
		return nil
	}
	if err := w.Remove(); err != nil {
		return err
	}
	delete(s.homes, home)
	return nil
}
func (s *homeSource) finishBook(book string) error {
	home := s.books[book]
	if err := s.removeHome(home); err != nil {
		return err
	}
	delete(s.books, book)
	return nil
}

package collect

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"time"

	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/throwaway"
)

// Options supplies parity-only home fixtures. Production callers leave it zero.
type Options struct{ HomeFiles []throwaway.NamedFile }

type preparation struct {
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
	captures    map[string][]observe.Invocation
}

func newHomeSource(p plan.Plan, source observe.Source, options Options) *homeSource {
	s := &homeSource{preparation: preparation{plant: throwaway.PlantGivenCanary, snapshot: throwaway.SnapshotPlanted, backdate: throwaway.BackdatePlanted}, source: source, directory: p.SubjectDirectory, options: options, checks: map[string]plan.Check{}, books: map[string]string{}, captures: map[string][]observe.Invocation{}}
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
		w, err := throwaway.Create("")
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
	seed := homeText(ch.Subject["seed"])
	if kind != "empty" && kind != "scratch" && seed != "" {
		if s.directory == "" {
			return "", &plan.Fault{Rule: "QP5", Cause: "seed directory context missing"}
		}
		if err := throwaway.CopySeed(filepath.Join(s.directory, seed), home); err != nil {
			return "", err
		}
	}
	if err := throwaway.EmptyHome(home); err != nil {
		return "", err
	}
	if fixtures {
		if err := throwaway.WriteHomeFiles(home, s.options.HomeFiles); err != nil {
			return "", err
		}
	}
	return home, nil
}
func (s *homeSource) prepare(ch plan.Check, key observe.Key, spec observe.Spec, canary string) (observe.Spec, error) {
	kind := homeText(key["home"])
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
			var raw [16]byte
			if _, err := rand.Read(raw[:]); err != nil {
				return spec, err
			}
			canary = "pqlcanary" + hex.EncodeToString(raw[:])
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
func (s *homeSource) storeControl(ch plan.Check) (*bool, error) {
	home, err := s.fresh(ch, "seed", false)
	if err != nil {
		return nil, err
	}
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
	changes, unreadable := throwaway.CompareHomes(before, after)
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
func (s *homeSource) Observe(key observe.Key, spec observe.Spec) (observe.Run, error) {
	ch, ok := s.checks[spec.CheckID]
	if !ok {
		return observe.Run{}, &plan.Fault{Rule: "KTD13", Cause: "missing home context"}
	}
	prepared, err := s.prepare(ch, key, spec, "")
	if err != nil {
		return observe.Run{}, err
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
	attribution, err := observe.Attribute(run)
	if err != nil {
		return observe.Run{}, err
	}
	if prepared.HomeKind == "planted" && s.controls.ReadCalibrationMoved && p != nil && p.AtimeMoved && p.SameFile && attribution == observe.Exited && decide.AcceptedInvocationCode(ch, key, run.End.Code) {
		controlSpec, err := s.prepare(ch, key, spec, prepared.Canary)
		if err != nil {
			return observe.Run{}, err
		}
		controlSpec.Control = true
		if err := assertBackdated(controlSpec); err != nil {
			return observe.Run{}, err
		}
		control, err := s.source.Observe(key, controlSpec)
		if err != nil {
			return observe.Run{}, err
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

// CollectWithOptions supplies parity fixtures without changing released wiring.
func CollectWithOptions(p plan.Plan, source observe.Source, options Options) (checks []Check, err error) {
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

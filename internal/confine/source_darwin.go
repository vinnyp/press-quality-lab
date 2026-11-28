package confine

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/throwaway"
	"golang.org/x/sys/unix"
)

// Source collects raw Darwin facts. Only tests construct it until PR16.
// The resolver supplies planned argv, literal environment and computation paths;
// homes, controls and before-snapshots always come from collect's Spec.
type Source struct {
	copies, controlCopies *Copies
	workspace             *throwaway.Workspace
	subject, computation  Invocation
	resolve               func(observe.Key, observe.Spec) (Invocation, error)
	afterWait             func(observe.Spec) error
	controls              *observe.Controls
	product, build        string
	controlProbe          observe.Run
	controlFacts          ControlFacts
}

func (s *Source) Controls() (observe.Controls, error) {
	if s.controls != nil {
		return *s.controls, nil
	}
	unchanged := s.copies.Unchanged() && s.controlCopies.Unchanged()
	s.product, s.build = "", ""
	product, err := unix.Sysctl("kern.osproductversion")
	if err != nil {
		return observe.Controls{}, err
	}
	build, err := unix.Sysctl("kern.osversion")
	if err != nil {
		return observe.Controls{}, err
	}
	s.product, s.build = product, build
	c := observe.Controls{PrivateCopyUnchanged: unchanged, SystemNamed: observe.SystemNamed(product)}
	// QH6's marker-only run establishes whether sandbox-exec applied confinement.
	probe := s.subject
	probe.Fast = true
	probe.Workspace = s.workspace
	p, err := s.controlCopies.Start(probe)
	if err != nil {
		return c, err
	}
	raw, err := p.Wait()
	if err != nil {
		return c, err
	}
	s.controlProbe = raw
	probeControls := ControlsFromProbe(raw)
	c.ConfinementApplied = probeControls.ConfinementApplied
	c.SandboxExecExit = probeControls.SandboxExecExit
	c.PrivateCopyUnchanged = c.PrivateCopyUnchanged && probeControls.PrivateCopyUnchanged
	if raw.CopyChanged {
		return c, fmt.Errorf("control private copy changed before probe")
	}
	if c.ConfinementApplied {
		facts, err := RunControlSuite(s.workspace, s.controlCopies, s.subject, s.computation)
		if err != nil {
			return observe.Controls{}, err
		}
		s.controlFacts = facts
		c = facts.Apply(c)
	}
	s.controls = &c
	return c, nil
}

func (s *Source) Observe(key observe.Key, spec observe.Spec) (result observe.Run, err error) {
	inv, err := s.resolve(key, spec)
	if err != nil {
		return observe.Run{}, err
	}
	if inv.Paths.PlantedDirs != nil {
		return observe.Run{}, fmt.Errorf("resolver supplied planted directories")
	}
	inv.Paths.PlantedDirs, err = plantedDirectories(spec.HomePath, spec.PlantedPath)
	if err != nil {
		return observe.Run{}, err
	}
	inv.Paths.Values = clonePathValues(inv.Paths.Values)
	inv.Paths.Values["HOME"] = spec.HomePath
	planted := spec.PlantedPath
	if planted == "" {
		var reservation *throwaway.Workspace
		reservation, planted, err = absentPlanted(s.workspace.Path())
		if err != nil {
			return observe.Run{}, err
		}
		defer func() { err = errors.Join(err, reservation.Remove()) }()
	}
	inv.Paths.Values["PLANTED"] = planted
	inv.Directory, inv.Workspace, inv.Control = spec.HomePath, s.workspace, spec.Control
	inv.Budget = time.Duration(spec.BudgetSeconds) * time.Second
	if inv.Budget <= 0 {
		return observe.Run{}, fmt.Errorf("invocation budget required")
	}
	// Start checks the persistent private copy before rendering or spawning.
	p, err := s.copies.Start(inv)
	if err != nil {
		return observe.Run{}, err
	}
	run, err := p.Wait()
	if err != nil || run.CopyChanged {
		return run, err
	}
	if s.afterWait != nil {
		if err := s.afterWait(spec); err != nil {
			return observe.Run{}, err
		}
	}
	// Nothing above this point reads the planted contents after Wait.
	relative := ""
	if spec.PlantedPath != "" {
		after, err := throwaway.InspectFile(spec.PlantedPath)
		if err != nil {
			return observe.Run{}, err
		}
		facts := observe.PlantedFacts(spec.PlantedBefore, after)
		run.Details.Planted = &facts
		relative, err = filepath.Rel(spec.HomePath, spec.PlantedPath)
		if err != nil {
			return observe.Run{}, err
		}
	}
	after, err := throwaway.SnapshotHome(spec.HomePath)
	if err != nil {
		return observe.Run{}, err
	}
	changes, unreadable := throwaway.CompareHomes(spec.Before, after)
	run.Details.Home.Unreadable = unreadable
	run.Details.Home.ControlDetected = spec.StoreControl
	for _, change := range changes {
		var digest *string
		if change.Kind == "file" && change.Change != "removed" && change.Digest != "" {
			value := change.Digest
			digest = &value
		}
		run.Details.Home.Changes = append(run.Details.Home.Changes, observe.HomeChange{Path: change.Path, Kind: change.Kind, Change: change.Change, Digest: digest})
	}
	if spec.Canary != "" {
		var forms []throwaway.ScanForm
		for _, form := range observe.EncodeForms(spec.Canary) {
			forms = append(forms, throwaway.ScanForm{Name: form.Name, Text: form.Text})
		}
		scan, err := throwaway.ScanHome(spec.HomePath, relative, forms, run.Stdout, run.Stderr, run.StdoutCut || run.StderrCut || run.OutputCutAfterExit)
		if err != nil {
			return observe.Run{}, err
		}
		run.Details.LeakScan.Unreadable, run.Details.LeakScan.BoundReached = scan.Unreadable, scan.BoundReached
		for _, leak := range scan.Leaks {
			run.Details.LeakScan.Leaks = append(run.Details.LeakScan.Leaks, observe.Leak{Place: leak.Place, Encoding: leak.Encoding})
		}
	}
	run.Details.Stdout = sourceStream(run.Stdout, run.StdoutCut)
	run.Details.Stderr = sourceStream(run.Stderr, run.StderrCut)
	if inv.Computation {
		if value, ok := observe.ParseComputed(run.Stdout, run.StdoutCut || run.OutputCutAfterExit, run.StdoutSeen); ok {
			run.Details.Printed = &observe.PrintedValue{Value: value}
		}
	}
	return run, nil
}

package confine

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/throwaway"
)

// RunControlSuite uses one reusable pair of private pql images, prepared with
// NewCopies(workspace.Path(), launcher, launcher). Each witness replaces the
// program in the grade's subject or computation profile; real subject data is
// never used as an attempted target. The caller owns workspace removal.
func RunControlSuite(workspace *throwaway.Workspace, copies *Copies, subject, computation Invocation) (facts ControlFacts, err error) {
	if runtime.GOARCH != "arm64" {
		return facts, fmt.Errorf("confinement self-test requires darwin/arm64")
	}
	if workspace == nil || copies == nil {
		return facts, fmt.Errorf("control workspace and copies required")
	}
	if subject.Computation || !computation.Computation || subject.Control || computation.Control {
		return facts, fmt.Errorf("invalid self-test profiles")
	}
	launcher, program := copies.CopyPaths()
	var subjectFiles *throwaway.SelfTestFiles
	for i, base := range []Invocation{subject, computation} {
		files, e := workspace.PrepareSelfTest()
		if e != nil {
			return facts, e
		}
		if i == 0 {
			subjectFiles = files
		}
		inv := selfTestInvocation(base, files, workspace)
		runs, e := runProfileSelfTest(copies, inv, launcher, program, files)
		if e != nil {
			return facts, e
		}
		if i == 0 {
			facts.Subject = runs
		} else {
			facts.Computation = runs
		}
	}
	inv := selfTestInvocation(subject, subjectFiles, workspace)
	facts.Fast, err = RunKnownFast(copies, inv)
	if err != nil {
		return facts, err
	}
	facts.Before, facts.After, facts.CalibrationRead, err = RunCalibration(copies, inv, subjectFiles)
	if err != nil {
		facts.CalibrationRead = observe.Run{}
	}
	return facts, err
}

func selfTestInvocation(base Invocation, files *throwaway.SelfTestFiles, workspace GroupWorkspace) Invocation {
	base.Paths.Values = clonePathValues(base.Paths.Values)
	base.Paths.Values["HOME"], base.Paths.Values["PLANTED"] = files.Home, files.Calibration
	if base.Computation {
		base.Paths.Values["PLANTED"] = filepath.Join(files.Home, "absent_planted")
	}
	base.Paths.PlantedDirs = nil
	base.Workspace = workspace
	base.Directory = files.Home
	base.Env = nil
	base.Fast = false
	base.Budget = 10 * time.Second
	return base
}

func clonePathValues(values map[string]string) map[string]string {
	out := make(map[string]string, len(values))
	for k, v := range values {
		out[k] = v
	}
	return out
}

func runProfileSelfTest(c *Copies, inv Invocation, launcher, program string, files *throwaway.SelfTestFiles) (runs []observe.Run, err error) {
	// The decoy also reuses the private launcher. Never execute a changed
	// image unconfined, even though the confined witnesses would reject it.
	if !c.launcher.unchanged() || !c.program.unchanged() {
		return []observe.Run{{CopyChanged: true}}, nil
	}
	decoy, err := startSelfTestDecoy(launcher, files.Home, inv.Workspace)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, files.CheckDecoys(), decoy.close()) }()
	for _, p := range SelfTestPrograms(inv.Computation) {
		target := ""
		switch p.Name {
		case "tcp":
			target = strconv.Itoa(decoy.tcp)
		case "udp":
			target = strconv.Itoa(decoy.udp)
		case "unix":
			target = "socket"
		case "read", "write":
			target = files.Outside
		case "create":
			target = files.Create
		case "map":
			target = program
		case "service":
			target = "org.pql.selftest.decoy"
		case "child":
			target = "/usr/bin/true"
		case "clean":
			target = files.Clean
		}
		inv.Words = []string{SelfTestMode, p.Name, target, strconv.Itoa(decoy.pid)}
		r, e := runControl(c, inv)
		if e != nil {
			return runs, e
		}
		runs = append(runs, r)
	}
	return runs, nil
}

// RunKnownFast always runs the subject's marker-only launcher five times.
// Run lifetimes include launch and wait; no per-run image is created.
func RunKnownFast(c *Copies, inv Invocation) ([]ControlRun, error) {
	if inv.Computation || inv.Control {
		return nil, fmt.Errorf("known-fast requires subject profile")
	}
	inv.Fast = true
	inv.Words = nil
	inv.Budget = 10 * time.Second
	runs := make([]ControlRun, 0, 5)
	for range 5 {
		start := time.Now()
		r, err := runControl(c, inv)
		if err != nil {
			return runs, err
		}
		runs = append(runs, ControlRun{Run: r, Elapsed: time.Since(start)})
	}
	return runs, nil
}

// RunCalibration reads only the prepared decoy under the subject's profile.
// Preparation sets atime one day before mtime inside internal/throwaway; the
// final metadata read does not itself access the decoy's data.
func RunCalibration(c *Copies, inv Invocation, files *throwaway.SelfTestFiles) (before, after time.Time, read observe.Run, err error) {
	if inv.Computation || inv.Control || inv.Fast {
		return before, after, read, fmt.Errorf("calibration requires subject profile")
	}
	inv.Words = []string{SelfTestMode, "calibrate", files.Calibration, "1"}
	read, err = runControl(c, inv)
	if err != nil {
		return files.Before, after, read, err
	}
	after, err = files.CalibrationAfter()
	if err != nil {
		read = observe.Run{}
	}
	return files.Before, after, read, err
}

func runControl(c *Copies, inv Invocation) (observe.Run, error) {
	p, err := c.Start(inv)
	if err != nil {
		return observe.Run{}, err
	}
	return p.Wait()
}

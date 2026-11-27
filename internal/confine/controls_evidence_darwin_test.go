//go:build darwin && macos_evidence

package confine_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vinnyp/press-quality-lab/internal/confine"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/throwaway"
)

func controlFixture(t *testing.T, launcher string) (*throwaway.Workspace, *confine.Copies, confine.Invocation, confine.Invocation) {
	t.Helper()
	w, err := throwaway.BeginAt(filepath.Join(t.TempDir(), "runs"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := w.Remove(); err != nil {
			t.Error(err)
		}
	})
	c, err := confine.NewCopies(w.Path(), launcher, launcher)
	if err != nil {
		t.Fatal(err)
	}
	l, p := c.CopyPaths()
	home, err := filepath.EvalSymlinks(w.Path())
	if err != nil {
		t.Fatal(err)
	}
	subject := confine.Invocation{Workspace: w, Directory: home, Paths: evidencePaths(t, home, l, p), Budget: 10 * time.Second}
	computation := subject
	computation.Paths.Values = make(map[string]string)
	for k, v := range subject.Paths.Values {
		computation.Paths.Values[k] = v
	}
	computation.Computation = true
	computation.Paths.Values["BIN_DIR"] = "/bin"
	computation.Paths.Values["USR_BIN_DIR"] = "/usr/bin"
	computation.Paths.Values["TTY_DEVICE"] = "/dev/tty"
	// The subject exclusion names a separate existing decoy, not the private
	// pql image taking the computation's place.
	subjectPath := filepath.Join(t.TempDir(), "subject")
	if err := os.WriteFile(subjectPath, []byte("subject_decoy"), 0600); err != nil {
		t.Fatal(err)
	}
	computation.Paths.Values["SUBJECT"], computation.Paths.Values["SUBJECT_COPY"] = subjectPath, subjectPath
	return w, c, subject, computation
}

func controlSuite(t *testing.T, launcher string) confine.ControlFacts {
	t.Helper()
	w, c, s, computation := controlFixture(t, launcher)
	facts, err := confine.RunControlSuite(w, c, s, computation)
	if err != nil {
		t.Fatal(err)
	}
	return facts
}

func assertSelfTestPrograms(t *testing.T, facts confine.ControlFacts) {
	t.Helper()
	for profile, runs := range [][]observe.Run{facts.Subject, facts.Computation} {
		names := []string{"tcp", "unix", "udp", "bind", "listen", "read", "map", "write", "create", "service", "sysctl", "arguments", "environment", "process", "signal", "codesign", "priority", "resource"}
		if profile == 0 {
			names = append(names, "child")
		} else {
			names = append(names, "setsid", "setpgid")
		}
		wantCount := len(names) + 1
		if profile == 1 {
			wantCount++
		}
		if len(runs) != wantCount {
			t.Fatalf("profile %d: %d runs want %d", profile, len(runs), wantCount)
		}
		for i, name := range names {
			r := runs[i]
			if !r.Markers.Confinement || r.End.Signal != 9 || r.End.EngineKilled || r.End.BudgetExpired || string(r.Stdout) != "attempt_"+name+"\n" || len(r.Stderr) != 0 {
				t.Errorf("profile %d %s: end=%+v markers=%+v stdout=%q stderr=%q", profile, name, r.End, r.Markers, r.Stdout, r.Stderr)
			}
		}
		if profile == 1 {
			r := runs[len(names)]
			if r.End.Code != 0 || r.End.Signal != 0 || string(r.Stdout) != "attempt_child\nchild_ok\n" {
				t.Errorf("child end=%+v output=%q stderr=%q", r.End, r.Stdout, r.Stderr)
			}
		}
		r := runs[len(runs)-1]
		if r.End.Code != 0 || r.End.Signal != 0 || string(r.Stdout) != "clean_value\n" {
			t.Errorf("clean profile %d end=%+v output=%q stderr=%q", profile, r.End, r.Stdout, r.Stderr)
		}
	}
}

func TestMacOSSelfTestPrograms(t *testing.T) {
	// QH9: every named refusal is killed after its own attempt line, under
	// both profiles; the clean and allowed-child programs print known output.
	_, launcher := evidenceLauncher(t)
	facts := controlSuite(t, launcher)
	assertSelfTestPrograms(t, facts)
	if !facts.Apply(observe.Controls{}).SelfTestPassed {
		t.Fatal("self-test rejected the literal traces")
	}
}

func TestMacOSSelfTestWrongOutput(t *testing.T) {
	// QH9: an otherwise successful clean program printing wrong data fails.
	root, launcher := evidenceLauncher(t)
	positive := controlSuite(t, launcher)
	if !positive.Apply(observe.Controls{}).SelfTestPassed {
		t.Fatal("positive control failed")
	}
	source := filepath.Join(root, "internal/confine/selftest_darwin.go")
	b, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	old := []byte(`[]byte("clean_value\n")`)
	if bytes.Count(b, old) != 1 {
		t.Fatal("clean output mutation target changed")
	}
	dir := t.TempDir()
	replacement := filepath.Join(dir, "selftest_darwin.go")
	if err := os.WriteFile(replacement, bytes.Replace(b, old, []byte(`[]byte("wrong_value\n")`), 1), 0600); err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(dir, "overlay.json")
	data, err := json.Marshal(map[string]any{"Replace": map[string]string{source: replacement}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overlay, data, 0600); err != nil {
		t.Fatal(err)
	}
	mutant := filepath.Join(dir, "pql")
	evidenceBuild(t, root, mutant, "-overlay", overlay, "./cmd/pql")
	facts := controlSuite(t, mutant)
	for profile, runs := range [][]observe.Run{facts.Subject, facts.Computation} {
		r := runs[len(runs)-1]
		if r.End.Code != 0 || r.End.Signal != 0 || string(r.Stdout) != "wrong_value\n" {
			t.Fatalf("mutation did not evaluate clean output profile %d: %+v %q", profile, r.End, r.Stdout)
		}
	}
	if facts.Apply(observe.Controls{}).SelfTestPassed {
		t.Fatal("wrong clean output passed self-test")
	}
}

func TestMacOSKnownFastControl(t *testing.T) {
	// QH24: five runs of the engine launcher, each marker-only and <= 2 s.
	_, launcher := evidenceLauncher(t)
	_, c, s, _ := controlFixture(t, launcher)
	runs, err := confine.RunKnownFast(c, s)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 5 {
		t.Fatalf("runs %d want 5", len(runs))
	}
	for i, r := range runs {
		if !r.Run.Markers.Confinement || r.Run.End.Code != 0 || r.Run.End.Signal != 0 || len(r.Run.Stdout) != 0 || len(r.Run.Stderr) != 0 || r.Elapsed > 2*time.Second {
			t.Errorf("run %d: end=%+v marker=%+v elapsed=%s out=%q err=%q", i, r.Run.End, r.Run.Markers, r.Elapsed, r.Run.Stdout, r.Run.Stderr)
		}
	}
	if !confine.KnownFastWithin(runs) {
		t.Fatal("control result was not within")
	}
}

func TestMacOSReadCalibration(t *testing.T) {
	// QH26: the real volume's old atime moves after a confined decoy read.
	_, launcher := evidenceLauncher(t)
	w, c, s, _ := controlFixture(t, launcher)
	files, err := w.PrepareSelfTest()
	if err != nil {
		t.Fatal(err)
	}
	s.Directory = files.Home
	s.Paths.Values["HOME"], s.Paths.Values["PLANTED"] = files.Home, files.Calibration
	info, err := os.Stat(files.Calibration)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.ModTime().Sub(files.Before); got != 24*time.Hour {
		t.Fatalf("atime precedes mtime by %s want 24h", got)
	}
	// QH26: a confined marker-only run does not read the calibration decoy.
	noRead := s
	noRead.Fast = true
	process, err := c.Start(noRead)
	if err != nil {
		t.Fatal(err)
	}
	run, err := process.Wait()
	if err != nil || !run.Markers.Confinement || run.End.Code != 0 || run.End.Signal != 0 || len(run.Stdout) != 0 || len(run.Stderr) != 0 {
		t.Fatalf("no-read run=%+v err=%v", run, err)
	}
	unchanged, err := files.CalibrationAfter()
	if err != nil || !unchanged.Equal(files.Before) {
		t.Fatalf("no-read atime=%v before=%v err=%v", unchanged, files.Before, err)
	}
	before, after, read, err := confine.RunCalibration(c, s, files)
	if err != nil {
		t.Fatal(err)
	}
	if !read.Markers.Confinement || read.End.Code != 0 || read.End.Signal != 0 || string(read.Stdout) != "calibration_value\n" {
		t.Fatalf("read end=%+v marker=%+v out=%q stderr=%q", read.End, read.Markers, read.Stdout, read.Stderr)
	}
	if after.Equal(before) {
		t.Fatalf("access time did not move: before=%s after=%s", before, after)
	}
	facts := confine.ControlFacts{Before: before, After: after, CalibrationRead: read}
	if !facts.Apply(observe.Controls{}).ReadCalibrationMoved {
		t.Fatal("calibration result was not moved")
	}
}

func TestMacOSCalibrationErrorClearsRead(t *testing.T) {
	// QH26: a successful read cannot survive a metadata identity error.
	_, launcher := evidenceLauncher(t)
	w, c, s, _ := controlFixture(t, launcher)
	files, err := w.PrepareSelfTest()
	if err != nil {
		t.Fatal(err)
	}
	s.Directory = files.Home
	s.Paths.Values["HOME"] = files.Home
	s.Paths.Values["PLANTED"] = files.Calibration
	replacement := filepath.Join(files.Home, "replacement")
	if err := os.WriteFile(replacement, []byte("calibration_value\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, files.Calibration); err != nil {
		t.Fatal(err)
	}
	_, _, read, err := confine.RunCalibration(c, s, files)
	if err == nil || confine.CalibrationReadPassed(read) {
		t.Fatalf("read=%+v err=%v", read, err)
	}
}

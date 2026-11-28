//go:build darwin && macos_evidence

package confine_test

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/vinnyp/press-quality-lab/internal/confine"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/throwaway"
)

type sourceFixture struct {
	source    *confine.Source
	workspace *throwaway.Workspace
	base      confine.Invocation
}

func realSourceFixture(t *testing.T, afterWait func(observe.Spec) error) sourceFixture {
	t.Helper()
	_, launcher := evidenceLauncher(t)
	w, c, subject, computation := controlFixture(t, launcher)
	controls, err := confine.NewCopies(w.Path(), launcher, launcher)
	if err != nil {
		t.Fatal(err)
	}
	source := confine.NewSourceForTest(w, c, controls, subject, computation, func(key observe.Key, spec observe.Spec) (confine.Invocation, error) {
		inv := subject
		name, ok := key["witness"].(string)
		if !ok {
			return inv, fmt.Errorf("missing witness")
		}
		target := spec.PlantedPath
		if store, ok := key["store_target"].(string); ok {
			target = filepath.Join(spec.HomePath, store)
		}
		inv.Words = []string{confine.SelfTestMode, name, target, "1"}
		inv.Paths.PlantedDirs = nil
		if dirs, ok := key["resolver_dirs"].([]string); ok {
			inv.Paths.PlantedDirs = dirs
		}
		return inv, nil
	}, afterWait)
	return sourceFixture{source, w, subject}
}

func (f sourceFixture) planted(t *testing.T) observe.Spec {
	t.Helper()
	return f.plantedAt(t, "token")
}
func (f sourceFixture) plantedAt(t *testing.T, name string) observe.Spec {
	t.Helper()
	w, err := throwaway.Create(f.workspace.Path())
	if err != nil {
		t.Fatal(err)
	}
	home, err := filepath.EvalSymlinks(w.Path())
	if err != nil {
		t.Fatal(err)
	}
	if err := throwaway.EmptyHome(home); err != nil {
		t.Fatal(err)
	}
	if err := throwaway.WriteHomeFiles(home, []throwaway.NamedFile{{Name: name, Data: []byte("secret-token")}, {Name: "store", Data: []byte("store_value")}, {Name: ".pql-tmp/replacement", Data: []byte("replacement")}}); err != nil {
		t.Fatal(err)
	}
	canary := "pqlcanary11111111111111111111111111111111"
	digest, err := throwaway.PlantGivenCanary(home, name, "secret-token", canary)
	if err != nil {
		t.Fatal(err)
	}
	before, err := throwaway.SnapshotPlanted(home, name, digest)
	if err != nil {
		t.Fatal(err)
	}
	facts, err := throwaway.BackdatePlanted(home, name)
	if err != nil {
		t.Fatal(err)
	}
	return observe.Spec{HomePath: home, HomeKind: "planted", Canary: canary, PlantedPath: filepath.Join(home, name), PlantedBefore: facts, Before: before, BudgetSeconds: 10}
}

func sourceObserve(t *testing.T, f sourceFixture, name string, control bool) (observe.Key, observe.Run, observe.Spec) {
	t.Helper()
	spec := f.planted(t)
	spec.Control = control
	key := observe.Key{"words": []any{}, "witness": name, "home": "planted"}
	run, err := f.source.Observe(key, spec)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s control=%v before=%+v run=%+v raw=%+v", name, control, spec.PlantedBefore, run, observe.ProjectRun(run))
	return key, run, spec
}

func assertSourceRaw(t *testing.T, run observe.Run, output string, killed, moved bool, changes ...observe.HomeChange) {
	t.Helper()
	end := observe.End{}
	if killed {
		end = observe.End{Code: -1, Signal: 9}
	}
	if !reflect.DeepEqual(run.End, end) || run.Markers != (observe.Markers{Confinement: true}) || run.CopyChanged || run.OutputCutAfterExit || run.StdoutCut || run.StderrCut || string(run.Stdout) != output || len(run.Stderr) != 0 || run.StdoutSeen != int64(len(output)) || run.StderrSeen != 0 {
		t.Fatalf("raw end/streams %+v", run)
	}
	if run.Details.Planted == nil || run.Details.Planted.AtimeMoved != moved || !run.Details.Planted.SameFile {
		t.Fatalf("planted %+v", run.Details.Planted)
	}
	if !reflect.DeepEqual(run.Details.Home.Changes, changes) || run.Details.Home.Unreadable || run.Details.LeakScan.Unreadable || run.Details.LeakScan.BoundReached || len(run.Details.LeakScan.Leaks) != 0 {
		t.Fatalf("inspection %+v", run.Details)
	}
	if run.Details.Stdout.Digest != fmt.Sprintf("%x", sha256.Sum256([]byte(output))) || run.Details.Stdout.Head != output || !run.Details.Stdout.Kept {
		t.Fatalf("stdout facts %+v", run.Details.Stdout)
	}
}

func TestSourceRealRead(t *testing.T) {
	// QH27: a real read moves atime; only its otherwise identical read control is killed.
	f := realSourceFixture(t, nil)
	for range 25 {
		key, run, spec := sourceObserve(t, f, "real_read", false)
		assertSourceRaw(t, run, "attempt_read\nread_value\n", false, true)
		_, control, controlSpec := sourceObserve(t, f, "real_read", true)
		assertSourceRaw(t, control, "attempt_read\n", true, false)
		paths := f.base.Paths
		paths.Values = map[string]string{}
		for k, v := range f.base.Paths.Values {
			paths.Values[k] = v
		}
		paths.Values["HOME"], paths.Values["PLANTED"] = controlSpec.HomePath, controlSpec.PlantedPath
		paths.PlantedDirs = nil
		args, err := confine.Arguments(false, true, paths, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(args[1], "(deny file-read-data (literal (param \"PLANTED\")) (with send-signal SIGKILL))") {
			t.Fatal("missing QH11 read-control line")
		}
		if spec.Canary != controlSpec.Canary {
			t.Fatal("control canary differs")
		}
		assertSourceReadDecision(t, key, run, control, true)
		_, clean, _ := sourceObserve(t, f, "never_reads_secret", true)
		assertSourceRaw(t, clean, "never_reads_value\n", false, false)
	}
}

func TestSourceControlNeverReads(t *testing.T) {
	// QH27: QH11's extra line leaves a program that never opens the planted file alone.
	f := realSourceFixture(t, nil)
	for range 25 {
		for _, control := range []bool{false, true} {
			_, run, _ := sourceObserve(t, f, "never_reads_secret", control)
			assertSourceRaw(t, run, "never_reads_value\n", false, false)
		}
	}
}

func TestSourceNeverReads(t *testing.T) {
	// QH27: after-facts precede the snapshot and scan that themselves read home contents.
	f := realSourceFixture(t, nil)
	for range 25 {
		_, run, spec := sourceObserve(t, f, "never_reads_secret", false)
		assertSourceRaw(t, run, "never_reads_value\n", false, false)
		after, err := throwaway.InspectFile(spec.PlantedPath)
		if err != nil {
			t.Fatal(err)
		}
		if !after.Access.After(spec.PlantedBefore.Access) {
			t.Fatal("snapshot did not exercise the content reader")
		}
		_, control, _ := sourceObserve(t, f, "never_reads_secret", true)
		assertSourceRaw(t, control, "never_reads_value\n", false, false)
	}
}

func TestSourceHarnessRead(t *testing.T) {
	// QH27: QH28's external read passes both metadata signals, but its clean control prevents a held decision.
	f := realSourceFixture(t, func(spec observe.Spec) error { _, err := os.ReadFile(spec.PlantedPath); return err })
	for range 25 {
		key, run, _ := sourceObserve(t, f, "never_reads_secret", false)
		assertSourceRaw(t, run, "never_reads_value\n", false, true)
		_, control, _ := sourceObserve(t, f, "never_reads_secret", true)
		assertSourceRaw(t, control, "never_reads_value\n", false, true)
		assertSourceReadDecision(t, key, run, control, false)
	}
}

func sourceForgery(t *testing.T, name string) {
	t.Helper()
	storeActive := false
	var storeBefore throwaway.FileFacts
	f := realSourceFixture(t, func(spec observe.Spec) error {
		if storeActive {
			assertForgeryEffect(t, name, filepath.Join(spec.HomePath, "store"), storeBefore)
		}
		return nil
	})
	for range 25 {
		// Unconfined twin: the same native operation returns, with known output.
		spec := f.planted(t)
		_, program := f.base.Paths.Values["LAUNCHER"], f.base.Paths.Values["BIN"]
		log, err := os.CreateTemp(t.TempDir(), "witness")
		if err != nil {
			t.Fatal(err)
		}
		p, err := os.StartProcess(program, []string{program, confine.SelfTestMode, name, spec.PlantedPath, "1"}, &os.ProcAttr{Env: []string{}, Files: []*os.File{nil, log, log}})
		if err != nil {
			t.Fatal(err)
		}
		state, waitErr := p.Wait()
		closeErr := log.Close()
		b, readErr := os.ReadFile(log.Name())
		t.Logf("unconfined %s state=%v output=%q", name, state, b)
		if waitErr != nil || closeErr != nil || readErr != nil || !state.Success() || string(b) != "attempt_"+name+"\nreturned_"+name+"\n" {
			t.Fatalf("unconfined witness %v %v %v %v %q", state, waitErr, closeErr, readErr, b)
		}
		assertForgeryEffect(t, name, spec.PlantedPath, spec.PlantedBefore)
		storeSpec := f.planted(t)
		store := filepath.Join(storeSpec.HomePath, "store")
		storeBefore, err = throwaway.InspectFile(store)
		if err != nil {
			t.Fatal(err)
		}
		storeActive = true
		storeRun, err := f.source.Observe(observe.Key{"witness": name, "store_target": "store"}, storeSpec)
		storeActive = false
		if err != nil {
			t.Fatal(err)
		}
		if storeRun.End != (observe.End{}) || storeRun.Markers != (observe.Markers{Confinement: true}) || string(storeRun.Stdout) != "attempt_"+name+"\nreturned_"+name+"\n" || len(storeRun.Stderr) != 0 {
			t.Fatalf("confined store twin %+v", storeRun)
		}
		_, run, _ := sourceObserve(t, f, name, false)
		assertSourceRaw(t, run, "attempt_"+name+"\n", true, false)
		_, clean, _ := sourceObserve(t, f, "never_reads_secret", false)
		assertSourceRaw(t, clean, "never_reads_value\n", false, false)
	}
}

func assertForgeryEffect(t *testing.T, name, target string, before throwaway.FileFacts) {
	t.Helper()
	switch name {
	case "forge_utimes", "forge_setattrlist", "forge_directory":
		after, err := throwaway.InspectFile(target)
		if err != nil || !after.Access.Equal(time.Unix(4102444800, 0)) {
			t.Fatalf("future access time %+v: %v", after, err)
		}
	case "forge_write":
		data, err := os.ReadFile(target)
		if err != nil || !strings.HasPrefix(string(data), "forged") {
			t.Fatalf("forged content %q: %v", data, err)
		}
	case "forge_rename":
		_, err := os.Lstat(target)
		moved, movedErr := throwaway.InspectFile(target + ".renamed")
		if !os.IsNotExist(err) || movedErr != nil || moved.Inode != before.Inode || moved.Device != before.Device {
			t.Fatalf("renamed identity %+v: %v %v", moved, err, movedErr)
		}
	case "forge_replace":
		data, err := os.ReadFile(target)
		after, statErr := throwaway.InspectFile(target)
		if err != nil || statErr != nil || string(data) != "replacement" || after.Inode == before.Inode {
			t.Fatalf("replacement %q %+v: %v %v", data, after, err, statErr)
		}
	default:
		t.Fatalf("no effect oracle for %s", name)
	}
}

func TestSourceForgeryUtimes(t *testing.T) {
	// QH27: utimes never reads the file and is killed by the planted write rule.
	sourceForgery(t, "forge_utimes")
}
func TestSourceForgerySetattrlist(t *testing.T) {
	// QH27: setattrlist never reads the file and is killed by the planted write rule.
	sourceForgery(t, "forge_setattrlist")
}
func TestSourceForgeryWrite(t *testing.T) {
	// QH27: the direct write is killed before changing content or atime.
	sourceForgery(t, "forge_write")
}
func TestSourceForgeryRename(t *testing.T) {
	// QH27: renaming the planted file is killed before its identity changes.
	sourceForgery(t, "forge_rename")
}
func TestSourceForgeryReplace(t *testing.T) {
	// QH27: replacing the planted file is killed before its identity changes.
	sourceForgery(t, "forge_replace")
}

func TestSourceStoreControl(t *testing.T) {
	// QP9: a real control copy detects an appended byte; its untouched twin detects none.
	f := realSourceFixture(t, nil)
	seed := t.TempDir()
	if err := throwaway.WriteHomeFiles(seed, []throwaway.NamedFile{{Name: "store", Data: []byte("store_value")}}); err != nil {
		t.Fatal(err)
	}
	for range 25 {
		for _, appendByte := range []bool{true, false} {
			w, err := throwaway.Create(f.workspace.Path())
			if err != nil {
				t.Fatal(err)
			}
			home := filepath.Join(w.Path(), "home")
			if err := throwaway.CopySeed(seed, home); err != nil {
				t.Fatal(err)
			}
			if err := throwaway.EmptyHome(home); err != nil {
				t.Fatal(err)
			}
			before, err := throwaway.SnapshotHome(home)
			if err != nil {
				t.Fatal(err)
			}
			spec := observe.Spec{HomePath: home, Before: before}
			if appendByte {
				if err := throwaway.AppendStoreControl(spec.HomePath, "store"); err != nil {
					t.Fatal(err)
				}
			}
			after, err := throwaway.SnapshotHome(spec.HomePath)
			if err != nil {
				t.Fatal(err)
			}
			changes, unreadable := throwaway.CompareHomes(spec.Before, after)
			detected := len(changes) == 1 && changes[0].Path == "store"
			t.Logf("append=%v changes=%+v unreadable=%v", appendByte, changes, unreadable)
			if unreadable || detected != appendByte || len(changes) != map[bool]int{true: 1, false: 0}[appendByte] {
				t.Fatal("control copy comparison")
			}
			spec = f.planted(t)
			spec.StoreControl = &detected
			run, err := f.source.Observe(observe.Key{"witness": "never_reads_secret"}, spec)
			if err != nil {
				t.Fatal(err)
			}
			assertSourceRaw(t, run, "never_reads_value\n", false, false)
			if run.Details.Home.ControlDetected == nil || *run.Details.Home.ControlDetected != appendByte {
				t.Fatalf("control fact %+v", run.Details.Home)
			}
		}
	}
}

func TestSourceGradeControls(t *testing.T) {
	// QH26: the real source uses the suite's successful read and moved metadata, and caches grade-wide facts.
	_, launcher := evidenceLauncher(t)
	w, c, subject, computation := controlFixture(t, launcher)
	controls, err := confine.NewCopies(w.Path(), launcher, launcher)
	if err != nil {
		t.Fatal(err)
	}
	for range 25 {
		source := confine.NewSourceForTest(w, c, controls, subject, computation, nil, nil)
		got, err := source.Controls()
		if err != nil {
			t.Fatal(err)
		}
		probe, suite, product, build := confine.SourceControlsForTest(source)
		t.Logf("product=%q build=%q probe=%+v suite=%+v controls=%+v", product, build, probe, suite, got)
		assertSelfTestPrograms(t, suite)
		if !got.ReadCalibrationMoved || !got.KnownFastWithin || !got.SelfTestPassed || !got.PrivateCopyUnchanged || !got.ConfinementApplied || got.SandboxExecExit != nil || got.SystemNamed != (product == "27.0.1" || product == "27.2") || build == "" {
			t.Fatalf("controls %+v", got)
		}
		if suite.Before.IsZero() || !suite.After.After(suite.Before) || string(suite.CalibrationRead.Stdout) != "calibration_value\n" {
			t.Fatalf("calibration %+v", suite)
		}
		again, err := source.Controls()
		if err != nil || !reflect.DeepEqual(got, again) {
			t.Fatalf("cached facts %+v %v", again, err)
		}
	}
}

func TestSourceLeafLink(t *testing.T) {
	// QH25: source inspection records a link without reading or changing its outside target.
	sourceHostileEntry(t, false)
}

func TestSourceFIFO(t *testing.T) {
	// QH25: source inspection records a FIFO without opening it; the clean twin still completes.
	sourceHostileEntry(t, true)
}

func sourceHostileEntry(t *testing.T, fifo bool) {
	t.Helper()
	f := realSourceFixture(t, nil)
	for range 25 {
		for _, hostile := range []bool{false, true} {
			spec := f.planted(t)
			outside := filepath.Join(t.TempDir(), "outside")
			if err := os.WriteFile(outside, []byte("outside_value"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(outside, 0400); err != nil {
				t.Fatal(err)
			}
			original, err := os.Lstat(outside)
			if err != nil {
				t.Fatal(err)
			}
			if hostile {
				name := filepath.Join(spec.HomePath, "entry")
				if fifo {
					err = unix.Mkfifo(name, 0600)
				} else {
					err = os.Symlink(outside, name)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			run, err := f.source.Observe(observe.Key{"witness": "never_reads_secret"}, spec)
			if err != nil {
				t.Fatal(err)
			}
			want := []observe.HomeChange(nil)
			if hostile {
				kind := "link"
				if fifo {
					kind = "special"
				}
				want = []observe.HomeChange{{Path: "entry", Kind: kind, Change: "created"}}
			}
			assertSourceRaw(t, run, "never_reads_value\n", false, false, want...)
			t.Logf("fifo=%v hostile=%v home=%+v", fifo, hostile, run.Details.Home)
			if run.Details.Home.Unreadable || !reflect.DeepEqual(run.Details.Home.Changes, want) {
				t.Fatalf("home %+v want %+v", run.Details.Home, want)
			}
			current, err := os.Lstat(outside)
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(original, current) || current.Mode() != original.Mode() || current.ModTime() != original.ModTime() {
				t.Fatal("outside metadata changed")
			}
			b, err := os.ReadFile(outside)
			if err != nil || string(b) != "outside_value" {
				t.Fatalf("outside %q %v", b, err)
			}
		}
	}
}

func TestSourcePrivateCopyChanged(t *testing.T) {
	// QH8: the grade-wide check detects a changed copy, and every later use refuses without markers or an end.
	f := realSourceFixture(t, nil)
	clean := realSourceFixture(t, nil)
	// The healthy twin establishes that this program/profile starts and prints its value.
	_, good, _ := sourceObserve(t, f, "never_reads_secret", false)
	assertSourceRaw(t, good, "never_reads_value\n", false, false)
	copied := f.base.Paths.Values["BIN"]
	if err := os.Chmod(copied, 0600); err != nil {
		t.Fatal(err)
	}
	controls, err := f.source.Controls()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("changed-copy grade controls=%+v", controls)
	if controls.PrivateCopyUnchanged || !controls.ConfinementApplied || !controls.SelfTestPassed {
		t.Fatal("grade-wide copy check did not isolate changed subject")
	}
	for range 25 {
		_, run, _ := sourceObserve(t, f, "never_reads_secret", false)
		if !reflect.DeepEqual(run, observe.Run{CopyChanged: true}) {
			t.Fatalf("refused run %+v", run)
		}
		_, twin, _ := sourceObserve(t, clean, "never_reads_secret", false)
		assertSourceRaw(t, twin, "never_reads_value\n", false, false)
	}
}

func TestSourceComputation(t *testing.T) {
	// QH8, QA6: a real computation uses a persistent private copy and the shared exact-value parser.
	_, launcher := evidenceLauncher(t)
	program := evidenceProgram(t, `package main
import "fmt"
func main(){fmt.Print("{\"value\":1}\n")}
`)
	program, err := filepath.EvalSymlinks(program)
	if err != nil {
		t.Fatal(err)
	}
	w, controls, subject, computation := controlFixture(t, launcher)
	copies, err := confine.NewCopies(w.Path(), launcher, launcher)
	if err != nil {
		t.Fatal(err)
	}
	source := confine.NewSourceForTest(w, copies, controls, subject, computation, func(observe.Key, observe.Spec) (confine.Invocation, error) {
		inv := computation
		inv.Executable = program
		return inv, nil
	}, nil)
	f := sourceFixture{source, w, subject}
	for range 25 {
		spec := f.planted(t)
		run, err := source.Observe(observe.Key{"computation": []any{program}}, spec)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("computation raw=%+v", observe.ProjectRun(run))
		assertSourceRaw(t, run, "{\"value\":1}\n", false, false)
		if run.Details.Printed == nil || !reflect.DeepEqual(run.Details.Printed.Value, strictjson.Object{"value": strictjson.Number("1")}) {
			t.Fatalf("printed %+v", run.Details.Printed)
		}
		// The subject profile is the clean twin for this computation's literal output.
		process, err := copies.Start(confine.Invocation{Workspace: w, Paths: subject.Paths, Directory: spec.HomePath, Words: []string{confine.SelfTestMode, "never_reads_secret", spec.PlantedPath, "1"}, Budget: 10 * time.Second})
		if err != nil {
			t.Fatal(err)
		}
		twin, err := process.Wait()
		if err != nil {
			t.Fatal(err)
		}
		if twin.Markers != (observe.Markers{Confinement: true}) || twin.End != (observe.End{}) || string(twin.Stdout) != "never_reads_value\n" || len(twin.Stderr) != 0 {
			t.Fatalf("clean twin %+v", twin)
		}
	}
}

func TestSourceDirectoryRename(t *testing.T) {
	// QH11, QH27: renaming an ancestor is killed before the file can be retimed through its new path.
	storeActive := false
	f := realSourceFixture(t, func(spec observe.Spec) error {
		if storeActive {
			assertForgeryEffect(t, "forge_directory", filepath.Join(spec.HomePath, "ordinary/store"), throwaway.FileFacts{})
		}
		return nil
	})
	for range 25 {
		spec := f.plantedAt(t, "d/token")
		program := f.base.Paths.Values["BIN"]
		log, err := os.CreateTemp(t.TempDir(), "witness")
		if err != nil {
			t.Fatal(err)
		}
		process, err := os.StartProcess(program, []string{program, confine.SelfTestMode, "forge_directory", spec.PlantedPath, "1"}, &os.ProcAttr{Files: []*os.File{nil, log, log}})
		if err != nil {
			t.Fatal(err)
		}
		state, waitErr := process.Wait()
		closeErr := log.Close()
		output, readErr := os.ReadFile(log.Name())
		if waitErr != nil || closeErr != nil || readErr != nil || !state.Success() || string(output) != "attempt_forge_directory\nreturned_forge_directory\n" {
			t.Fatalf("unconfined %v %v %v %v %q", state, waitErr, closeErr, readErr, output)
		}
		assertForgeryEffect(t, "forge_directory", spec.PlantedPath, spec.PlantedBefore)
		spec = f.plantedAt(t, "d/token")
		run, err := f.source.Observe(observe.Key{"witness": "forge_directory"}, spec)
		if err != nil {
			t.Fatal(err)
		}
		assertSourceRaw(t, run, "attempt_forge_directory\n", true, false)
		spec = f.plantedAt(t, "d/token")
		clean, err := f.source.Observe(observe.Key{"witness": "real_read"}, spec)
		if err != nil {
			t.Fatal(err)
		}
		assertSourceRaw(t, clean, "attempt_read\nread_value\n", false, true)
		storeSpec := f.planted(t)
		if err := throwaway.WriteHomeFiles(storeSpec.HomePath, []throwaway.NamedFile{{Name: "ordinary/store", Data: []byte("store_value")}}); err != nil {
			t.Fatal(err)
		}
		storeActive = true
		storeRun, err := f.source.Observe(observe.Key{"witness": "forge_directory", "store_target": "ordinary/store"}, storeSpec)
		storeActive = false
		if err != nil || storeRun.End != (observe.End{}) || storeRun.Markers != (observe.Markers{Confinement: true}) || string(storeRun.Stdout) != "attempt_forge_directory\nreturned_forge_directory\n" || len(storeRun.Stderr) != 0 {
			t.Fatalf("confined directory twin %+v: %v", storeRun, err)
		}
	}
}

func TestSourceChangedControlCopy(t *testing.T) {
	// QH8, QH6: a refused probe due to changed control copies is an engine fault, never a failed confinement observation.
	_, launcher := evidenceLauncher(t)
	w, c, subject, computation := controlFixture(t, launcher)
	controls, err := confine.NewCopies(w.Path(), launcher, launcher)
	if err != nil {
		t.Fatal(err)
	}
	_, program := controls.CopyPaths()
	if err := os.WriteFile(program, []byte("changed"), 0700); err != nil {
		t.Fatal(err)
	}
	source := confine.NewSourceForTest(w, c, controls, subject, computation, nil, nil)
	facts, err := source.Controls()
	if err == nil || err.Error() != "control private copy changed before probe" || facts.PrivateCopyUnchanged || facts.SandboxExecExit != nil {
		t.Fatalf("%+v %v", facts, err)
	}
}

func TestSourceRefusesResolverDirectories(t *testing.T) {
	// QH11: the source alone supplies planted ancestor rules, including the no-ancestor home case.
	f := realSourceFixture(t, nil)
	for range 25 {
		spec := f.plantedAt(t, "d/token")
		run, err := f.source.Observe(observe.Key{"witness": "real_read"}, spec)
		if err != nil {
			t.Fatal(err)
		}
		assertSourceRaw(t, run, "attempt_read\nread_value\n", false, true)
		for _, dirs := range [][]string{{}, {spec.HomePath}} {
			_, err := f.source.Observe(observe.Key{"witness": "real_read", "resolver_dirs": dirs}, spec)
			if err == nil || err.Error() != "resolver supplied planted directories" {
				t.Fatalf("resolver refusal %v", err)
			}
		}
	}
}

func TestSourceLeakScan(t *testing.T) {
	// QP12, QP13, QT5: the real source finds encoded streams and plain home files.
	f := realSourceFixture(t, nil)
	for range 25 {
		_, run, spec := sourceObserve(t, f, "leak_secret", false)
		want := []observe.Leak{
			{Place: "stdout", Encoding: "base64"},
			{Place: "stdout", Encoding: "base64-nopad"},
			{Place: "stdout", Encoding: "base64url"},
			{Place: "stdout", Encoding: "base64url-nopad"},
			{Place: "home/leaked", Encoding: "plain"},
		}
		if !reflect.DeepEqual(run.Details.LeakScan.Leaks, want) || run.Details.LeakScan.Unreadable || run.Details.LeakScan.BoundReached || run.End != (observe.End{}) || run.Markers != (observe.Markers{Confinement: true}) || string(run.Stdout) != base64.StdEncoding.EncodeToString([]byte(spec.Canary))+"\n" || len(run.Stderr) != 0 {
			t.Fatalf("leaks %+v", run)
		}
		_, clean, _ := sourceObserve(t, f, "never_reads_secret", false)
		assertSourceRaw(t, clean, "never_reads_value\n", false, false)
	}
}

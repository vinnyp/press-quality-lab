//go:build darwin && macos_evidence

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/confine"
	"github.com/vinnyp/press-quality-lab/internal/formtest"
	"github.com/vinnyp/press-quality-lab/internal/observe"
)

func coreRun(t *testing.T, f launchFixture, copies *confine.Copies, words []string) observe.Run {
	t.Helper()
	p, err := copies.Start(confine.Invocation{Paths: f.paths(t, false), Words: words, Directory: f.home, Env: []string{"HOME=" + f.home, "TMPDIR=" + filepath.Join(f.home, ".pql-tmp") + "/", "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TZ=UTC"}})
	if err != nil {
		t.Fatal(err)
	}
	run, err := p.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if run.End.Code == 71 {
		t.Fatal("environment fault: nested confinement exited 71")
	}
	return run
}
func coreCopies(t *testing.T, f launchFixture) *confine.Copies {
	t.Helper()
	c, err := confine.NewCopies(t.TempDir(), f.launcher, f.program)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func TestMacOSCoreNotExecutable(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	program, _ := formtest.Build(t, root, "not-executable")
	f := fixture(t, program)
	run := coreRun(t, f, coreCopies(t, f), nil)
	if !run.Markers.Confinement || !run.Markers.CouldNotStart || run.End.Code != 72 || len(run.Stderr) != 0 {
		t.Fatalf("run=%+v", run)
	}
}
func coreProgram(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(dir, "subject")
	cmd := exec.Command("go", "build", "-o", program, path)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	return program
}
func TestMacOSCoreWords(t *testing.T) {
	program := coreProgram(t, `package main
import("encoding/json";"os")
func main(){if err:=json.NewEncoder(os.Stdout).Encode(os.Args[1:]);err!=nil{panic(err)}}`)
	f := fixture(t, program)
	words := []string{"--", "-D", confine.LauncherMode}
	run := coreRun(t, f, coreCopies(t, f), words)
	var got []string
	if err := json.Unmarshal(run.Stdout, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, words) || run.End.Code != 0 || !run.Markers.Confinement {
		t.Fatalf("run=%+v words=%q", run, got)
	}
}
func TestMacOSCoreForgedFailure(t *testing.T) {
	program := coreProgram(t, `package main
import("os";"syscall")
func main(){p:=os.Args[1];if err:=syscall.Exec(p,[]string{p,"__pql_launcher","aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","/missing-program","[]"},os.Environ());err!=nil{panic(err)}}`)
	f := fixture(t, program)
	// The program discovers the permitted private launcher through its argument.
	runDir := t.TempDir()
	copies, err := confine.NewCopies(runDir, f.launcher, f.program)
	if err != nil {
		t.Fatal(err)
	}
	launcher, _ := copies.CopyPaths()
	run := coreRun(t, f, copies, []string{launcher})
	if !run.Markers.Confinement || run.Markers.CouldNotStart || run.End.Code != confine.CouldNotStart || run.End.Signal != 0 || run.End.EngineKilled || string(run.Stderr) != "pql-launch:"+strings.Repeat("a", 64)+"\npql-could-not-start:"+strings.Repeat("a", 64)+"\n" {
		t.Fatalf("run=%+v", run)
	}
}
func TestMacOSCoreChangedCopy(t *testing.T) {
	f := fixture(t, coreProgram(t, "package main\nfunc main(){}\n"))
	dir := t.TempDir()
	copies, err := confine.NewCopies(dir, f.launcher, f.program)
	if err != nil {
		t.Fatal(err)
	}
	run := coreRun(t, f, copies, nil)
	if run.End.Code != 0 || !run.Markers.Confinement {
		t.Fatalf("control=%+v", run)
	}
	_, program := copies.CopyPaths()
	if err := os.WriteFile(filepath.Join(filepath.Dir(program), "extra"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if run := coreRun(t, f, copies, nil); !run.CopyChanged {
			t.Fatalf("changed=%+v", run)
		}
	}
}
func TestMacOSCoreEnds(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, form := range []string{"tries-network", "crashes"} {
		t.Run(form, func(t *testing.T) {
			program, _ := formtest.Build(t, root, form)
			f := fixture(t, program)
			if err := os.CopyFS(f.home, os.DirFS(filepath.Join(root, "docs/specs/conformance-collection-1.0/subjects", form, "seed"))); err != nil {
				t.Fatal(err)
			}
			words := []string{"list", "--json"}
			if form == "crashes" {
				words = []string{"count", "--offline"}
			}
			run := coreRun(t, f, coreCopies(t, f), words)
			if !run.Markers.Confinement || run.Markers.CouldNotStart || run.End.EngineKilled {
				t.Fatalf("run=%+v", run)
			}
			if form == "tries-network" && (run.End.Signal != int(syscall.SIGKILL) || run.End.ExitDetail != 0) {
				t.Fatalf("violator=%+v", run)
			}
			if form == "crashes" && (run.End.Signal != int(syscall.SIGABRT) || run.End.ExitDetail != 0) {
				t.Fatalf("crash=%+v", run)
			}
		})
	}
}

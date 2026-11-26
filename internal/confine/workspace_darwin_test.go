package confine

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/throwaway"
	"golang.org/x/sys/unix"
)

func TestInvocationWorkspaceGroupLifecycle(t *testing.T) {
	// KTD5, QE20, QH23: publication precedes handoff; ending precedes forgetting.
	// This replaces only the spawn seam with an unconfined shell, never sandbox-exec.
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	w, err := throwaway.Create(parent)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Publish(throwaway.Process{PID: 41, StartTime: "engine"}); err != nil {
		t.Fatal(err)
	}
	copies, err := NewCopies(w.Path(), "/bin/sh", "/bin/sh")
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	for _, name := range commonNames {
		values[name] = "/"
	}
	p, err := copies.startWith(Invocation{Workspace: w, Paths: Paths{Values: values}}, func(string, []string, string, []string, string) (Process, error) {
		return spawn("/bin/sh", []string{"sh", "-c", "printf live; sleep 0.05"}, "", []string{"PATH=/usr/bin:/bin"}, "test")
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := p.Kill(); err != nil {
			t.Error(err)
		}
		if _, err := p.Wait(); err != nil {
			t.Error(err)
		}
	})
	real, ok := p.(*process)
	if !ok {
		t.Fatal("not a native process")
	}
	kernel, err := unix.SysctlKinfoProc("kern.proc.pid", real.pid)
	if err != nil {
		t.Fatal(err)
	}
	start := kernel.Proc.P_starttime
	want := fmt.Sprintf("{\"groups\":[{\"pid\":%d,\"start_time\":\"%d:%d\"}],\"pid\":41,\"start_time\":\"engine\"}\n", real.pid, start.Sec, start.Usec)
	data, err := os.ReadFile(filepath.Join(w.Path(), "process.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("unpublished handoff: %s want %s", data, want)
	}
	run, err := p.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if run.End.Code != 0 || string(run.Stdout) != "live" {
		t.Fatalf("clean control: %+v %q", run.End, run.Stdout)
	}
	data, err = os.ReadFile(filepath.Join(w.Path(), "process.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{\"pid\":41,\"start_time\":\"engine\"}\n" {
		t.Fatalf("ended group still recorded: %s", data)
	}
	if err := w.EndGroups(io.Discard); err != nil {
		t.Fatal(err)
	}
	spawned := 0
	shutdownProcess, err := copies.startWith(Invocation{Workspace: w, Paths: Paths{Values: values}}, func(string, []string, string, []string, string) (Process, error) {
		p, err := spawn("/bin/sh", []string{"sh", "-c", "sleep 30"}, "", []string{"PATH=/usr/bin:/bin"}, "test")
		if err == nil {
			t.Cleanup(func() {
				if err := p.Kill(); err != nil {
					t.Error(err)
				}
				if _, err := p.Wait(); err != nil {
					t.Error(err)
				}
			})
		}
		if err == nil {
			native, ok := p.(*process)
			if !ok {
				t.Fatal("not native")
			}
			spawned = native.pid
		}
		return p, err
	})
	if err == nil || shutdownProcess != nil || spawned == 0 {
		t.Fatalf("shutdown spawn: %v %v pid=%d", shutdownProcess, err, spawned)
	}
	procs, lookupErr := unix.SysctlKinfoProcSlice("kern.proc.pid", spawned)
	if lookupErr != nil && !errors.Is(lookupErr, unix.ESRCH) {
		t.Fatal(lookupErr)
	}
	if len(procs) != 0 {
		t.Fatal("spawn survived failed recording")
	}
}

func TestStartRequiresWorkspace(t *testing.T) {
	// QE20, KTD5: the real entry point refuses an unrecordable invocation.
	called := false
	p, err := (&Copies{}).startWith(Invocation{}, func(string, []string, string, []string, string) (Process, error) { called = true; return nil, nil })
	if err == nil || p != nil || called {
		t.Fatalf("missing workspace: %v %v %v", p, err, called)
	}
}

package confine

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

// This test starts only an unconfined shell, never sandbox-exec.
func TestWaitRemovesCaptures(t *testing.T) {
	run := t.TempDir()
	copies, err := NewCopies(run, "/bin/sh", "/bin/sh")
	if err != nil {
		t.Fatal(err)
	}
	launcher, program := copies.CopyPaths()
	want := []string{filepath.Base(filepath.Dir(launcher)), filepath.Base(filepath.Dir(program)), filepath.Base(copies.scratch)}
	slices.Sort(want)
	for range 2 {
		stdout, err := copies.capture(".stdout-")
		if err != nil {
			t.Fatal(err)
		}
		stderr, err := copies.capture(".stderr-")
		if err != nil {
			t.Fatal(err)
		}
		queue, err := unix.Kqueue()
		if err != nil {
			t.Fatal(err)
		}
		unix.CloseOnExec(queue)
		pid, err := syscall.ForkExec("/bin/sh", []string{"sh", "-c", "printf output; printf 'pql-launch:test\ndiagnostic' >&2"}, &syscall.ProcAttr{
			Env:   []string{"PATH=/usr/bin:/bin"},
			Files: []uintptr{0, stdout.Fd(), stderr.Fd()},
			Sys:   &syscall.SysProcAttr{Setpgid: true},
		})
		if err != nil {
			t.Fatal(err)
		}
		p := &process{pid: pid, queue: queue, stdout: stdout, stderr: stderr, marker: "test"}
		got, err := p.Wait()
		if err != nil {
			t.Fatal(err)
		}
		if got.End.Code != 0 || !got.Markers.Confinement || string(got.Stdout) != "output" || string(got.Stderr) != "diagnostic" {
			t.Fatalf("run=%+v", got)
		}
		entries, err := os.ReadDir(run)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, entry := range entries {
			if !entry.IsDir() {
				t.Fatalf("run contains non-directory %q", entry.Name())
			}
			names = append(names, entry.Name())
		}
		if !reflect.DeepEqual(names, want) {
			t.Fatalf("run entries=%q, want %q", names, want)
		}
		captures, err := os.ReadDir(copies.scratch)
		if err != nil {
			t.Fatal(err)
		}
		if len(captures) != 0 {
			t.Fatalf("captures remain after Wait: %v", captures)
		}
	}
}

package grade

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/vinnyp/press-quality-lab/internal/collect"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/throwaway"
)

func TestSignalGroupHelper(t *testing.T) {
	if os.Getenv("PQL_GROUP_HELPER") != "1" {
		return
	}
	// QH23: a real, test-owned group; no confined program is invoked.
	if _, err := fmt.Fprintln(os.Stdout, "ready"); err != nil {
		os.Exit(90)
	}
	time.Sleep(30 * time.Second)
	os.Exit(0)
}

func startSignalGroup(t *testing.T) (int, *os.File) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	pid, err := syscall.ForkExec(exe, []string{exe, "-test.run=^TestSignalGroupHelper$"}, &syscall.ProcAttr{Env: append(os.Environ(), "PQL_GROUP_HELPER=1"), Files: []uintptr{0, w.Fd(), 2}, Sys: &syscall.SysProcAttr{Setpgid: true}})
	if closeErr := w.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := r.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			t.Error(err)
		}
	})
	var buf [6]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil || string(buf[:]) != "ready\n" {
		t.Fatalf("helper %q: %v", buf, err)
	}
	return pid, r
}

func TestSignalsEndGroupsAndCleanup(t *testing.T) {
	// KTD17, QH23, QE20: all four signals use the exit funnel, including reap.
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT} {
		t.Run(fmt.Sprintf("Signal%d", sig), func(t *testing.T) {
			o, d, _ := fixture(t)
			parent := t.TempDir()
			if err := os.Chmod(parent, 0700); err != nil {
				t.Fatal(err)
			}
			w, err := throwaway.Create(parent)
			if err != nil {
				t.Fatal(err)
			}
			if err := w.Publish(throwaway.Process{PID: os.Getpid(), StartTime: "test"}); err != nil {
				t.Fatal(err)
			}
			d.Workspace = func() (*throwaway.Workspace, error) { return w, nil }
			ch := make(chan os.Signal, 1)
			d.Signals = ch
			pid, _ := startSignalGroup(t)
			reaped := false
			t.Cleanup(func() {
				if !reaped {
					if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
						t.Error(err)
					}
					var s syscall.WaitStatus
					if _, err := syscall.Wait4(pid, &s, 0, nil); err != nil {
						t.Error(err)
					}
				}
			})
			start, err := throwaway.ProcessStart(pid)
			if err != nil {
				t.Fatal(err)
			}
			if err := w.RecordGroup(throwaway.Process{PID: pid, StartTime: start}); err != nil {
				t.Fatal(err)
			}
			release := make(chan struct{})
			reapedDone := make(chan struct{})
			var killStatus syscall.WaitStatus
			d.Collect = func(plan.Plan, observe.Source) ([]collect.Check, error) {
				ch <- sig
				var status syscall.WaitStatus
				_, err := syscall.Wait4(pid, &status, 0, nil)
				if err != nil {
					return nil, err
				}
				reaped = true
				killStatus = status
				close(reapedDone)
				<-release
				if !status.Signaled() || status.Signal() != syscall.SIGKILL {
					return nil, fmt.Errorf("group not killed: %v", status)
				}
				return nil, nil
			}
			type result struct {
				code int
				err  error
			}
			returned := make(chan result, 1)
			go func() { code, err := Run(o, d, io.Discard); returned <- result{code, err} }()
			select {
			case <-reapedDone:
			case <-time.After(time.Second):
				close(release)
				t.Fatal("funnel did not end group")
			}
			select {
			case r := <-returned:
				close(release)
				t.Fatalf("returned before collector release: %+v", r)
			case <-time.After(50 * time.Millisecond):
			}
			close(release)
			r := <-returned
			code, err := r.code, r.err
			if !killStatus.Signaled() || killStatus.Signal() != syscall.SIGKILL {
				t.Fatalf("kill status %v", killStatus)
			}
			if code != 70 || err == nil || !strings.Contains(err.Error(), sig.String()) {
				t.Fatalf("%d %v", code, err)
			}
			if !reaped {
				t.Fatal("returned without waiting for collector")
			}
			if _, err := os.Lstat(w.Path()); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("workspace remains: %v", err)
			}
		})
	}
}

func TestCleanupFailureIsNamedAndRetried(t *testing.T) {
	// QE20, E-11: a failed removal names the home and a later sweep retries.
	o, d, _ := fixture(t)
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	w, err := throwaway.Create(parent)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Publish(throwaway.Process{PID: 41, StartTime: "old"}); err != nil {
		t.Fatal(err)
	}
	d.Workspace = func() (*throwaway.Workspace, error) { return w, nil }
	// Positive control from this fixture: unchanged parent permissions remove it.
	if code, err := Run(o, d, io.Discard); code != 4 || err != nil {
		t.Fatalf("clean control %d %v", code, err)
	}
	if _, err := os.Lstat(w.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("clean control left workspace: %v", err)
	}
	w, err = throwaway.Create(parent)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Publish(throwaway.Process{PID: 41, StartTime: "old"}); err != nil {
		t.Fatal(err)
	}
	original, readErr := os.ReadFile(filepath.Join(w.Path(), "process.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	d.Source = func() (observe.Source, func() error, error) {
		return ReleasedSource{}, func() error { return os.Chmod(parent, 0500) }, nil
	}
	defer func() {
		if err := os.Chmod(parent, 0700); err != nil {
			t.Error(err)
		}
	}()
	if os.Geteuid() == 0 {
		t.Fatal("cleanup permission fixture requires a non-root user")
	}
	var output bytes.Buffer
	code, err := Run(o, d, &output)
	if code != 70 || err == nil || !strings.Contains(err.Error(), w.Path()) {
		t.Fatalf("%d %v", code, err)
	}
	if !strings.Contains(output.String(), "cleanup failed; directory left behind: "+Escape(w.Path())+"\n") {
		t.Fatalf("scorecard does not name directory: %q", output.String())
	}
	got, readErr := os.ReadFile(filepath.Join(w.Path(), "process.json"))
	if readErr != nil || !bytes.Equal(got, original) {
		t.Fatalf("publication changed: %q %v", got, readErr)
	}
	if _, err := os.Lstat(w.Path()); err != nil {
		t.Fatal("failed directory missing", err)
	}
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	if err := throwaway.Sweep(parent, func(int) (string, error) { return "", os.ErrNotExist }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(w.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("retry: %v", err)
	}
}

func TestStartupSweepBeforeInputRefusal(t *testing.T) {
	// QE20, KTD5: a refused next run still sweeps the abandoned workspace first.
	o, d, _ := fixture(t)
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	old, err := throwaway.Create(parent)
	if err != nil {
		t.Fatal(err)
	}
	if err := old.Publish(throwaway.Process{PID: 41, StartTime: "old"}); err != nil {
		t.Fatal(err)
	}
	called := false
	d.Workspace = func() (*throwaway.Workspace, error) {
		if err := throwaway.Sweep(parent, func(int) (string, error) { return "", os.ErrNotExist }); err != nil {
			return nil, err
		}
		called = true
		return throwaway.Create(parent)
	}
	d.Open = func(string) (io.ReadCloser, error) {
		if !called {
			t.Fatal("subject read before startup sweep")
		}
		if _, err := os.Stat(old.Path()); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("abandoned workspace remains", err)
		}
		return nil, os.ErrNotExist
	}
	code, err := Run(o, d, io.Discard)
	if code != 2 || err == nil {
		t.Fatalf("input refusal: %d %v", code, err)
	}
	if !called {
		t.Fatal("startup sweep not evaluated")
	}
}

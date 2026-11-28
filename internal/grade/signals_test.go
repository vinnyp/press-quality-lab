package grade

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/vinnyp/press-quality-lab/internal/collect"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/throwaway"
)

func TestInterruptedRunHelper(t *testing.T) {
	// KTD17, QE20: interrupted helpers use parent-owned fixtures.
	if os.Getenv("PQL_INTERRUPTED_HELPER") != "1" {
		return
	}
	o, d, _ := fixtureAt(t, os.Getenv("PQL_SIGNAL_FIXTURE"))
	if filepath.Dir(o.SubjectFile) != os.Getenv("PQL_SIGNAL_FIXTURE") {
		t.Fatal("helper fixture escaped parent directory")
	}
	var w *throwaway.Workspace
	d.SourceInWorkspace = func(workspace *throwaway.Workspace) (observe.Source, func() error, error) {
		w = workspace
		return ReleasedSource{}, func() error { return nil }, nil
	}
	pid, _ := startSignalGroup(t)
	start, err := throwaway.ProcessStart(pid)
	if err != nil {
		os.Exit(91)
	}
	d.Collect = func(plan.Plan, observe.Source) ([]collect.Check, error) {
		if err := w.RecordGroup(throwaway.Process{PID: pid, StartTime: start}); err != nil {
			return nil, err
		}
		if _, err := fmt.Fprintf(os.Stdout, "ready %d %s\n", pid, w.Path()); err != nil {
			return nil, err
		}
		var status syscall.WaitStatus
		if _, err := syscall.Wait4(pid, &status, 0, nil); err != nil {
			return nil, err
		}
		if !status.Signaled() || status.Signal() != syscall.SIGKILL {
			return nil, fmt.Errorf("group not killed: %v", status)
		}
		return nil, nil
	}
	code, err := Run(o, d, io.Discard)
	if code != 70 || err == nil {
		os.Exit(92)
	}
	if _, err := os.Stat(w.Path()); !errors.Is(err, os.ErrNotExist) {
		os.Exit(93)
	}
	os.Exit(code)
}
func TestOSSignalsExit70(t *testing.T) {
	// KTD17, QH23, QE20: actual OS signals reach the funnel and wait for the group.
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT} {
		t.Run(fmt.Sprintf("Signal%d", sig), func(t *testing.T) {
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := r.Close(); err != nil {
					t.Error(err)
				}
			}()
			helperEnv := signalHelperEnvironment(t)
			pid, err := syscall.ForkExec(exe, []string{exe, "-test.run=^TestInterruptedRunHelper$"}, &syscall.ProcAttr{Env: append(helperEnv, "PQL_INTERRUPTED_HELPER=1"), Files: []uintptr{0, w.Fd(), 2}})
			if closeErr := w.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
			if err != nil {
				t.Fatal(err)
			}
			reaped := false
			group := 0
			defer func() {
				if group > 0 {
					if err := syscall.Kill(-group, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
						t.Error(err)
					}
				}
				if !reaped {
					if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
						t.Error(err)
					}
					var status syscall.WaitStatus
					if _, err := syscall.Wait4(pid, &status, 0, nil); err != nil {
						t.Error(err)
					}
				}
			}()
			scanner := bufio.NewScanner(r)
			if !scanner.Scan() {
				t.Fatal("missing ready control", scanner.Err())
			}
			fields := strings.SplitN(scanner.Text(), " ", 3)
			if len(fields) != 3 || fields[0] != "ready" {
				t.Fatalf("ready control %q", scanner.Text())
			}
			group, err = strconv.Atoi(fields[1])
			if err != nil {
				t.Fatal(err)
			}
			if _, err := throwaway.ProcessStart(group); err != nil {
				t.Fatal("group not live", err)
			}
			if err := syscall.Kill(pid, sig); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(10 * time.Second)
			for {
				var status syscall.WaitStatus
				got, err := syscall.Wait4(pid, &status, syscall.WNOHANG, nil)
				if err != nil {
					t.Fatal(err)
				}
				if got == pid {
					reaped = true
					if !status.Exited() || status.ExitStatus() != 70 {
						t.Fatalf("signal %d: %v", sig, status)
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("interrupted run did not finish")
				}
				time.Sleep(time.Millisecond)
			}
			if _, err := os.Stat(fields[2]); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("workspace remains: %v", err)
			}
			if _, err := throwaway.ProcessStart(group); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("group not reaped: %v", err)
			}
			group = 0
		})
	}
}

func TestCleanupSignalHelper(t *testing.T) {
	mode := os.Getenv("PQL_CLEANUP_SIGNAL_HELPER")
	if mode == "" {
		return
	}
	// QE16, QE20, KTD17: subprocess permits testing immediate os.Exit.
	if mode == "Ignored" {
		signal.Ignore(syscall.SIGHUP)
	}
	o, d, _ := fixtureAt(t, os.Getenv("PQL_SIGNAL_FIXTURE"))
	if filepath.Dir(o.SubjectFile) != os.Getenv("PQL_SIGNAL_FIXTURE") {
		t.Fatal("helper fixture escaped parent directory")
	}
	d.SourceInWorkspace = func(w *throwaway.Workspace) (observe.Source, func() error, error) {
		return ReleasedSource{}, func() error {
			if mode == "Second" {
				if _, err := fmt.Fprintln(os.Stdout, "cleanup "+w.Path()); err != nil {
					return err
				}
				time.Sleep(30 * time.Second)
			}
			return nil
		}, nil
	}
	d.Collect = func(p plan.Plan, s observe.Source) ([]collect.Check, error) {
		if _, err := fmt.Fprintln(os.Stdout, "ready"); err != nil {
			return nil, err
		}
		time.Sleep(200 * time.Millisecond)
		return collect.Collect(p, s, collect.Options{RunDirectory: t.TempDir()})
	}
	code, runErr := Run(o, d, io.Discard)
	if runErr != nil {
		os.Exit(91)
	}
	os.Exit(code)
}

func TestIgnoredAndSecondSignals(t *testing.T) {
	// QE16, QE20, KTD17: inherited ignores persist; a second signal stops cleanup.
	for _, mode := range []string{"Ignored", "Second"} {
		t.Run(mode, func(t *testing.T) {
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := r.Close(); err != nil {
					t.Error(err)
				}
			}()
			helperEnv := signalHelperEnvironment(t)
			pid, err := syscall.ForkExec(exe, []string{exe, "-test.run=^TestCleanupSignalHelper$"}, &syscall.ProcAttr{Env: append(helperEnv, "PQL_CLEANUP_SIGNAL_HELPER="+mode), Files: []uintptr{0, w.Fd(), 2}})
			if closeErr := w.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
			if err != nil {
				t.Fatal(err)
			}
			reaped := false
			defer func() {
				if !reaped {
					if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
						t.Error(err)
					}
					var status syscall.WaitStatus
					if _, err := syscall.Wait4(pid, &status, 0, nil); err != nil {
						t.Error(err)
					}
				}
			}()
			scanner := bufio.NewScanner(r)
			if !scanner.Scan() || scanner.Text() != "ready" {
				t.Fatal("missing readiness", scanner.Err())
			}
			sig := syscall.SIGHUP
			if mode == "Second" {
				sig = syscall.SIGTERM
			}
			if err := syscall.Kill(pid, sig); err != nil {
				t.Fatal(err)
			}
			leftBehind := ""
			if mode == "Second" {
				if !scanner.Scan() || !strings.HasPrefix(scanner.Text(), "cleanup ") {
					t.Fatal("cleanup not entered", scanner.Err())
				}
				leftBehind = strings.TrimPrefix(scanner.Text(), "cleanup ")
				if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
					t.Fatal(err)
				}
			}
			deadline := time.Now().Add(time.Second)
			for {
				var status syscall.WaitStatus
				got, err := syscall.Wait4(pid, &status, syscall.WNOHANG, nil)
				if err != nil {
					t.Fatal(err)
				}
				if got == pid {
					reaped = true
					want := 4
					if mode == "Second" {
						want = 70
					}
					if !status.Exited() || status.ExitStatus() != want {
						t.Fatalf("exit: %v want %d", status, want)
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("signal handling did not finish")
				}
				time.Sleep(time.Millisecond)
			}
			if leftBehind != "" {
				if _, err := os.Stat(leftBehind); err != nil {
					t.Fatal("second signal removed workspace", err)
				}
			}
		})
	}
}

func signalHelperEnvironment(t *testing.T) []string {
	t.Helper()
	// KTD17, QE20: the parent owns directories even when the helper calls os.Exit.
	parent := t.TempDir()
	fixtureDir := filepath.Join(parent, "fixture")
	if err := os.Mkdir(fixtureDir, 0700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		entries, err := os.ReadDir(parent)
		if err != nil {
			t.Error(err)
			return
		}
		if len(entries) != 1 || entries[0].Name() != "fixture" {
			t.Errorf("helper left temporary directories: %v", entries)
		}
		if err := os.RemoveAll(parent); err != nil {
			t.Error(err)
		}
		if _, err := os.Stat(parent); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("helper parent remains: %v", err)
		}
	})
	return append(os.Environ(), "TMPDIR="+parent, "PQL_SIGNAL_FIXTURE="+fixtureDir)
}

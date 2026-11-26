package grade

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
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
	if os.Getenv("PQL_INTERRUPTED_HELPER") != "1" {
		return
	}
	o, d, _ := fixture(t)
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
			pid, err := syscall.ForkExec(exe, []string{exe, "-test.run=^TestInterruptedRunHelper$"}, &syscall.ProcAttr{Env: append(os.Environ(), "PQL_INTERRUPTED_HELPER=1", "TMPDIR="+t.TempDir()), Files: []uintptr{0, w.Fd(), 2}})
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

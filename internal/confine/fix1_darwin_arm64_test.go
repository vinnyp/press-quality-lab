package confine

import (
	"debug/macho"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/vinnyp/press-quality-lab/internal/throwaway"
	"golang.org/x/sys/unix"
)

func TestBudgetSampleFailureKills(t *testing.T) {
	// QO4, QH21: a failed sample is an error, never permission to leave a leader live.
	for _, budget := range []bool{false, true} {
		name := "Kill"
		if budget {
			name = "Budget"
		}
		t.Run(name, func(t *testing.T) {
			p := shellProcess(t, "sleep 0.3")
			first, second := errors.New("first sample"), errors.New("second sample")
			calls := 0
			p.sampleLeader = func(int) (*unix.KinfoProc, error) {
				calls++
				if calls == 1 {
					return nil, first
				}
				return nil, second
			}
			var killErr error
			if budget {
				p.expireBudget()
			} else {
				killErr = p.Kill()
			}
			r, waitErr := p.Wait()
			err := errors.Join(killErr, waitErr)
			if !errors.Is(err, first) || budget && !errors.Is(err, second) {
				t.Fatalf("lost sample error: %v", err)
			}
			if !r.End.EngineKilled || r.End.Signal != 9 || r.End.BudgetExpired != budget || r.End.RunningAtBudget {
				t.Fatalf("end=%+v", r.End)
			}
		})
	}
}

func TestPollRetriesEINTR(t *testing.T) {
	// QE10: interruption must not turn an otherwise readable drain into an error.
	calls := 0
	if err := retryPoll(func() error {
		calls++
		if calls < 3 {
			return unix.EINTR
		}
		return nil
	}); err != nil || calls != 3 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestSelfTestInvocation(t *testing.T) {
	// QH11, QH12, QH24: owned home, absent computation PLANTED, isolated environment and fixed bound.
	w, err := throwaway.BeginAt(filepath.Join(t.TempDir(), "runs"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := w.Remove(); err != nil {
			t.Error(err)
		}
	}()
	files, err := w.PrepareSelfTest()
	if err != nil {
		t.Fatal(err)
	}
	for _, computation := range []bool{false, true} {
		inv := selfTestInvocation(Invocation{Computation: computation, Env: []string{"FOREIGN=value"}, Paths: Paths{Values: map[string]string{}}}, files, w)
		if inv.Env != nil || inv.Budget != 10*time.Second || inv.Directory != files.Home {
			t.Fatalf("invocation=%+v", inv)
		}
		_, err := os.Lstat(inv.Paths.Values["PLANTED"])
		if computation && !os.IsNotExist(err) || !computation && err != nil {
			t.Fatalf("PLANTED computation=%t err=%v", computation, err)
		}
	}
}

func TestStartArmsBudget(t *testing.T) {
	// QO4: the public start path arms a positive invocation budget.
	c, err := NewCopies(t.TempDir(), "/usr/bin/true", "/usr/bin/true")
	if err != nil {
		t.Fatal(err)
	}
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	for _, name := range commonNames {
		values[name] = home
	}
	p, err := c.startWith(Invocation{Workspace: fakeWorkspace{}, Paths: Paths{Values: values}, Budget: time.Hour}, func(string, []string, string, []string, string) (Process, error) {
		return &process{changed: true, pid: os.Getpid()}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	real, ok := p.(*process)
	if !ok {
		t.Fatal("wrong process")
	}
	if real.budgetTimer == nil {
		t.Fatal("budget not armed")
	}
	real.budgetTimer.Stop()
}

func TestSelfTestProcessPIDOne(t *testing.T) {
	// QH9: process-route probes refuse the launcher's own pid sentinel.
	for _, name := range []string{"arguments", "environment", "process", "resource", "signal", "codesign", "priority"} {
		t.Run(name, func(t *testing.T) {
			if code := SelfTestHidden(name, "", "1"); code != 64 {
				t.Fatalf("code=%d", code)
			}
		})
	}
}

func TestDrainEscapedWriters(t *testing.T) {
	// QE10, QH19: live session-escaped writers cannot extend the post-exit drain.
	// Each owned writer has a three-second alarm even if the drain regresses.
	p := shellProcess(t, `for i in 1 2 3 4; do /usr/bin/perl -MPOSIX -e 'setsid(); alarm 3; $SIG{PIPE}="IGNORE"; while(1){syswrite STDOUT,"x" x 32768;}' & done; sleep 0.1`)
	start := time.Now()
	r, err := p.Wait()
	elapsed := time.Since(start)
	// Let the bounded writers terminate before the test returns.
	if remaining := 3*time.Second - time.Since(start); remaining > 0 {
		time.Sleep(remaining + 100*time.Millisecond)
	}
	if err != nil || !r.OutputCutAfterExit || elapsed >= time.Second {
		t.Fatalf("elapsed=%s cut=%t err=%v", elapsed, r.OutputCutAfterExit, err)
	}
}

func TestServiceImportLibraries(t *testing.T) {
	// QH9: the native witness uses libSystem's re-export without an extra load command.
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	file, err := macho.Open(executable)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	}()
	libraries, err := file.ImportedLibraries()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, library := range libraries {
		if library == "/usr/lib/libproc.dylib" {
			t.Fatal("extra libproc load command")
		}
		if library == "/usr/lib/libSystem.B.dylib" {
			found = true
		}
	}
	if !found {
		t.Fatal("libSystem load command missing")
	}
}

func TestSysctlSignedResult(t *testing.T) {
	// QH9: native failure diagnostics preserve the signed C return value.
	err := selfTestSysctl([]int32{2147483647})
	if err == nil || !strings.HasPrefix(err.Error(), "sysctl result=-1 ") {
		t.Fatalf("diagnostic=%v", err)
	}
}

func TestNativeSelfTestHelper(t *testing.T) {
	// QH9: success errno zero is not an error interface containing Errno(0).
	name := os.Getenv("PQL_NATIVE_SELFTEST_HELPER")
	if name == "" {
		return
	}
	os.Exit(SelfTestHidden(name, "", strconv.Itoa(os.Getpid())))
}

func TestNativeSelfTestSuccessErrno(t *testing.T) {
	// QH9: the unconfined native operation must report its actual result.
	for _, name := range []string{"resource", "codesign"} {
		t.Run(name, func(t *testing.T) {
			out, err := os.Create(filepath.Join(t.TempDir(), "output"))
			if err != nil {
				t.Fatal(err)
			}
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			p, err := os.StartProcess(executable, []string{executable, "-test.run=^TestNativeSelfTestHelper$"}, &os.ProcAttr{Env: []string{"PQL_NATIVE_SELFTEST_HELPER=" + name}, Files: []*os.File{nil, out, out}})
			if err != nil {
				t.Fatal(err)
			}
			state, waitErr := p.Wait()
			if err := out.Close(); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(out.Name())
			if err != nil {
				t.Fatal(err)
			}
			if waitErr != nil || state.ExitCode() != 1 || string(b) != "attempt_"+name+"\nreturned <nil>\n" {
				t.Fatalf("status=%v err=%v output=%q", state, waitErr, b)
			}
		})
	}
}

func TestBudgetLeaderEndsBetweenSamples(t *testing.T) {
	// QH21, QO4: the second sample can observe the confinement kill after a live first sample.
	p := shellProcess(t, "printf 'pql-launch:test\\n' >&2; sleep 0.05; kill -9 $$")
	calls := 0
	p.sampleLeader = func(pid int) (*unix.KinfoProc, error) {
		calls++
		if calls > 1 {
			waitZombie(t, pid)
		}
		proc, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
		// Make the earlier live sample deterministic even on a delayed host.
		if err == nil && calls == 1 {
			proc.Proc.P_stat = 2
		}
		return proc, err
	}
	p.expireBudget()
	r, err := p.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if r.End.BudgetExpired || r.End.RunningAtBudget || r.End.EngineKilled || r.End.Signal != 9 {
		t.Fatalf("end=%+v", r.End)
	}
}

func TestDecoyLaunchLifetime(t *testing.T) {
	// QH9, QE20: the launcher passes only the read end to the owned helper.
	w, err := throwaway.BeginAt(filepath.Join(t.TempDir(), "runs"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := w.Remove(); err != nil {
			t.Error(err)
		}
	}()
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(home, "decoy")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf 'pql-dec:\\001\\001\\001\\001'\n/bin/cat <&3 >/dev/null\n"), 0700); err != nil {
		t.Fatal(err)
	}
	d, err := startSelfTestDecoy(script, home, w)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.lifetime.Close(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		var status syscall.WaitStatus
		_, err := syscall.Wait4(d.pid, &status, 0, nil)
		if err == nil && status != 0 {
			err = errors.New("decoy read end missing")
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(time.Second):
		if err := unix.Kill(-d.pid, unix.SIGKILL); err != nil {
			t.Error(err)
		}
		if err := <-done; err == nil {
			t.Error("killed decoy succeeded")
		}
		t.Error("decoy inherited a writer")
	}
	if err := w.ForgetGroup(d.pid); err != nil {
		t.Fatal(err)
	}
}

func TestDrainPollRetriesEINTR(t *testing.T) {
	// QE10: the actual expired-deadline path retries an interrupted poll.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := w.Close(); err != nil {
			t.Error(err)
		}
	}()
	calls := 0
	c := drainLimitWithPoll(r, 1024, func([]unix.PollFd, int) (int, error) {
		calls++
		if calls == 1 {
			return 0, unix.EINTR
		}
		return 0, nil
	})
	if err := c.setDeadline(time.Now()); err != nil {
		t.Fatal(err)
	}
	<-c.done
	if calls != 2 || c.err != nil || !c.cutAfterExit {
		t.Fatalf("calls=%d err=%v cut=%t", calls, c.err, c.cutAfterExit)
	}
}

func TestSampleAndKillErrorsJoined(t *testing.T) {
	// QH21: a failed observation does not suppress a group kill or either diagnostic.
	for _, budget := range []bool{false, true} {
		name := "Kill"
		if budget {
			name = "Budget"
		}
		t.Run(name, func(t *testing.T) {
			sampleErr, killErr := errors.New("sample failed"), errors.New("signal failed")
			p := shellProcess(t, "sleep 0.3")
			p.sampleLeader = func(int) (*unix.KinfoProc, error) { return nil, sampleErr }
			p.signalGroup = func(pid int) (bool, error) { sent, err := killGroup(pid); return sent, errors.Join(err, killErr) }
			var err error
			if budget {
				p.expireBudget()
			} else {
				err = p.Kill()
			}
			r, waitErr := p.Wait()
			err = errors.Join(err, waitErr)
			if !errors.Is(err, sampleErr) || !errors.Is(err, killErr) || !r.End.EngineKilled || r.End.Signal != 9 {
				t.Fatalf("end=%+v err=%v", r.End, err)
			}
		})
	}
}

func TestDrainDeadlineRequiresHUP(t *testing.T) {
	// QE10, QH19: queued bytes drain after the deadline only once every writer closed.
	for _, closed := range []bool{false, true} {
		name := "LiveWriter"
		if closed {
			name = "ClosedWriter"
		}
		t.Run(name, func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			if !closed {
				defer func() {
					if err := w.Close(); err != nil {
						t.Error(err)
					}
				}()
			}
			payload := "queued bytes"
			if _, err := w.WriteString(payload); err != nil {
				t.Fatal(err)
			}
			if closed {
				if err := w.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if err := r.SetReadDeadline(time.Now()); err != nil {
				t.Fatal(err)
			}
			c := drainLimitWithPoll(r, 1024, unix.Poll)
			<-c.done
			wantSeen, wantHead := int64(0), ""
			if closed {
				wantSeen, wantHead = int64(len(payload)), payload
			}
			if c.err != nil || c.seen != wantSeen || string(c.head) != wantHead || c.cutAfterExit != !closed {
				t.Fatalf("seen=%d head=%q cut=%t err=%v", c.seen, c.head, c.cutAfterExit, c.err)
			}
		})
	}
}

func TestControlSuiteErrorDiscardsFacts(t *testing.T) {
	// QH9, QH24, QH26: infrastructure errors invalidate every control fact.
	facts, err := RunControlSuite(nil, nil, Invocation{}, Invocation{})
	if err == nil || facts.Error != err || len(facts.Subject) != 0 || len(facts.Computation) != 0 || len(facts.Fast) != 0 || !facts.Before.IsZero() || !facts.After.IsZero() {
		t.Fatalf("facts=%+v err=%v", facts, err)
	}
}

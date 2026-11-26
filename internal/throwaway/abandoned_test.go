package throwaway

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
)

func TestAbandonedHelper(t *testing.T) {
	if os.Getenv("PQL_ABANDONED_HELPER") != "1" {
		return
	}
	// QE20, QH23: only a test-owned child process is started, without confinement.
	if os.Getenv("PQL_START_MEMBER") == "1" {
		exe, err := os.Executable()
		if err != nil {
			os.Exit(80)
		}
		env := []string{"PQL_ABANDONED_HELPER=1"}
		pid, err := syscall.ForkExec(exe, []string{exe, "-test.run=^TestAbandonedHelper$"}, &syscall.ProcAttr{Env: env, Files: []uintptr{0, 1, 2}})
		if err != nil {
			os.Exit(81)
		}
		if _, err := fmt.Fprintf(os.Stdout, "member %d\n", pid); err != nil {
			os.Exit(82)
		}
	} else {
		if _, err := fmt.Fprintln(os.Stdout, "ready"); err != nil {
			os.Exit(83)
		}
	}
	time.Sleep(30 * time.Second)
	os.Exit(0)
}

func abandonedChild(t *testing.T, member bool) (int, *os.File) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "PQL_ABANDONED_HELPER=1")
	if member {
		env = append(env, "PQL_START_MEMBER=1")
	}
	pid, err := syscall.ForkExec(exe, []string{exe, "-test.run=^TestAbandonedHelper$"}, &syscall.ProcAttr{Env: env, Files: []uintptr{0, w.Fd(), 2}, Sys: &syscall.SysProcAttr{Setpgid: true}})
	if closeErr := w.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	})
	return pid, r
}
func killReap(t *testing.T, pid int) {
	t.Helper()
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		t.Error(err)
	}
	var status syscall.WaitStatus
	if _, err := syscall.Wait4(pid, &status, 0, nil); err != nil {
		t.Error(err)
	}
}
func TestSweepAfterKilledRun(t *testing.T) {
	// QE20, QH23, KTD5: real recorded leader, killed engine, group ends first.
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	engine, engineOut := abandonedChild(t, false)
	engineReaped := false
	t.Cleanup(func() {
		if !engineReaped {
			killReap(t, engine)
		}
	})
	var ready [6]byte
	if _, err := io.ReadFull(engineOut, ready[:]); err != nil || string(ready[:]) != "ready\n" {
		t.Fatalf("engine control %q %v", ready, err)
	}
	group, groupOut := abandonedChild(t, false)
	groupReaped := false
	t.Cleanup(func() {
		if !groupReaped {
			killReap(t, group)
		}
	})
	if _, err := io.ReadFull(groupOut, ready[:]); err != nil || string(ready[:]) != "ready\n" {
		t.Fatalf("group control %q %v", ready, err)
	}
	engineStart, err := ProcessStart(engine)
	if err != nil {
		t.Fatal(err)
	}
	groupStart, err := ProcessStart(group)
	if err != nil {
		t.Fatal(err)
	}
	w, err := Create(parent)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Publish(Process{PID: engine, StartTime: engineStart}); err != nil {
		t.Fatal(err)
	}
	if err := w.RecordGroup(Process{PID: group, StartTime: groupStart}); err != nil {
		t.Fatal(err)
	}
	// Positive control: a live owner keeps this exact published fixture intact.
	if err := Sweep(parent, ProcessStart); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(w.Path()); err != nil {
		t.Fatal(err)
	}
	killReap(t, engine)
	engineReaped = true
	if err := Sweep(parent, ProcessStart, SweepOptions{End: func(pid int) error {
		if pid != group {
			return fmt.Errorf("wrong group %d", pid)
		}
		if _, err := os.Stat(w.Path()); err != nil {
			return fmt.Errorf("removed before end: %w", err)
		}
		if err := EndGroup(pid); err != nil {
			return err
		}
		var status syscall.WaitStatus
		if _, err := syscall.Wait4(pid, &status, 0, nil); err != nil {
			return err
		}
		groupReaped = true
		if !status.Signaled() || status.Signal() != syscall.SIGKILL {
			return fmt.Errorf("group status %v", status)
		}
		return nil
	}}); err != nil {
		t.Fatal(err)
	}
	if !groupReaped {
		t.Fatal("group was not ended")
	}
	if _, err := os.Stat(w.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("workspace remains %v", err)
	}
}
func TestLeaderGoneMemberIsNotSignalled(t *testing.T) {
	// KTD5, QE20, QH23: an unverifiable computation group is reported, never killed.
	leader, out := abandonedChild(t, true)
	leaderReaped := false
	t.Cleanup(func() {
		if !leaderReaped {
			killReap(t, leader)
		}
	})
	scanner := bufio.NewScanner(out)
	member := 0
	ready := false
	for i := 0; i < 2; i++ {
		if !scanner.Scan() {
			t.Fatal("missing child control", scanner.Err())
		}
		words := strings.Fields(scanner.Text())
		if len(words) == 1 && words[0] == "ready" {
			ready = true
		} else if len(words) == 2 && words[0] == "member" {
			var err error
			member, err = strconv.Atoi(words[1])
			if err != nil {
				t.Fatal(err)
			}
		} else {
			t.Fatalf("unexpected control %q", scanner.Text())
		}
	}
	if member <= 0 || !ready {
		t.Fatal("member did not start")
	}
	t.Cleanup(func() {
		if err := syscall.Kill(member, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			t.Error(err)
		}
	})
	start, err := ProcessStart(leader)
	if err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	w, err := Create(parent)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Publish(Process{PID: os.Getpid(), StartTime: "stale"}); err != nil {
		t.Fatal(err)
	}
	if err := w.RecordGroup(Process{PID: leader, StartTime: start}); err != nil {
		t.Fatal(err)
	}
	// Positive control: matching leader exists and its group contains the member.
	actual, err := ProcessStart(leader)
	if err != nil || actual != start {
		t.Fatal(actual, err)
	}
	if err := syscall.Kill(member, 0); err != nil {
		t.Fatal(err)
	}
	killReap(t, leader)
	leaderReaped = true
	var report strings.Builder
	if err := Sweep(parent, ProcessStart, SweepOptions{Report: &report, End: func(int) error { t.Fatal("unverifiable group signalled"); return nil }}); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("cannot verify recorded process group %d; not signalled\n", leader)
	if report.String() != want {
		t.Fatalf("report %q want %q", report.String(), want)
	}
	if err := syscall.Kill(member, 0); err != nil {
		t.Fatal("surviving member was killed", err)
	}
	assertMemberExecuting(t, member)
	if _, err := os.Stat(w.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("home not removed: %v", err)
	}
}

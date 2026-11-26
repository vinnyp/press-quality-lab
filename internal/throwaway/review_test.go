package throwaway

import (
	"bufio"
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
)

func TestPublicationBoundDoesNotWedge(t *testing.T) {
	// QE20: failed publication cannot prevent subsequent group recording.
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	w, err := Create(parent)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Publish(Process{PID: 41, StartTime: "owner"}); err != nil {
		t.Fatal(err)
	}
	if err := w.RecordGroup(Process{PID: 42, StartTime: strings.Repeat("x", 4096)}); err == nil {
		t.Fatal("oversize publication accepted")
	}
	if _, err := os.Lstat(filepath.Join(w.Path(), ".process-pending")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pending remains: %v", err)
	}
	if err := w.RecordGroup(Process{PID: 42, StartTime: "group"}); err != nil {
		t.Fatal("failed record wedged publication", err)
	}
}

func TestSweepContinuesAfterEndFailure(t *testing.T) {
	// QE20, E-19: every stale candidate is attempted and failures are escaped.
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	var paths []string
	for i := 0; i < 2; i++ {
		w, err := Create(parent)
		if err != nil {
			t.Fatal(err)
		}
		if err := w.Publish(Process{PID: 41, StartTime: "owner"}); err != nil {
			t.Fatal(err)
		}
		if err := w.RecordGroup(Process{PID: 42, StartTime: "group"}); err != nil {
			t.Fatal(err)
		}
		renamed := w.Path() + "\n"
		if err := os.Rename(w.Path(), renamed); err != nil {
			t.Fatal(err)
		}
		w.path = renamed
		paths = append(paths, w.Path())
	}
	var report bytes.Buffer
	calls := 0
	err := Sweep(parent, func(pid int) (string, error) {
		if pid == 41 {
			return "", os.ErrNotExist
		}
		return "group", nil
	}, SweepOptions{Report: &report, End: func(int) error { calls++; return errors.New("cannot end") }})
	if err != nil || calls != 2 {
		t.Fatalf("calls=%d error=%v", calls, err)
	}
	if strings.Count(report.String(), "\n") != 2 {
		t.Fatalf("unescaped report: %q", report.String())
	}
	for _, path := range paths {
		if _, err := os.Stat(filepath.Join(path, "process.json")); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(report.String(), fmt.Sprintf("%q", path)) {
			t.Fatalf("missing name: %q", report.String())
		}
	}
}

func TestGroupsGrammar(t *testing.T) {
	// QE20, KTD5: malformed groups never reach process lookup or EndGroup.
	rows := []struct{ name, groups string }{
		{"Control", `[{"pid":42,"start_time":"group"}]`},
		{"NotArray", `{}`},
		{"OneKey", `[{"pid":42}]`},
		{"ThreeKeys", `[{"pid":42,"start_time":"group","extra":true}]`},
		{"ZeroPID", `[{"pid":0,"start_time":"group"}]`},
		{"NegativePID", `[{"pid":-1,"start_time":"group"}]`},
		{"FractionPID", `[{"pid":1.5,"start_time":"group"}]`},
		{"MissingStart", `[{"pid":42,"other":"group"}]`},
		{"EmptyStart", `[{"pid":42,"start_time":""}]`},
		{"DuplicatePID", `[{"pid":42,"start_time":"group"},{"pid":42,"start_time":"group"}]`},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			parent := t.TempDir()
			if err := os.Chmod(parent, 0700); err != nil {
				t.Fatal(err)
			}
			w, err := Create(parent)
			if err != nil {
				t.Fatal(err)
			}
			data := []byte(`{"pid":41,"start_time":"owner","groups":` + row.groups + `}`)
			if err := os.WriteFile(filepath.Join(w.Path(), "process.json"), data, 0600); err != nil {
				t.Fatal(err)
			}
			lookups, ends := 0, 0
			err = Sweep(parent, func(pid int) (string, error) {
				lookups++
				if row.name != "Control" {
					t.Fatal("invalid groups reached lookup")
				}
				if pid == 41 {
					return "", os.ErrNotExist
				}
				return "group", nil
			}, SweepOptions{End: func(pid int) error {
				if pid != 42 {
					t.Fatalf("wrong group %d", pid)
				}
				ends++
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
			_, err = os.Stat(w.Path())
			if row.name == "Control" {
				if !errors.Is(err, os.ErrNotExist) || lookups != 2 || ends != 1 {
					t.Fatalf("control: %v %d %d", err, lookups, ends)
				}
			} else if err != nil || lookups != 0 || ends != 0 {
				t.Fatalf("negative: %v %d %d", err, lookups, ends)
			}
		})
	}
}

func TestBeginRealSweepAndPublication(t *testing.T) {
	// QE20, KTD5: BeginAt uses the real kernel lookup, sweep and publication.
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	group, out := abandonedChild(t, false)
	defer killReap(t, group)
	var ready [6]byte
	if _, err := io.ReadFull(out, ready[:]); err != nil || string(ready[:]) != "ready\n" {
		t.Fatalf("control: %q %v", ready, err)
	}
	start, err := ProcessStart(group)
	if err != nil {
		t.Fatal(err)
	}
	old, err := Create(parent)
	if err != nil {
		t.Fatal(err)
	}
	if err := old.Publish(Process{PID: os.Getpid(), StartTime: "stale"}); err != nil {
		t.Fatal(err)
	}
	if err := old.RecordGroup(Process{PID: group, StartTime: start}); err != nil {
		t.Fatal(err)
	}
	w, err := BeginAt(parent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old workspace remains: %v", err)
	}
	assertGroupEmpty(t, group)
	ownerStart, err := ProcessStart(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(w.Path(), "process.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("{\"pid\":%d,\"start_time\":%q}\n", os.Getpid(), ownerStart)
	if string(data) != want {
		t.Fatalf("publication: %s want %s", data, want)
	}
	if err := w.Remove(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := BeginAt(parent); err == nil {
		t.Fatal("wrong-mode parent accepted")
	}
}

func TestEndGroupWaitsForMember(t *testing.T) {
	// QH23: no Wait4 hides members before the kernel membership assertion.
	leader, out := abandonedChild(t, true)
	defer killReap(t, leader)
	scanner := bufio.NewScanner(out)
	member := 0
	ready := false
	for i := 0; i < 2; i++ {
		if !scanner.Scan() {
			t.Fatal("missing readiness", scanner.Err())
		}
		line := scanner.Text()
		if line == "ready" {
			ready = true
		} else if _, err := fmt.Sscanf(line, "member %d", &member); err != nil {
			t.Fatal(err)
		}
	}
	if !ready || member <= 0 {
		t.Fatal("member not ready")
	}
	defer func() {
		if err := syscall.Kill(member, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			t.Error(err)
		}
	}()
	assertMemberExecuting(t, member)
	if err := EndGroup(leader); err != nil {
		t.Fatal(err)
	}
	assertGroupEmpty(t, leader)
}

func TestProcessPublicationFIFO(t *testing.T) {
	// QE20, KTD20: neither cleanup nor sweep opens a process publication FIFO.
	for _, operation := range []string{"Remove", "Sweep"} {
		for _, kind := range []string{"FIFO", "Link"} {
			t.Run(operation+kind, func(t *testing.T) {
				parent := t.TempDir()
				if err := os.Chmod(parent, 0700); err != nil {
					t.Fatal(err)
				}
				w, err := Create(parent)
				if err != nil {
					t.Fatal(err)
				}
				fifo := filepath.Join(parent, "fifo")
				if err := syscall.Mkfifo(fifo, 0600); err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(w.Path(), "process.json")
				if kind == "Link" {
					err = os.Symlink(fifo, target)
				} else {
					err = os.Link(fifo, target)
				}
				if err != nil {
					t.Fatal(err)
				}
				opened := make(chan error, 1)
				go func() {
					var fd int
					var err error
					for {
						fd, err = syscall.Open(fifo, syscall.O_WRONLY, 0)
						if !errors.Is(err, syscall.EINTR) {
							break
						}
					}
					if err == nil {
						err = syscall.Close(fd)
					}
					opened <- err
				}()
				defer func() {
					fd, err := syscall.Open(fifo, syscall.O_RDONLY|syscall.O_NONBLOCK, 0)
					if err != nil {
						t.Error(err)
						return
					}
					defer func() {
						if err := syscall.Close(fd); err != nil {
							t.Error(err)
						}
					}()
					select {
					case err := <-opened:
						if err != nil {
							t.Error(err)
						}
					case <-time.After(time.Second):
						t.Error("writer not released by control")
					}
				}()
				select {
				case err := <-opened:
					t.Fatalf("writer not blocked: %v", err)
				case <-time.After(20 * time.Millisecond):
				}
				if operation == "Remove" {
					err = w.Remove()
				} else {
					err = Sweep(parent, func(int) (string, error) { t.Error("FIFO reached lookup"); return "", os.ErrNotExist })
				}
				if err != nil {
					t.Fatal(err)
				}
				select {
				case err := <-opened:
					opened <- err
					t.Fatal("publication opened FIFO")
				case <-time.After(20 * time.Millisecond):
				}
			})
		}
	}
}

func TestPublicationRenameFailureDoesNotWedge(t *testing.T) {
	// QE20: all errors after creating the pending file remove it.
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	w, err := Create(parent)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(w.Path(), "process.json")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	if err := w.Publish(Process{PID: 41, StartTime: "owner"}); err == nil {
		t.Fatal("rename over directory succeeded")
	}
	if _, err := os.Lstat(filepath.Join(w.Path(), ".process-pending")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pending remains: %v", err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := w.Publish(Process{PID: 41, StartTime: "owner"}); err != nil {
		t.Fatal("rename failure wedged next publication", err)
	}
}

func TestSweepContinuesAfterRemoveFailure(t *testing.T) {
	// QE20, E-19: an unremovable stale home does not stop the next candidate.
	if os.Geteuid() == 0 {
		t.Fatal("permission fixture requires non-root user")
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(parent, 0700); err != nil {
			t.Error(err)
		}
	})
	var paths []string
	for i := 0; i < 2; i++ {
		w, err := Create(parent)
		if err != nil {
			t.Fatal(err)
		}
		if err := w.Publish(Process{PID: 41, StartTime: "owner"}); err != nil {
			t.Fatal(err)
		}
		if err := w.RecordGroup(Process{PID: 42, StartTime: "group"}); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, w.Path())
	}
	calls := 0
	var report bytes.Buffer
	err := Sweep(parent, func(pid int) (string, error) {
		if pid == 41 {
			return "", os.ErrNotExist
		}
		return "group", nil
	}, SweepOptions{Report: &report, End: func(int) error {
		calls++
		mode := os.FileMode(0500)
		if calls == 2 {
			mode = 0700
		}
		return os.Chmod(parent, mode)
	}})
	if err != nil || calls != 2 {
		t.Fatalf("sweep: calls=%d err=%v", calls, err)
	}
	kept, gone := 0, 0
	for _, path := range paths {
		_, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			gone++
		} else if err == nil {
			kept++
			if !strings.Contains(report.String(), fmt.Sprintf("%q", path)) {
				t.Fatalf("unnamed: %q", report.String())
			}
		} else {
			t.Fatal(err)
		}
	}
	if kept != 1 || gone != 1 {
		t.Fatalf("kept=%d gone=%d", kept, gone)
	}
	if err := Sweep(parent, func(int) (string, error) { return "", os.ErrNotExist }); err != nil {
		t.Fatal(err)
	}
}

func TestProcessLookupExitRace(t *testing.T) {
	// QE20: ESRCH from a kernel process lookup is disappearance, not uncertainty.
	uncertain := errors.New("permission uncertainty")
	if got := processLookupError(uncertain); got != uncertain {
		t.Fatalf("uncertainty lost: %v", got)
	}
	if got := processLookupError(nil); got != nil {
		t.Fatalf("success changed: %v", got)
	}
	if got := processLookupError(fmt.Errorf("stat: %w", syscall.ESRCH)); !errors.Is(got, os.ErrNotExist) {
		t.Fatalf("exit race remains uncertain: %v", got)
	}
}

func TestSweepUncertainGroupFails(t *testing.T) {
	// QE20, E-19: uncertain group identity fails the new run and keeps its home.
	for _, row := range []string{"Control", "Uncertain"} {
		t.Run(row, func(t *testing.T) {
			parent := t.TempDir()
			if err := os.Chmod(parent, 0700); err != nil {
				t.Fatal(err)
			}
			w, err := Create(parent)
			if err != nil {
				t.Fatal(err)
			}
			if err := w.Publish(Process{PID: 41, StartTime: "owner"}); err != nil {
				t.Fatal(err)
			}
			if err := w.RecordGroup(Process{PID: 42, StartTime: "group"}); err != nil {
				t.Fatal(err)
			}
			uncertain := errors.New("cannot determine group identity")
			ends := 0
			err = Sweep(parent, func(pid int) (string, error) {
				if pid == 41 {
					return "", os.ErrNotExist
				}
				if row == "Uncertain" {
					return "", uncertain
				}
				return "group", nil
			}, SweepOptions{Report: io.Discard, End: func(int) error { ends++; return nil }})
			_, statErr := os.Stat(w.Path())
			if row == "Control" {
				if err != nil || ends != 1 || !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("control: %v ends=%d stat=%v", err, ends, statErr)
				}
			} else {
				if !errors.Is(err, uncertain) || ends != 0 || statErr != nil {
					t.Fatalf("uncertainty: %v ends=%d stat=%v", err, ends, statErr)
				}
			}
		})
	}
}

type unavailableReport struct{}

func (unavailableReport) Write([]byte) (int, error) { return 0, os.ErrClosed }

func TestSweepReportFailureContinues(t *testing.T) {
	// QE20, E-19: only unsafe parents and uncertain lookups fail a new run.
	for _, report := range []io.Writer{io.Discard, unavailableReport{}} {
		parent := t.TempDir()
		if err := os.Chmod(parent, 0700); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			w, err := Create(parent)
			if err != nil {
				t.Fatal(err)
			}
			if err := w.Publish(Process{PID: 41, StartTime: "owner"}); err != nil {
				t.Fatal(err)
			}
			if err := w.RecordGroup(Process{PID: 42, StartTime: "group"}); err != nil {
				t.Fatal(err)
			}
		}
		calls := 0
		err := Sweep(parent, func(pid int) (string, error) {
			if pid == 41 {
				return "", os.ErrNotExist
			}
			return "group", nil
		}, SweepOptions{Report: report, End: func(int) error { calls++; return errors.New("end failed") }})
		if err != nil || calls != 2 {
			t.Fatalf("report failure aborted sweep: calls=%d error=%v", calls, err)
		}
	}
}

func TestPublicationSwapToFIFOLink(t *testing.T) {
	// QE20, KTD20: O_NOFOLLOW refuses links; O_NONBLOCK bounds a renamed FIFO.
	t.Run("RenameFIFO", func(t *testing.T) {
		w, err := Create(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(w.Path(), "process.json")
		if err := os.WriteFile(target, []byte(`{"pid":41,"start_time":"owner"}`), 0600); err != nil {
			t.Fatal(err)
		}
		fifo := filepath.Join(w.Path(), "fifo")
		if err := syscall.Mkfifo(fifo, 0600); err != nil {
			t.Fatal(err)
		}
		root, err := os.OpenRoot(w.Path())
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := root.Close(); err != nil {
				t.Error(err)
			}
		}()
		want := `{"pid":41,"start_time":"owner"}`
		data, err := readPublicationWith(root, nil)
		if err != nil || string(data) != want {
			t.Fatalf("regular publication control: %q %v", data, err)
		}
		done := make(chan error, 1)
		go func() {
			var swapErr error
			_, err := readPublicationWith(root, func() { swapErr = os.Rename(fifo, target) })
			done <- errors.Join(swapErr, err)
		}()
		select {
		case err := <-done:
			if !errors.Is(err, errNotRegular) {
				t.Fatalf("FIFO not refused: %v", err)
			}
		case <-time.After(time.Second):
			// Release a regressed blocking open before failing, leaving no goroutine.
			fd, err := syscall.Open(target, syscall.O_WRONLY|syscall.O_NONBLOCK, 0)
			if err != nil {
				t.Fatal(err)
			}
			if err := syscall.Close(fd); err != nil {
				t.Error(err)
			}
			if err := <-done; !errors.Is(err, errNotRegular) {
				t.Errorf("unblocked FIFO not refused: %v", err)
			}
			t.Fatal("renamed FIFO open blocked")
		}
	})
	w, err := Create(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(w.Path(), "process.json")
	if err := os.WriteFile(target, []byte(`{"pid":41,"start_time":"owner"}`), 0600); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(w.Path(), "fifo")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	opened := make(chan error, 1)
	go func() {
		var fd int
		var err error
		for {
			fd, err = syscall.Open(fifo, syscall.O_WRONLY, 0)
			if !errors.Is(err, syscall.EINTR) {
				break
			}
		}
		if err == nil {
			err = syscall.Close(fd)
		}
		opened <- err
	}()
	defer func() {
		fd, err := syscall.Open(fifo, syscall.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			t.Error(err)
			return
		}
		defer func() {
			if err := syscall.Close(fd); err != nil {
				t.Error(err)
			}
		}()
		select {
		case err := <-opened:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(time.Second):
			t.Error("writer control timed out")
		}
	}()
	select {
	case err := <-opened:
		opened <- err
		t.Fatal("writer not blocked")
	case <-time.After(20 * time.Millisecond):
	}
	root, err := os.OpenRoot(w.Path())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	}()
	called := false
	_, err = readPublicationWith(root, func() {
		called = true
		if err := os.Remove(target); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("fifo", target); err != nil {
			t.Fatal(err)
		}
	})
	if !called || err == nil {
		t.Fatalf("replacement not refused: %v %v", called, err)
	}
	select {
	case err := <-opened:
		opened <- err
		t.Fatal("reader followed replacement FIFO link")
	case <-time.After(20 * time.Millisecond):
	}
}

func TestPublicationDescriptorType(t *testing.T) {
	// QE20, KTD20: revalidate the descriptor after a successful nonblocking open.
	w, err := Create(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(w.Path(), "process.json")
	if err := os.WriteFile(target, []byte(`{"pid":41,"start_time":"owner"}`), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(w.Path())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	}()
	// Positive control uses the same bounded regular publication before replacement.
	data, err := readPublicationWith(root, nil)
	if err != nil || string(data) != `{"pid":41,"start_time":"owner"}` {
		t.Fatalf("control: %q %v", data, err)
	}
	data, err = readPublicationWith(root, func() {
		if err := os.Remove(target); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(target, 0700); err != nil {
			t.Fatal(err)
		}
	})
	if !errors.Is(err, errNotRegular) || len(data) != 0 {
		t.Fatalf("descriptor not refused before reading: %q %v", data, err)
	}
}

package confine

import (
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// These tests start unconfined programs only, never sandbox-exec.
func shellProcess(t *testing.T, command string) *process {
	t.Helper()
	p, err := spawn("/bin/sh", []string{"sh", "-c", command}, "", []string{"PATH=/usr/bin:/bin"}, "test")
	if err != nil {
		t.Fatal(err)
	}
	concrete, ok := p.(*process)
	if !ok {
		t.Fatal("spawn did not return process")
	}
	return concrete
}
func TestPipeCapture(t *testing.T) {
	p := shellProcess(t, "printf output; printf 'pql-launch:test\\ndiagnostic' >&2")
	run, err := p.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if run.End.Code != 0 || !run.Markers.Confinement || string(run.Stdout) != "output" || string(run.Stderr) != "diagnostic" || run.StdoutSeen != 6 || run.StderrSeen != 10 {
		t.Fatalf("pipe capture: %+v", run)
	}
	if err := p.Kill(); err != nil {
		t.Fatalf("Kill after reap: %v", err)
	}
	// Without the reaped guard, kern.proc.pid lookup after reap returns an error;
	// the Kill error assertion above catches that regression, not p.killed.
	if !p.reaped || p.killed {
		t.Fatal("Kill after reap sent a signal")
	}
}
func TestBoundedCapture(t *testing.T) {
	p := shellProcess(t, "head -c 3145728 /dev/zero; head -c 3145728 /dev/zero >&2")
	run, err := p.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Stdout) != 1048576 || len(run.Stderr) != 1048576 || run.StdoutSeen != 3145728 || run.StderrSeen != 3145728 || !run.StdoutCut || !run.StderrCut {
		t.Fatalf("bounded capture: out=%d/%d cut=%v err=%d/%d cut=%v", len(run.Stdout), run.StdoutSeen, run.StdoutCut, len(run.Stderr), run.StderrSeen, run.StderrCut)
	}
}
func TestEngineKill(t *testing.T) {
	p := shellProcess(t, "sleep 60")
	if err := p.Kill(); err != nil {
		t.Fatal(err)
	}
	run, err := p.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if !run.End.EngineKilled || run.End.Signal != int(syscall.SIGKILL) {
		t.Fatalf("engine kill: %+v", run.End)
	}
}
func TestGroupEnd(t *testing.T) {
	p := shellProcess(t, "sleep 60 & exit 0")
	t.Cleanup(func() {
		if err := unix.Kill(-p.pid, unix.SIGKILL); err != nil && err != unix.ESRCH && err != unix.EPERM {
			t.Error(err)
		}
	})
	start := time.Now()
	if _, err := p.Wait(); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("group ending exceeded bound")
	}
	procs, err := unix.SysctlKinfoProcSlice("kern.proc.pgrp", p.pid)
	if err != nil {
		t.Fatal(err)
	}
	for _, proc := range procs {
		if proc.Proc.P_stat != zombieState {
			t.Fatalf("live background member: %d", proc.Proc.P_pid)
		}
	}
}
func TestLateKillAttribution(t *testing.T) {
	p := shellProcess(t, "sleep 60 & exit 0")
	waitZombie(t, p.pid)
	if err := p.Kill(); err != nil {
		t.Fatal(err)
	}
	run, err := p.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if run.End.EngineKilled {
		t.Fatal("late kill attributed to engine")
	}
}
func waitZombie(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		proc, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
		if err != nil {
			t.Fatal(err)
		}
		if proc.Proc.P_stat == zombieState {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("process did not become zombie")
}
func TestRegistrationESRCH(t *testing.T) {
	queue, err := unix.Kqueue()
	if err != nil {
		t.Fatal(err)
	}
	unix.CloseOnExec(queue)
	out, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	pid, err := syscall.ForkExec("/usr/bin/true", []string{"true"}, &syscall.ProcAttr{Files: []uintptr{0, outW.Fd(), errW.Fd()}, Sys: &syscall.SysProcAttr{Setsid: true}})
	if err != nil {
		t.Fatal(err)
	}
	if err := outW.Close(); err != nil {
		t.Fatal(err)
	}
	if err := errW.Close(); err != nil {
		t.Fatal(err)
	}
	p := &process{pid: pid, queue: queue, stdout: drain(out), stderr: drain(stderr)}
	waitZombie(t, pid)
	if err := p.registerExit(); err != nil {
		t.Fatal(err)
	}
	if !p.registrationMissing {
		t.Fatal("zombie registration did not record ESRCH")
	}
	run, err := p.Wait()
	if err != nil || run.End.Code != 0 {
		t.Fatalf("ESRCH wait: %+v %v", run, err)
	}
}
func TestChangedCopies(t *testing.T) {
	for _, kind := range []string{"digest", "size", "mtime", "ctime", "mode", "inode", "launcher_entry", "launcher_bytes"} {
		t.Run(kind, func(t *testing.T) {
			c, err := NewCopies(t.TempDir(), "/bin/sh", "/bin/sh")
			if err != nil {
				t.Fatal(err)
			}
			target := c.program.path
			if strings.HasPrefix(kind, "launcher") {
				target = c.launcher.path
			}
			info, err := os.Stat(target)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "digest", "launcher_bytes":
				b, err := os.ReadFile(target)
				if err != nil {
					t.Fatal(err)
				}
				b[len(b)-1] ^= 1
				if err := os.WriteFile(target, b, info.Mode()); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(target, info.ModTime(), info.ModTime()); err != nil {
					t.Fatal(err)
				}
			case "size":
				b, err := os.ReadFile(target)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(target, append(b, 0), info.Mode()); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(target, info.ModTime(), info.ModTime()); err != nil {
					t.Fatal(err)
				}
			case "mtime":
				changed := info.ModTime().Add(-time.Hour)
				if err := os.Chtimes(target, changed, changed); err != nil {
					t.Fatal(err)
				}
			case "ctime":
				if err := os.Chmod(target, info.Mode()); err != nil {
					t.Fatal(err)
				}

			case "mode":
				if err := os.Chmod(target, 0600); err != nil {
					t.Fatal(err)
				}
			case "inode":
				b, err := os.ReadFile(target)
				if err != nil {
					t.Fatal(err)
				}
				replacement := filepath.Join(t.TempDir(), "replacement")
				if err := os.WriteFile(replacement, b, info.Mode()); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(replacement, target); err != nil {
					t.Fatal(err)
				}
			case "launcher_entry":
				if err := os.WriteFile(filepath.Join(filepath.Dir(target), "extra"), nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			// Isolate each fingerprint property: unrelated timestamps must not
			// make a missing fingerprint comparison appear guarded.
			if kind != "launcher_entry" {
				copy := &c.program
				if kind == "launcher_bytes" {
					copy = &c.launcher
				}
				original := copy.original
				current, err := fingerprintOf(target)
				if err != nil {
					t.Fatal(err)
				}
				copy.original = current
				switch kind {
				case "digest", "launcher_bytes":
					copy.original.Digest = original.Digest
				case "size":
					copy.original.Size = original.Size
				case "mtime":
					copy.original.MtimeSeconds, copy.original.MtimeNanos = original.MtimeSeconds, original.MtimeNanos
				case "ctime":
					copy.original.CtimeSeconds, copy.original.CtimeNanos = original.CtimeSeconds, original.CtimeNanos
				case "mode":
					copy.original.Mode = original.Mode
				case "inode":
					copy.original.Inode = original.Inode
				}
			}

			p, err := c.Start(Invocation{})
			if err != nil {
				t.Fatalf("changed copy reached argument building: %v", err)
			}
			run, err := p.Wait()
			if err != nil || !run.CopyChanged {
				t.Fatalf("changed copy accepted: %+v %v", run, err)
			}
		})
	}
}
func TestCanonicalCopies(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(parent, "link")
	target, err := filepath.Rel(parent, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cwd, err = filepath.EvalSymlinks(cwd)
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, link)
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewCopies(relative, "/bin/sh", "/bin/sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{c.launcher.path, c.program.path} {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || path != resolved || !filepath.IsAbs(path) {
			t.Fatalf("noncanonical copy: %s %s %v", path, resolved, err)
		}
	}
}
func TestArgumentSize(t *testing.T) {
	if err := argumentSize([]string{"abc"}, []string{"A=b"}, 24); err != nil {
		t.Fatal(err)
	}
	if err := argumentSize([]string{"abc"}, []string{"A=b"}, 23); err == nil {
		t.Fatal("oversized arguments accepted")
	}
}

func TestImmediateRegistration(t *testing.T) {
	p := shellProcess(t, "sleep 0.05; exit 0")
	waitZombie(t, p.pid)
	events := make([]unix.Kevent_t, 1)
	zero := unix.Timespec{}
	n, err := unix.Kevent(p.queue, nil, events, &zero)
	// The event is consumed here; Wait's ESRCH path still ends and reaps.
	p.registrationMissing = true
	_, waitErr := p.Wait()
	if waitErr != nil {
		t.Fatal(waitErr)
	}
	if err != nil || n != 1 || events[0].Filter != unix.EVFILT_PROC {
		t.Fatalf("exit event not registered at spawn: n=%d err=%v", n, err)
	}
}

type panicStream struct{}

func (panicStream) Read([]byte) (int, error) { panic("planted read panic") }
func (panicStream) Close() error             { return nil }
func TestDrainPanic(t *testing.T) {
	c := drain(panicStream{})
	<-c.done
	if c.err == nil || !strings.Contains(c.err.Error(), "stream drain panic: planted read panic") {
		t.Fatalf("drain panic not converted to engine error: %v", c.err)
	}
}
func TestNewSession(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "ready")
	p := shellProcess(t, fmt.Sprintf("if ( : </dev/tty ) 2>/dev/null; then printf tty_open; else printf tty_closed; fi; : > %q; sleep 60", ready))
	t.Cleanup(func() {
		if err := p.Kill(); err != nil {
			t.Error(err)
		}
		if _, err := p.Wait(); err != nil {
			t.Error(err)
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, err := os.Stat(ready)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatal("shell did not report its tty before the deadline")
		}
		time.Sleep(time.Millisecond)
	}
	sid, err := unix.Getsid(p.pid)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Kill(); err != nil {
		t.Fatal(err)
	}
	run, err := p.Wait()
	if string(run.Stdout) != "tty_closed" {
		t.Fatalf("controlling tty: %q", run.Stdout)
	}
	if err != nil {
		t.Fatal(err)
	}
	if sid != p.pid {
		t.Fatalf("session does not equal pid: sid=%d pid=%d", sid, p.pid)
	}
}

func TestGroupEndRepeats(t *testing.T) {
	signals, lookups := 0, 0
	err := endGroupWith(func() (bool, error) { signals++; return true, nil }, func() ([]groupMember, error) { lookups++; return []groupMember{{pid: 1, zombie: lookups >= 2}}, nil })
	if err != nil || signals != 2 || lookups != 2 {
		t.Fatalf("group kill did not repeat until zombie-only: signals=%d lookups=%d error=%v", signals, lookups, err)
	}
}

func TestGroupEndEPERM(t *testing.T) {
	calls := 0
	err := endGroupWith(func() (bool, error) {
		calls++
		if calls == 1 {
			return false, unix.EPERM
		}
		return true, nil
	}, func() ([]groupMember, error) { return []groupMember{{pid: 1, zombie: calls > 1}}, nil })
	if err != nil || calls != 2 {
		t.Fatalf("transient EPERM not retried: calls=%d err=%v", calls, err)
	}
}

func TestConsecutiveKill(t *testing.T) {
	p := shellProcess(t, "sleep 60")
	first, second := p.Kill(), p.Kill()
	_, waitErr := p.Wait()
	if first != nil || second != nil || waitErr != nil {
		t.Fatalf("consecutive kills: %v %v %v", first, second, waitErr)
	}
}

func TestConcurrentKillWait(t *testing.T) {
	p := shellProcess(t, "sleep 0.02")
	done := make(chan error, 1)
	go func() { _, err := p.Wait(); done <- err }()
	killErr := p.Kill()
	if err := <-done; err != nil || killErr != nil {
		t.Fatalf("concurrent Kill Wait: %v %v", killErr, err)
	}
}

func TestEscapedWriter(t *testing.T) {
	p := shellProcess(t, "set -m; sleep 2 & exit 0")
	start := time.Now()
	run, err := p.Wait()
	if err != nil || time.Since(start) > time.Second || !run.OutputCutAfterExit {
		t.Fatalf("escaped writer drain not bounded or recorded: elapsed=%v cut=%v error=%v", time.Since(start), run.OutputCutAfterExit, err)
	}
}

func TestWaitCached(t *testing.T) {
	p := shellProcess(t, "printf cached")
	first, err := p.Wait()
	if err != nil {
		t.Fatal(err)
	}
	// A closed queue must never be read again, even if its fd is reused.
	p.queue = -1
	second, err := p.Wait()
	if err != nil || string(second.Stdout) != "cached" || second.End != first.End {
		t.Fatalf("second Wait did not return cached result: %+v %v", second, err)
	}
}

func TestDecodeEvent(t *testing.T) {
	detail, err := decodeEvent(unix.Kevent_t{Fflags: 0x10000, Data: 0x20009})
	if err != nil || detail != 0x20000 {
		t.Fatalf("event data misrouted: %#x %v", detail, err)
	}
}

func TestStartEnvironment(t *testing.T) {
	c, err := NewCopies(t.TempDir(), "/usr/bin/true", "/usr/bin/true")
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	for _, name := range commonNames {
		values[name] = "/"
	}
	called := false
	_, err = c.startWith(Invocation{Paths: Paths{Values: values}, Env: []string{"SUBJECT=private"}}, func(_ string, _ []string, _ string, env []string, _ string) (Process, error) {
		called = true
		if len(env) != 0 {
			t.Fatalf("spawn inherited program environment: %q", env)
		}
		return &process{changed: true}, nil
	})
	if err != nil || !called {
		t.Fatalf("spawn seam not reached: %v", err)
	}
}

func TestCaptureBoundary(t *testing.T) {
	for _, tc := range []struct {
		name string
		size int
		cut  bool
	}{
		{"ExactCap", 1048576, false},
		{"OverCap", 1048577, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := shellProcess(t, fmt.Sprintf("head -c %d /dev/zero; head -c %d /dev/zero >&2", tc.size, tc.size))
			run, err := p.Wait()
			if err != nil {
				t.Fatal(err)
			}
			if run.End.Code != 0 || len(run.Stdout) != 1048576 || len(run.Stderr) != 1048576 || run.StdoutSeen != int64(tc.size) || run.StderrSeen != int64(tc.size) || run.StdoutCut != tc.cut || run.StderrCut != tc.cut {
				t.Fatalf("capture boundary: size=%d out=%d/%d cut=%v stderr=%d/%d cut=%v end=%+v", tc.size, len(run.Stdout), run.StdoutSeen, run.StdoutCut, len(run.Stderr), run.StderrSeen, run.StderrCut, run.End)
			}
		})
	}
}

func TestStartArgumentLimit(t *testing.T) {
	limit, err := unix.SysctlUint32("kern.argmax")
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewCopies(t.TempDir(), "/usr/bin/true", "/usr/bin/true")
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	for _, name := range commonNames {
		values[name] = "/"
	}
	called := false
	p, err := c.startWith(Invocation{Paths: Paths{Values: values}, Words: []string{strings.Repeat("a", int(limit)+1)}}, func(string, []string, string, []string, string) (Process, error) {
		called = true
		return &process{changed: true}, nil
	})
	if called || p != nil || err == nil || err.Error() != "program arguments and environment exceed kern.argmax" {
		t.Fatalf("argument limit did not fault before launch: called=%v process=%v error=%v", called, p, err)
	}
}

func TestNullStdin(t *testing.T) {
	saved, err := unix.Dup(0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unix.Dup2(saved, 0); err != nil {
			t.Error(err)
		}
		if err := unix.Close(saved); err != nil {
			t.Error(err)
		}
	})
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
		if err := w.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := unix.Dup2(int(r.Fd()), 0); err != nil {
		t.Fatal(err)
	}
	var input unix.Stat_t
	if err := unix.Fstat(0, &input); err != nil {
		t.Fatal(err)
	}
	if input.Mode&unix.S_IFMT != unix.S_IFIFO {
		t.Fatal("stdin control is not a pipe")
	}
	p := shellProcess(t, "if [ /dev/fd/0 -ef /dev/null ]; then printf null; else printf inherited; fi")
	run, err := p.Wait()
	if err != nil || run.End.Code != 0 || string(run.Stdout) != "null" {
		t.Fatalf("standard input is not /dev/null: output=%q end=%+v error=%v", run.Stdout, run.End, err)
	}
}

func TestCopyFIFO(t *testing.T) {
	root := t.TempDir()
	fifo := filepath.Join(root, "fifo")
	if err := unix.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := NewCopies(root, "/bin/sh", fifo); done <- err }()
	select {
	case err := <-done:
		if err == nil || err.Error() != "executable is not regular" {
			t.Fatalf("FIFO program was not refused: %v", err)
		}
	case <-time.After(time.Second):
		// Release a regressed blocking open before failing, leaving no worker behind.
		fd, err := unix.Open(fifo, unix.O_RDWR|unix.O_NONBLOCK, 0)
		if err != nil {
			t.Fatal(err)
		}
		result := <-done
		if err := unix.Close(fd); err != nil {
			t.Error(err)
		}
		t.Fatalf("FIFO program blocked in open: %v", result)
	}
}

func TestStartEncodedArgumentLimit(t *testing.T) {
	limit, err := unix.SysctlUint32("kern.argmax")
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewCopies(t.TempDir(), "/usr/bin/true", "/usr/bin/true")
	if err != nil {
		t.Fatal(err)
	}
	words := []string{strings.Repeat("\t", int(limit)/2)}
	if err := argumentSize(append([]string{c.program.path}, words...), nil, int(limit)); err != nil {
		t.Fatalf("program words must fit: %v", err)
	}
	values := map[string]string{}
	for _, name := range commonNames {
		values[name] = "/"
	}
	called := false
	p, err := c.startWith(Invocation{Paths: Paths{Values: values}, Words: words}, func(string, []string, string, []string, string) (Process, error) {
		called = true
		return &process{changed: true}, nil
	})
	if called || p != nil || err == nil || err.Error() != "program arguments and environment exceed kern.argmax" {
		t.Fatalf("encoded launcher arguments did not fault before launch: called=%v process=%v error=%v", called, p, err)
	}
}

func TestCaptureDeadlineAfterClose(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	c := drain(r)
	<-c.done
	if c.err != nil {
		t.Fatal(c.err)
	}
	if err := c.setDeadline(time.Now()); err != nil {
		t.Fatalf("deadline on completed drain: %v", err)
	}
}

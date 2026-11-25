package confine

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/vinnyp/press-quality-lab/internal/observe"
	"golang.org/x/sys/unix"
)

type privateCopy struct {
	path     string
	original observe.CopySnapshot
}

// Copies is created once per grade, then reused without repairing changed files.
// Its caller owns removal of the enclosing run directory.
type Copies struct {
	launcher, program privateCopy
	scratch           string
}

func fingerprintOf(path string) (observe.CopySnapshot, error) {
	var s syscall.Stat_t
	if err := syscall.Lstat(path, &s); err != nil {
		return observe.CopySnapshot{}, err
	}
	if s.Mode&syscall.S_IFMT != syscall.S_IFREG {
		return observe.CopySnapshot{}, fmt.Errorf("private copy is not regular")
	}
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return observe.CopySnapshot{}, err
	}
	h := sha256.New()
	_, readErr := io.Copy(h, f)
	closeErr := f.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return observe.CopySnapshot{}, err
	}
	var digest [32]byte
	copy(digest[:], h.Sum(nil))
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		return observe.CopySnapshot{}, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return observe.CopySnapshot{Entries: names, Inode: s.Ino, Mode: uint32(s.Mode), Size: s.Size, MtimeSeconds: s.Mtimespec.Sec, MtimeNanos: s.Mtimespec.Nsec, CtimeSeconds: s.Ctimespec.Sec, CtimeNanos: s.Ctimespec.Nsec, Digest: digest}, nil
}
func makeCopy(run, source string) (privateCopy, error) {
	dir, err := os.MkdirTemp(run, "executable-")
	if err != nil {
		return privateCopy{}, err
	}
	path := filepath.Join(dir, "program")
	in, err := os.OpenFile(source, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return privateCopy{}, err
	}
	info, statErr := in.Stat()
	if statErr != nil {
		return privateCopy{}, errors.Join(statErr, in.Close())
	}
	if !info.Mode().IsRegular() {
		return privateCopy{}, errors.Join(fmt.Errorf("executable is not regular"), in.Close())
	}
	out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return privateCopy{}, errors.Join(err, in.Close())
	}
	_, err = io.Copy(out, in)
	err = errors.Join(err, out.Chmod(info.Mode().Perm()))
	err = errors.Join(err, in.Close(), out.Close())
	if err != nil {
		return privateCopy{}, err
	}
	fp, err := fingerprintOf(path)
	return privateCopy{path, fp}, err
}
func (c privateCopy) unchanged() bool {
	entries, err := os.ReadDir(filepath.Dir(c.path))
	if err != nil || len(entries) != 1 || entries[0].Name() != "program" {
		return false
	}
	fp, err := fingerprintOf(c.path)
	return err == nil && observe.SameCopy(fp, c.original)
}
func NewCopies(run, launcher, program string) (*Copies, error) {
	l, err := makeCopy(run, launcher)
	if err != nil {
		return nil, err
	}
	p, err := makeCopy(run, program)
	if err != nil {
		return nil, err
	}
	scratch, err := os.MkdirTemp(run, "captures-")
	if err != nil {
		return nil, err
	}
	return &Copies{l, p, scratch}, nil
}

// CopyPaths returns the private launcher and subject paths for inspection.
// Callers must not repair or replace these copies.
func (c *Copies) CopyPaths() (launcher, program string) {
	return c.launcher.path, c.program.path
}

func (c *Copies) capture(prefix string) (*os.File, error) {
	return os.CreateTemp(c.scratch, prefix)
}

// closeCapture releases a capture and removes its directory entry even if close fails.
func closeCapture(f *os.File) error {
	return errors.Join(f.Close(), os.Remove(f.Name()))
}

type process struct {
	pid, queue     int
	stdout, stderr *os.File
	marker         string
	mu             sync.Mutex
	killed         bool
	changed        bool
}

func (c *Copies) Start(inv Invocation) (Process, error) {
	if !c.launcher.unchanged() || !c.program.unchanged() {
		return &process{changed: true}, nil
	}
	marker, err := NewMarker()
	if err != nil {
		return nil, err
	}
	hidden, err := LauncherArgs(marker, c.program.path, inv.Words)
	if err != nil {
		return nil, err
	}
	if inv.Fast {
		hidden = []string{FastMode, marker}
	}
	values := make(map[string]string, len(inv.Paths.Values))
	for k, v := range inv.Paths.Values {
		values[k] = v
	}
	values["LAUNCHER"], values["LAUNCHER_PARENT"] = c.launcher.path, filepath.Dir(c.launcher.path)
	values["BIN"], values["BIN_PARENT"] = c.program.path, filepath.Dir(c.program.path)
	inv.Paths.Values = values
	args, err := Arguments(inv.Computation, inv.Control, inv.Paths, hidden)
	if err != nil {
		return nil, err
	}
	// Regular temporary capture files avoid a blocked pipe while waiting. U29
	// will supply bounded stream draining at the source seam.
	stdout, err := c.capture(".stdout-")
	if err != nil {
		return nil, err
	}
	stderr, err := c.capture(".stderr-")
	if err != nil {
		return nil, errors.Join(err, closeCapture(stdout))
	}
	null, err := os.Open(os.DevNull)
	if err != nil {
		return nil, errors.Join(err, closeCapture(stdout), closeCapture(stderr))
	}
	queue, err := unix.Kqueue()
	if err != nil {
		return nil, errors.Join(err, null.Close(), closeCapture(stdout), closeCapture(stderr))
	}
	unix.CloseOnExec(queue)
	pid, spawnErr := syscall.ForkExec(SandboxExec, append([]string{SandboxExec}, args...), &syscall.ProcAttr{Dir: inv.Directory, Env: inv.Env, Files: []uintptr{null.Fd(), stdout.Fd(), stderr.Fd()}, Sys: &syscall.SysProcAttr{Setpgid: true}})
	closeErr := null.Close()
	if spawnErr != nil {
		return nil, errors.Join(spawnErr, closeErr, unix.Close(queue), closeCapture(stdout), closeCapture(stderr))
	}
	p := &process{pid: pid, queue: queue, stdout: stdout, stderr: stderr, marker: marker}
	if closeErr != nil {
		killErr := p.Kill()
		_, waitErr := p.Wait()
		return nil, errors.Join(closeErr, killErr, waitErr)
	}
	return p, nil
}

// killGroup reports whether a signal was sent, separately from an already ended
// group, so accepting a zombie-only EPERM does not invent an engine kill.
func killGroup(pgid int) (bool, error) {
	err := unix.Kill(-pgid, unix.SIGKILL)
	if err == nil {
		return true, nil
	}
	if err == unix.ESRCH {
		return false, nil
	}
	if err != unix.EPERM {
		return false, err
	}
	procs, lookupErr := unix.SysctlKinfoProcSlice("kern.proc.pgrp", pgid)
	if lookupErr != nil {
		return false, errors.Join(err, lookupErr)
	}
	// Darwin's sys/proc.h defines SZOMB as 5; x/sys exposes P_stat but
	// does not export this process-state constant.
	const zombieState = 5
	members := make([]groupMember, len(procs))
	for i, proc := range procs {
		members[i] = groupMember{pid: int(proc.Proc.P_pid), zombie: proc.Proc.P_stat == zombieState}
	}
	if groupEnded(members) {
		return false, nil
	}
	return false, err
}

func (p *process) Kill() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.changed {
		return nil
	}
	sent, err := killGroup(p.pid)
	if sent {
		p.killed = true
	}
	return err
}
func (p *process) Wait() (observe.Run, error) {
	if p.changed {
		return observe.Run{CopyChanged: true}, nil
	}
	change := unix.Kevent_t{Ident: uint64(p.pid), Filter: unix.EVFILT_PROC, Flags: unix.EV_ADD | unix.EV_ONESHOT, Fflags: unix.NOTE_EXIT | unix.NOTE_EXITSTATUS | unix.NOTE_EXIT_DETAIL}
	events := make([]unix.Kevent_t, 1)
	var waitErr error
	for {
		_, waitErr = unix.Kevent(p.queue, []unix.Kevent_t{change}, nil, nil)
		if waitErr != unix.EINTR {
			break
		}
	}
	var detail uint32
	if waitErr == nil {
		for {
			n, err := unix.Kevent(p.queue, nil, events, nil)
			if err == unix.EINTR {
				continue
			}
			waitErr = err
			if err != nil {
				break
			}
			if n == 0 {
				continue
			}
			if events[0].Flags&unix.EV_ERROR != 0 {
				waitErr = syscall.Errno(events[0].Data)
			} else {
				detail = events[0].Fflags & unix.NOTE_EXIT_DETAIL_MASK
			}
			break
		}
	}
	if waitErr == unix.ESRCH {
		waitErr = nil
	}
	if waitErr != nil {
		waitErr = errors.Join(waitErr, p.Kill())
	}
	p.mu.Lock()
	killed := p.killed
	p.mu.Unlock()
	_, groupErr := killGroup(p.pid)
	var status syscall.WaitStatus
	var reapErr error
	for {
		_, reapErr = syscall.Wait4(p.pid, &status, 0, nil)
		if reapErr != syscall.EINTR {
			break
		}
	}
	run := observe.Run{End: observe.End{Code: status.ExitStatus(), ExitDetail: detail, EngineKilled: killed}}
	if status.Signaled() {
		run.End.Signal = int(status.Signal())
	}
	_, outSeek := p.stdout.Seek(0, 0)
	_, errSeek := p.stderr.Seek(0, 0)
	out, outErr := io.ReadAll(p.stdout)
	stderr, errErr := io.ReadAll(p.stderr)
	run.Stdout = out
	run.Markers, run.Stderr = observe.ParseMarkers(stderr, MarkerPrefix+p.marker, FailurePrefix+p.marker)
	return run, errors.Join(waitErr, groupErr, reapErr, outSeek, errSeek, outErr, errErr, closeCapture(p.stdout), closeCapture(p.stderr), unix.Close(p.queue))
}

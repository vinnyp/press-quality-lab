package confine

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/throwaway"
	"golang.org/x/sys/unix"
)

// zombieState is SZOMB from Darwin sys/proc.h; x/sys exposes no constant.
const zombieState = 5

type privateCopy struct {
	path     string
	original observe.CopySnapshot
}

// Copies is created once per grade, then reused without repairing changed files.
// Its caller owns removal of the enclosing run directory.
type Copies struct {
	launcher, program privateCopy
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
	// U40 will move run-directory entry ownership to internal/throwaway.
	dir, err := os.MkdirTemp(run, "executable-")
	if err != nil {
		return privateCopy{}, err
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return privateCopy{}, err
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		return privateCopy{}, err
	}
	path := filepath.Join(dir, "program")
	in, err := os.OpenFile(source, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
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
	return &Copies{l, p}, nil
}

// CopyPaths returns the private launcher and subject paths for inspection.
// Callers must not repair or replace these copies.
func (c *Copies) CopyPaths() (launcher, program string) {
	return c.launcher.path, c.program.path
}

// StreamCap is an interim per-stream memory cap. U30 owns final heads,
// counts and digests; drains continue consuming every byte beyond this cap.
const StreamCap = 1 << 20

// PostExitDrainGrace bounds inherited writers after group ending and reap.
// U30 owns the final draining design.
const PostExitDrainGrace = 100 * time.Millisecond

type capture struct {
	closeMu      sync.Mutex
	closed       bool
	head         []byte
	seen         int64
	err          error
	done         chan struct{}
	reader       *os.File
	cutAfterExit bool
}

// setDeadline serializes the deadline with drain closing the reader.
func (c *capture) setDeadline(deadline time.Time) error {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	if c.closed {
		return nil
	}
	return c.reader.SetReadDeadline(deadline)
}

func drain(r io.ReadCloser) *capture {
	c := &capture{done: make(chan struct{})}
	if reader, ok := r.(*os.File); ok {
		c.reader = reader
	}
	go func() {
		defer close(c.done)
		defer func() {
			if v := recover(); v != nil {
				c.err = errors.Join(c.err, fmt.Errorf("stream drain panic: %v", v))
			}
		}()
		defer func() {
			c.closeMu.Lock()
			defer c.closeMu.Unlock()
			c.err = errors.Join(c.err, r.Close())
			c.closed = true
		}()
		buf := make([]byte, 32*1024)
		for {
			n, err := r.Read(buf)
			c.seen += int64(n)
			keep := min(n, StreamCap-len(c.head))
			c.head = append(c.head, buf[:keep]...)
			if err != nil {
				if errors.Is(err, os.ErrDeadlineExceeded) {
					c.cutAfterExit = true
				} else if err != io.EOF {
					c.err = err
				}
				return
			}
		}
	}()
	return c
}

type process struct {
	workspace                                              GroupWorkspace
	waitMu                                                 sync.Mutex
	waited                                                 bool
	result                                                 observe.Run
	waitError                                              error
	pid, queue                                             int
	stdout, stderr                                         *capture
	marker                                                 string
	mu                                                     sync.Mutex
	killed, observed, reaped, changed, registrationMissing bool
}

// registerExit runs immediately after spawn, before handing the process out.
func (p *process) registerExit() error {
	change := unix.Kevent_t{Ident: uint64(p.pid), Filter: unix.EVFILT_PROC, Flags: unix.EV_ADD | unix.EV_ONESHOT, Fflags: unix.NOTE_EXIT | unix.NOTE_EXITSTATUS | unix.NOTE_EXIT_DETAIL}
	for {
		_, err := unix.Kevent(p.queue, []unix.Kevent_t{change}, nil, nil)
		if err == unix.EINTR {
			continue
		}
		if err == unix.ESRCH {
			p.registrationMissing = true
			return nil
		}
		return err
	}
}

func argumentSize(words, env []string, limit int) error {
	size := 0
	for _, list := range [][]string{words, env} {
		for _, word := range list {
			size += strconv.IntSize/8 + len(word) + 1
		}
	}
	if size > limit {
		return fmt.Errorf("program arguments and environment exceed kern.argmax")
	}
	return nil
}

func (c *Copies) Start(inv Invocation) (Process, error) {
	return c.startWith(inv, spawn)
}

func (c *Copies) startWith(inv Invocation, launch func(string, []string, string, []string, string) (Process, error)) (Process, error) {
	if inv.Workspace == nil {
		return nil, fmt.Errorf("invocation workspace required")
	}
	if !c.launcher.unchanged() || !c.program.unchanged() {
		return &process{changed: true}, nil
	}
	marker, err := NewMarker()
	if err != nil {
		return nil, err
	}
	hidden, err := LauncherArgs(marker, c.program.path, inv.Words, inv.Env)
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
	limit, err := unix.SysctlUint32("kern.argmax")
	if err != nil {
		return nil, err
	}
	if err := argumentSize(append([]string{c.program.path}, inv.Words...), inv.Env, int(limit)); err != nil {
		return nil, err
	}
	argv, env := append([]string{SandboxExec}, args...), EngineEnvironment()
	if err := argumentSize(argv, env, int(limit)); err != nil {
		return nil, err
	}
	p, err := launch(SandboxExec, argv, inv.Directory, env, marker)
	if err != nil {
		return nil, err
	}
	if inv.Workspace != nil {
		real, ok := p.(*process)
		if !ok {
			killErr := p.Kill()
			_, waitErr := p.Wait()
			return nil, errors.Join(fmt.Errorf("unrecordable process"), killErr, waitErr)
		}
		start, err := throwaway.ProcessStart(real.pid)
		if err == nil {
			err = inv.Workspace.RecordGroup(throwaway.Process{PID: real.pid, StartTime: start})
		}
		if err != nil {
			killErr := p.Kill()
			_, waitErr := p.Wait()
			return nil, errors.Join(err, killErr, waitErr)
		}
		real.workspace = inv.Workspace
	}
	return p, nil
}

func spawn(path string, args []string, directory string, env []string, marker string) (Process, error) {
	stdout, outWrite, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	stderr, errWrite, err := os.Pipe()
	if err != nil {
		return nil, errors.Join(err, stdout.Close(), outWrite.Close())
	}
	null, err := os.Open(os.DevNull)
	if err != nil {
		return nil, errors.Join(err, stdout.Close(), outWrite.Close(), stderr.Close(), errWrite.Close())
	}
	queue, err := unix.Kqueue()
	if err != nil {
		return nil, errors.Join(err, null.Close(), stdout.Close(), outWrite.Close(), stderr.Close(), errWrite.Close())
	}
	unix.CloseOnExec(queue)
	// Stated limit: SIGKILL between ForkExec and RecordGroup can leave an unrecorded subject.
	pid, spawnErr := syscall.ForkExec(path, args, &syscall.ProcAttr{Dir: directory, Env: env, Files: []uintptr{null.Fd(), outWrite.Fd(), errWrite.Fd()}, Sys: &syscall.SysProcAttr{Setsid: true}})
	p := &process{pid: pid, queue: queue, marker: marker}
	var registerErr error
	if spawnErr == nil {
		registerErr = p.registerExit()
		if registerErr != nil {
			p.registrationMissing = true
		}
	}
	closeErr := errors.Join(null.Close(), outWrite.Close(), errWrite.Close())
	if spawnErr != nil {
		return nil, errors.Join(spawnErr, closeErr, unix.Close(queue), stdout.Close(), stderr.Close())
	}
	p.stdout, p.stderr = drain(stdout), drain(stderr)
	if err := errors.Join(registerErr, closeErr); err != nil {
		killErr := p.Kill()
		_, waitErr := p.Wait()
		return nil, errors.Join(err, killErr, waitErr)
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
	members, lookupErr := groupMembers(pgid)
	if lookupErr != nil {
		return false, errors.Join(err, lookupErr)
	}
	if groupEnded(members) {
		return false, nil
	}
	return false, err
}

func (p *process) Kill() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.changed || p.reaped {
		return nil
	}
	// A confinement kill between this leader lookup and the group signal can
	// be attributed to the engine while a descendant is still live. QO4 uses
	// the running state the engine saw for a budget kill; other engine kills
	// come from the exit funnel (exit 70). Either leaves the check not observed,
	// so this window cannot change a held or not-held outcome (QH21).
	proc, lookupErr := unix.SysctlKinfoProc("kern.proc.pid", p.pid)
	if lookupErr != nil {
		return lookupErr
	}
	if proc.Proc.P_stat == zombieState {
		p.observed = true
	}
	sent := false
	err := endGroupWith(func() (bool, error) {
		signaled, killErr := killGroup(p.pid)
		sent = sent || signaled
		return signaled, killErr
	}, func() ([]groupMember, error) { return groupMembers(p.pid) })
	if sent && !p.observed {
		p.killed = true
	}
	return err
}
func (p *process) Wait() (observe.Run, error) {
	p.waitMu.Lock()
	defer p.waitMu.Unlock()
	if p.waited {
		return p.result, p.waitError
	}
	p.result, p.waitError = p.wait()
	p.waited = true
	return p.result, p.waitError
}

func decodeEvent(event unix.Kevent_t) (uint32, error) {
	return exitDetail(event.Flags, event.Fflags, event.Data)
}

func (p *process) wait() (observe.Run, error) {
	if p.changed {
		return observe.Run{CopyChanged: true}, nil
	}
	events := make([]unix.Kevent_t, 1)
	var waitErr error
	var detail uint32
	if !p.registrationMissing {
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
			detail, waitErr = decodeEvent(events[0])
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
	p.observed = true
	groupErr := endGroup(p.pid)
	var status syscall.WaitStatus
	var reapErr error
	for {
		_, reapErr = syscall.Wait4(p.pid, &status, 0, nil)
		if reapErr != syscall.EINTR {
			break
		}
	}
	if reapErr == nil {
		p.reaped = true
	}
	p.mu.Unlock()
	run := observe.Run{End: observe.End{Code: status.ExitStatus(), ExitDetail: detail, EngineKilled: killed}}
	run.End.MemoryKilled, run.End.CodeSigningKilled = observe.DecodeExitDetail(detail)
	if status.Signaled() {
		run.End.Signal = int(status.Signal())
	}
	deadline := time.Now().Add(PostExitDrainGrace)
	var deadlineErr error
	for _, c := range []*capture{p.stdout, p.stderr} {
		select {
		case <-c.done:
		default:
			if err := c.setDeadline(deadline); err != nil && !errors.Is(err, os.ErrClosed) {
				deadlineErr = errors.Join(deadlineErr, err)
			}
		}
	}
	<-p.stdout.done
	<-p.stderr.done
	run.OutputCutAfterExit = p.stdout.cutAfterExit || p.stderr.cutAfterExit
	run.Stdout = p.stdout.head
	run.StdoutSeen, run.StderrSeen = p.stdout.seen, p.stderr.seen
	run.StdoutCut, run.StderrCut = p.stdout.seen > StreamCap, p.stderr.seen > StreamCap
	run.Markers, run.Stderr = observe.ParseMarkers(p.stderr.head, MarkerPrefix+p.marker, FailurePrefix+p.marker)
	run.StderrSeen -= int64(len(p.stderr.head) - len(run.Stderr))
	var forgetErr error
	if p.workspace != nil && groupErr == nil && reapErr == nil {
		forgetErr = p.workspace.ForgetGroup(p.pid)
	}
	return run, errors.Join(forgetErr, deadlineErr, waitErr, groupErr, reapErr, p.stdout.err, p.stderr.err, unix.Close(p.queue))
}

func groupMembers(pid int) ([]groupMember, error) {
	procs, err := unix.SysctlKinfoProcSlice("kern.proc.pgrp", pid)
	if err != nil {
		return nil, err
	}
	members := make([]groupMember, len(procs))
	for i, proc := range procs {
		members[i] = groupMember{pid: int(proc.Proc.P_pid), zombie: proc.Proc.P_stat == zombieState}
	}
	return members, nil
}

func endGroup(pid int) error {
	return endGroupWith(func() (bool, error) { return killGroup(pid) }, func() ([]groupMember, error) { return groupMembers(pid) })
}

func endGroupWith(kill func() (bool, error), lookup func() ([]groupMember, error)) error {
	deadline := time.Now().Add(5 * time.Second)
	for {
		members, err := lookup()
		if err != nil {
			return err
		}
		if groupEnded(members) {
			return nil
		}
		_, killErr := kill()
		if killErr != nil && !errors.Is(killErr, unix.EPERM) {
			return killErr
		}
		if time.Now().After(deadline) {
			return errors.Join(fmt.Errorf("process group did not end"), killErr)
		}
		time.Sleep(time.Millisecond)
	}
}

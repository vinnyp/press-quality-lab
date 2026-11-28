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
	run               string
	programs          map[string]privateCopy
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
	path, err := throwaway.CopyExecutable(run, source)
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
	return &Copies{launcher: l, program: p, run: run, programs: map[string]privateCopy{program: p}}, nil
}

// Unchanged supplies the grade-wide check without repairing any copy.
func (c *Copies) Unchanged() bool {
	if !c.launcher.unchanged() {
		return false
	}
	for _, program := range c.programs {
		if !program.unchanged() {
			return false
		}
	}
	return true
}

// CopyPaths returns the private launcher and subject paths for inspection.
// Callers must not repair or replace these copies.
func (c *Copies) CopyPaths() (launcher, program string) {
	return c.launcher.path, c.program.path
}

// StreamCap is QE10's per-stream keep bound; drains count every byte.
const StreamCap = 1 << 20

// PostExitDrainGrace bounds inherited writers after group ending and reap.
// This also bounds writers that escaped a computation's process group.
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

func drain(r io.ReadCloser) *capture { return drainLimit(r, StreamCap) }

func drainLimit(r io.ReadCloser, limit int) *capture {
	return drainLimitWithPoll(r, limit, unix.Poll)
}

func drainLimitWithPoll(r io.ReadCloser, limit int, poll func([]unix.PollFd, int) (int, error)) *capture {
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
			if errors.Is(err, os.ErrDeadlineExceeded) && c.reader != nil {
				// The poll deadline may expire before this goroutine runs,
				// even when all writers have closed. Inspect the nonblocking
				// pipe once: queued bytes and EOF are still observable.
				raw, rawErr := c.reader.SyscallConn()
				if rawErr == nil {
					rawErr = raw.Control(func(fd uintptr) {
						fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
						pollErr := retryPoll(func() error { _, e := poll(fds, 0); return e })
						if pollErr != nil {
							err = pollErr
							return
						}
						// Only drain after HUP: a live inherited writer must
						// still be bounded by the original deadline.
						if fds[0].Revents&unix.POLLHUP != 0 {
							for {
								n, err = unix.Read(int(fd), buf)
								if err != unix.EINTR {
									break
								}
							}
							if n < 0 {
								n = 0
							}
						}
					})
					if err == nil && n == 0 {
						err = io.EOF
					}
					if err == unix.EAGAIN {
						err = os.ErrDeadlineExceeded
					}
				}
				if rawErr != nil {
					err = rawErr
				}
			}
			c.seen += int64(n)
			keep := min(n, limit-len(c.head))
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

func retryPoll(poll func() error) error {
	for {
		if err := poll(); err != unix.EINTR {
			return err
		}
	}
}

type process struct {
	sampleLeader func(int) (*unix.KinfoProc, error)
	signalGroup  func(int) (bool, error)

	workspace                                              GroupWorkspace
	waitMu                                                 sync.Mutex
	waited                                                 bool
	result                                                 observe.Run
	waitError                                              error
	budgetTimer                                            *time.Timer
	budgetError                                            error
	budgetExpired, runningAtBudget                         bool
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
	if inv.Executable != "" {
		if !filepath.IsAbs(inv.Executable) || filepath.Clean(inv.Executable) != inv.Executable {
			return nil, fmt.Errorf("executable path must be absolute and canonical")
		}
	}
	program := c.program
	inPlace := false
	if inv.Computation && inv.Executable != "" {
		if systemExecutable(inv.Executable) {
			resolved, err := resolvedSystemExecutable(inv.Executable, filepath.EvalSymlinks, os.Stat)
			if err != nil {
				return nil, err
			}
			program = privateCopy{path: resolved}
			inPlace = true
		} else {
			if err := canonicalExecutable(inv.Executable); err != nil {
				return nil, err
			}
			var ok bool
			program, ok = c.programs[inv.Executable]
			if !ok {
				var err error
				program, err = makeCopy(c.run, inv.Executable)
				if err != nil {
					return nil, err
				}
				c.programs[inv.Executable] = program
			}
		}
	}
	if !c.launcher.unchanged() || !inPlace && !program.unchanged() {
		return &process{changed: true}, nil
	}
	marker, err := NewMarker()
	if err != nil {
		return nil, err
	}
	environment, err := invocationEnvironment(inv.Paths.Values["HOME"], os.Getuid(), inv.Env)
	if err != nil {
		return nil, err
	}
	hidden, err := LauncherArgs(marker, program.path, inv.Words, environment)
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
	values["BIN"], values["BIN_PARENT"] = program.path, filepath.Dir(program.path)
	inv.Paths.Values = values
	args, err := Arguments(inv.Computation, inv.Control, inv.Paths, hidden)
	if err != nil {
		return nil, err
	}
	limit, err := unix.SysctlUint32("kern.argmax")
	if err != nil {
		return nil, err
	}
	if err := argumentSize(append([]string{program.path}, inv.Words...), environment, int(limit)); err != nil {
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
	if inv.Budget > 0 {
		real, ok := p.(*process)
		if !ok {
			return nil, fmt.Errorf("budget requires recordable process")
		}
		real.armBudget(inv.Budget)
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
	plan := planLaunch(directory, env)
	null, err := os.Open(plan.stdin)
	if err != nil {
		return nil, errors.Join(err, stdout.Close(), outWrite.Close(), stderr.Close(), errWrite.Close())
	}
	queue, err := unix.Kqueue()
	if err != nil {
		return nil, errors.Join(err, null.Close(), stdout.Close(), outWrite.Close(), stderr.Close(), errWrite.Close())
	}
	unix.CloseOnExec(queue)
	// Stated limit: SIGKILL between ForkExec and RecordGroup can leave an unrecorded subject.
	plan.attributes.Files = []uintptr{null.Fd(), outWrite.Fd(), errWrite.Fd()}
	plan.attributes.Sys = &syscall.SysProcAttr{Setsid: plan.newSession}
	pid, spawnErr := syscall.ForkExec(path, args, &plan.attributes)
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
	p.stdout = drain(stdout)
	// Reserve space for both protocol lines before applying the subject cap.
	p.stderr = drainLimit(stderr, StreamCap+len(MarkerPrefix+marker+"\n")+len(FailurePrefix+marker+"\n"))
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
	return p.killLocked()
}

func (p *process) killLocked() error {
	if p.changed || p.reaped {
		return nil
	}
	// A confinement kill between this leader lookup and the group signal can
	// be attributed to the engine while a descendant is still live. QO4 uses
	// the running state the engine saw for a budget kill; other engine kills
	// come from the exit funnel (exit 70). Either leaves the check not observed,
	// so this window cannot change a held or not-held outcome (QH21).
	proc, lookupErr := p.leader()
	if lookupErr != nil {
		// The unreaped leader pins the pid, even when sampling fails.
		sent, killErr := p.sendGroupKill()
		p.killed = p.killed || sent
		return errors.Join(lookupErr, killErr, endGroup(p.pid))
	}
	if proc.Proc.P_stat == zombieState {
		p.observed = true
	}
	sent := false
	err := endGroupWith(func() (bool, error) {
		signaled, killErr := p.sendGroupKill()
		sent = sent || signaled
		return signaled, killErr
	}, func() ([]groupMember, error) { return groupMembers(p.pid) })
	if sent && !p.observed {
		p.killed = true
	}
	return err
}

func (p *process) sendGroupKill() (bool, error) {
	if p.signalGroup != nil {
		return p.signalGroup(p.pid)
	}
	return killGroup(p.pid)
}

func (p *process) leader() (*unix.KinfoProc, error) {
	if p.sampleLeader != nil {
		return p.sampleLeader(p.pid)
	}
	return unix.SysctlKinfoProc("kern.proc.pid", p.pid)
}

func (p *process) armBudget(budget time.Duration) {
	p.budgetTimer = time.AfterFunc(budget, p.expireBudget)
}

// expireBudget samples the unreaped leader under the same lock as engine kills.
func (p *process) expireBudget() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.reaped || p.observed {
		return
	}
	proc, sampleErr := p.leader()
	if sampleErr == nil && proc.Proc.P_stat == zombieState {
		return
	}
	killErr := p.killLocked()
	if p.killed {
		p.budgetExpired = true
		p.runningAtBudget = sampleErr == nil && proc.Proc.P_stat != zombieState
	}
	p.budgetError = errors.Join(sampleErr, killErr)
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
	if p.budgetTimer != nil {
		p.budgetTimer.Stop()
	}
	p.mu.Lock()
	killed := p.killed
	budgetExpired, runningAtBudget, budgetErr := p.budgetExpired, p.runningAtBudget, p.budgetError
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
	run := observe.Run{End: observe.End{Code: status.ExitStatus(), ExitDetail: detail, EngineKilled: killed, BudgetExpired: budgetExpired, RunningAtBudget: runningAtBudget}}
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
	run.Markers, run.Stderr = observe.ParseMarkers(p.stderr.head, MarkerPrefix+p.marker, FailurePrefix+p.marker)
	run.StderrSeen -= int64(len(p.stderr.head) - len(run.Stderr))
	run.Stderr = run.Stderr[:min(len(run.Stderr), StreamCap)]
	run.StdoutCut, run.StderrCut = run.StdoutSeen > StreamCap, run.StderrSeen > StreamCap
	var forgetErr error
	if p.workspace != nil && groupErr == nil && reapErr == nil {
		forgetErr = p.workspace.ForgetGroup(p.pid)
	}
	return run, errors.Join(budgetErr, forgetErr, deadlineErr, waitErr, groupErr, reapErr, p.stdout.err, p.stderr.err, unix.Close(p.queue))
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

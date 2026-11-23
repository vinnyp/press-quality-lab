//go:build confinementspike && darwin

// Package confinement contains disposable experiments, never collector code.
package confinement

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed profile/profile.sb
var template string

func profile(computation bool) string {
	return plantedProfile(computation, 0)
}

func plantedProfile(computation bool, depth int) string {
	rules := ""
	if computation {
		rules = "; child: the computation must be able to start and reap a child.\n(allow process-fork)\n; sandbox-signal: a computation manages its own child.\n(allow signal (target same-sandbox))\n(deny syscall-unix (syscall-number SYS_setsid SYS_setpgid) (with send-signal SIGKILL))"
	}
	target := "self"
	if computation {
		target = "same-sandbox"
	}
	fileRules := `; ordinary-files: files are read back in the redirected home.
(allow file-read* (subpath (param "HOME")))
; home-write: a write in the redirected home.
(allow file-write* (subpath (param "HOME")))`
	if computation {
		fileRules = `; system-value: the shell checks the maximum group count.
(allow sysctl-read (sysctl-name "kern.ngroups"))
; sort-pipeline: the sorter sizes its buffer from memory and page size.
(allow sysctl-read (sysctl-name "hw.memsize") (sysctl-name "hw.pagesize"))
; script-pipeline: arbitrary system tools and their executable images.
(allow process-exec* file-read-data (subpath (param "BIN_DIR")) (subpath (param "USR_BIN_DIR")))
; system-value: shell terminal probe in a terminal-free session.
(allow file-read-data file-write-data (literal (param "TTY_DEVICE")))
; declared-read: the pipeline consumes this declared file.
(allow file-read-data (literal (param "DECLARED")))
; seed-read: pristine seed input, with no write grant.
(allow file-read-data (literal (param "SEED")))
; ordinary-files: computations read back their scratch files.
(allow file-read* (subpath (param "HOME")))
; home-write: computations write only to scratch.
(allow file-write* (subpath (param "HOME")))`
	}
	fileRules += `
; read-proof: the planted file access time cannot be forged.
(deny file-write* (literal (param "PLANTED")) (with send-signal SIGKILL))
; home-root: keep the redirected root in place while allowing writes within it.
(deny file-write-unlink file-write-create (literal (param "HOME")) (with send-signal SIGKILL))`
	if !computation {
		for i := 1; i <= depth; i++ {
			fileRules += fmt.Sprintf("\n(deny file-write-unlink file-write-create (literal (param \"PLANTED_DIR_%d\")) (with send-signal SIGKILL))", i)
		}
	}
	if computation {
		fileRules += `
; subject-boundary: tool directory grants never expose the subject.
(deny file-read-data process-exec* (literal (param "SUBJECT")) (literal (param "SUBJECT_COPY")) (with send-signal SIGKILL))`
	}
	return strings.NewReplacer("{{PROCESS_RULES}}", rules, "{{PROCESS_TARGET}}", target, "{{FILE_RULES}}", fileRules).Replace(template)
}

// Prevent CoreFoundation startup from reading the user-database home.
var coreFoundationEncoding = fmt.Sprintf("__CF_USER_TEXT_ENCODING=0x%X:0x0:0x0", os.Getuid())

type result struct {
	output        string
	diagnostic    string
	marked        bool
	ready         bool
	startFailed   bool
	harnessKilled bool
	exitDetail    uint32
	timedOut      bool
	status        syscall.WaitStatus
	err           error
}

func run(binary string, args, env []string, dir string) result {
	return runBudget(binary, args, env, dir, defaultBudget())
}

func runBudget(binary string, args, env []string, dir string, budget time.Duration) result {
	return runBudgetWithFiles(binary, args, env, dir, budget, nil, nil)
}

func runBudgetWithFiles(binary string, args, env []string, dir string, budget time.Duration, launcherFiles []*os.File, firstLine func(string) error) result {
	return runBudgetWithObserver(binary, args, env, dir, budget, launcherFiles, firstLine, nil)
}
func runBudgetWithObserver(binary string, args, env []string, dir string, budget time.Duration, launcherFiles []*os.File, firstLine func(string) error, beforeObservation func(int)) result {
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = env
	cmd.ExtraFiles = launcherFiles
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	// Only this command's newly created group is signalled.
	var reapMu sync.Mutex
	reaping := false
	cmd.Cancel = func() error {
		reapMu.Lock()
		defer reapMu.Unlock()
		if reaping {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = time.Second
	var out, diagnostic bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &diagnostic
	if firstLine != nil {
		cmd.Stdout = &lineOutput{output: &out, firstLine: firstLine}
	}
	// Observe exit without reaping the leader. Its id cannot be reused before
	// the group cleanup below. The command context still owns the deadline.
	queue, queueErr := syscall.Kqueue()
	if queueErr != nil {
		return result{err: queueErr, startFailed: true}
	}
	defer syscall.Close(queue)
	err := cmd.Start()
	started := err == nil
	supervisorKilled := false
	var exitDetail uint32
	if started {
		if beforeObservation != nil {
			beforeObservation(cmd.Process.Pid)
		}
		changes := []syscall.Kevent_t{{Ident: uint64(cmd.Process.Pid), Filter: syscall.EVFILT_PROC, Flags: syscall.EV_ADD | syscall.EV_ONESHOT, Fflags: syscall.NOTE_EXIT | syscall.NOTE_EXIT_DETAIL}}
		events := make([]syscall.Kevent_t, 1)
		n, observationErr := waitEvent(queue, changes, events, budget+time.Second)
		// Registration supplies no event list, so an already-exited leader
		// reports ESRCH as the call errno. It remains unreaped and cannot be reused.
		exitedBeforeRegistration := errors.Is(observationErr, syscall.ESRCH)
		if n == 1 {
			exitDetail = events[0].Fflags & syscall.NOTE_EXIT_DETAIL_MASK
		}
		if !exitedBeforeRegistration && (observationErr != nil || n != 1 || events[0].Flags&syscall.EV_ERROR != 0 || events[0].Fflags&syscall.NOTE_EXIT == 0) {
			supervisorKilled = true
		}
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		reapMu.Lock()
		reaping = true
		reapMu.Unlock()
		err = cmd.Wait()
	}
	rawDiagnostic := diagnostic.String()
	ready := strings.Contains(rawDiagnostic, "operation-ready\n")
	cleanDiagnostic := strings.ReplaceAll(rawDiagnostic, "operation-ready\n", "")
	r := result{exitDetail: exitDetail, output: out.String(), diagnostic: cleanDiagnostic, ready: ready, startFailed: !started || rawDiagnostic == "confined\nlaunch-failed\n", harnessKilled: supervisorKilled, marked: strings.HasPrefix(cleanDiagnostic, "confined\n"), timedOut: ctx.Err() != nil, err: err}
	if cmd.ProcessState != nil {
		r.status, _ = cmd.ProcessState.Sys().(syscall.WaitStatus)
	}
	return r
}

// Register once before waiting: repeating EV_ADD after an interrupted wait can
// replace a queued one-shot exit event. Registration retries only EINTR; an
// exited child reports ESRCH through errno. Observation retries never re-register.
func waitEvent(queue int, changes, events []syscall.Kevent_t, budget time.Duration) (int, error) {
	deadline := time.Now().Add(budget)
	if len(changes) != 0 {
		for {
			_, err := syscall.Kevent(queue, changes, nil, nil)
			if err == nil {
				break
			}
			if err != syscall.EINTR || !time.Now().Before(deadline) {
				return 0, err
			}
		}
	}
	for {
		remaining := max(time.Until(deadline), 0)
		wait := syscall.NsecToTimespec(remaining.Nanoseconds())
		n, err := syscall.Kevent(queue, nil, events, &wait)
		// After the deadline the wait is a non-blocking poll, so retrying an
		// interrupted one keeps the deadline and still reads a queued event.
		if err != syscall.EINTR {
			return n, err
		}
	}
}

func (r result) killed() bool {
	return r.exitDetail&(syscall.NOTE_EXIT_MEMORY|syscall.NOTE_EXIT_CSERROR) == 0 && r.ready && !r.harnessKilled && !r.startFailed && r.marked && !r.timedOut && r.status.Signaled() && r.status.Signal() == syscall.SIGKILL && r.output == "" && r.diagnostic == "confined\n"
}

// Required refusals are independent of the rendered template. This also checks
// kill modifiers for query-only operations, whose destructive effects must not
// be attempted by the self-test.
const requiredRefusals = `default sysctl-read asr-parser-enter coalition-info consume-extension darwin-notification-post file-clone file-graft file-link file-map-executable file-read-xattr file-ungraft fs-quota* fs-snapshot-mount iokit-get-properties iokit-open-service isp-command-send mach-bootstrap mach-cross-domain-lookup mach-task-exception-port-set mach-task-special-port* necp-client-open nvram* process-iopolicy* socket-ioctl socket-option-get socket-option-set system-memorystatus-control system-necp-client-action system-privilege process-info-codesignature process-info-sandbox-container process-info*`

func refusalsIntact(p string, computation bool) bool {
	for _, line := range []string{
		`(deny file-write-unlink file-write-create (literal (param "HOME")) (with send-signal SIGKILL))`,
		`(deny file-write* (literal (param "PLANTED")) (with send-signal SIGKILL))`,
	} {
		if !strings.Contains(p, line) {
			return false
		}
	}
	if computation && !strings.Contains(p, `(deny file-read-data process-exec* (literal (param "SUBJECT")) (literal (param "SUBJECT_COPY")) (with send-signal SIGKILL))`) {
		return false
	}

	if !strings.Contains(p, `(deny fs-info (with send-signal SIGKILL))`) {
		return false
	}
	if !strings.Contains(p, `(deny file-lock (with send-signal SIGKILL))`) {
		return false
	}
	for _, op := range []string{"exception-entitlement", "process-exec-update-label"} {
		if !strings.Contains(p, "(deny "+op+")") {
			return false
		}
	}
	for _, op := range strings.Fields(requiredRefusals) {
		if !strings.Contains(p, "(deny "+op+" (with send-signal SIGKILL))") {
			return false
		}
	}
	if computation && !strings.Contains(p, "(deny syscall-unix (syscall-number SYS_setsid SYS_setpgid) (with send-signal SIGKILL))") {
		return false
	}
	return true
}

// lineOutput observes one trusted protocol line while its parent is still alive.
type lineOutput struct {
	output    *bytes.Buffer
	firstLine func(string) error
	pending   []byte
}

func (w *lineOutput) Write(p []byte) (int, error) {
	if _, err := w.output.Write(p); err != nil {
		return 0, err
	}
	if w.firstLine != nil {
		w.pending = append(w.pending, p...)
		if index := bytes.IndexByte(w.pending, '\n'); index >= 0 {
			callback := w.firstLine
			w.firstLine = nil
			if err := callback(string(w.pending[:index])); err != nil {
				return len(p), err
			}
			w.pending = nil
		} else if len(w.pending) > 32 {
			return len(p), errors.New("trusted protocol line too long")
		}
	}
	return len(p), nil
}

// Budgets only bound a run that hangs or must outlive its program's start;
// no scenario measures their exact value. A slow machine may scale them with
// CONFINEMENT_BUDGET_SCALE (1 to 10).
func budgetScale() int {
	scale, err := strconv.Atoi(os.Getenv("CONFINEMENT_BUDGET_SCALE"))
	if err != nil || scale < 1 || scale > 10 {
		return 1
	}
	return scale
}

func defaultBudget() time.Duration { return time.Duration(budgetScale()) * 5 * time.Second }

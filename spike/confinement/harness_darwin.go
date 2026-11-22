//go:build confinementspike && darwin

// Package confinement contains disposable experiments, never collector code.
package confinement

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

//go:embed profile/profile.sb
var template string

func profile(computation bool) string {
	rules := ""
	if computation {
		rules = "; child: the computation must be able to start and reap a child.\n(allow process-fork)\n; sandbox-signal: a computation manages its own child.\n(allow signal (target same-sandbox))\n(deny syscall-unix (syscall-number SYS_setsid SYS_setpgid) (with send-signal SIGKILL))"
	}
	target := "self"
	if computation {
		target = "same-sandbox"
	}
	fileRules := `; home-read: a read in the redirected home.
(allow file-read* (literal (param "HOME")) (subpath (param "HOME")))
; home-write: a write in the redirected home.
(allow file-write* (subpath (param "HOME")))`
	if computation {
		fileRules = `; script-pipeline: arbitrary system tools and their executable images.
(allow process-exec* file-read-data (subpath (param "BIN_DIR")) (subpath (param "USR_BIN_DIR")))
; system-value: shell terminal probe in a terminal-free session.
(allow file-read-data file-write-data (literal (param "TTY_DEVICE")))
; declared-read: the pipeline consumes this declared file.
(allow file-read-data (literal (param "DECLARED")))
; database-read: its declared WAL and shared-memory side files.
(allow file-read-data (literal (param "WAL")))
(allow file-read-data (literal (param "SHM")))
; seed-read: pristine seed input, with no write grant.
(allow file-read-data (literal (param "SEED")))
; directory: scratch-directory metadata and traversal.
(allow file-read-data (literal (param "HOME")))
; home-write: computations write only to scratch.
(allow file-write* (subpath (param "HOME")))`
	}
	return strings.NewReplacer("{{PROCESS_RULES}}", rules, "{{PROCESS_TARGET}}", target, "{{FILE_RULES}}", fileRules).Replace(template)
}

type result struct {
	output     string
	diagnostic string
	marked     bool
	timedOut   bool
	status     syscall.WaitStatus
	err        error
}

func run(binary string, args, env []string, dir string) result {
	return runBudget(binary, args, env, dir, 5*time.Second)
}

func runBudget(binary string, args, env []string, dir string, budget time.Duration) result {
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = env
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	// Only this command's newly created group is signalled.
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = time.Second
	var out, diagnostic bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &diagnostic
	err := cmd.Run()
	if cmd.Process != nil {
		// End remaining group members after both ordinary exits and timeouts.
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	r := result{output: out.String(), diagnostic: diagnostic.String(), marked: diagnostic.String() == "confined\n", timedOut: ctx.Err() != nil, err: err}
	if cmd.ProcessState != nil {
		r.status, _ = cmd.ProcessState.Sys().(syscall.WaitStatus)
	}
	return r
}

func (r result) killed() bool {
	return r.marked && !r.timedOut && r.status.Signaled() && r.status.Signal() == syscall.SIGKILL && r.output == ""
}

// Required refusals are independent of the rendered template. This also checks
// kill modifiers for query-only operations, whose destructive effects must not
// be attempted by the self-test.
const requiredRefusals = `default asr-parser-enter coalition-info consume-extension darwin-notification-post file-clone file-graft file-link file-map-executable file-read-xattr file-ungraft fs-quota* fs-snapshot-mount iokit-get-properties iokit-open-service isp-command-send mach-bootstrap mach-cross-domain-lookup mach-task-exception-port-set mach-task-special-port* necp-client-open nvram* process-iopolicy* socket-ioctl socket-option-get socket-option-set system-memorystatus-control system-necp-client-action system-privilege process-info-codesignature process-info-sandbox-container process-info*`

func refusalsIntact(p string, computation bool) bool {
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

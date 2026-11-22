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
	return strings.ReplaceAll(strings.ReplaceAll(template, "{{PROCESS_RULES}}", rules), "{{PROCESS_TARGET}}", target)
}

type result struct {
	output   string
	marked   bool
	timedOut bool
	status   syscall.WaitStatus
	err      error
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
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
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
	r := result{output: out.String(), marked: diagnostic.String() == "confined\n", timedOut: ctx.Err() != nil, err: err}
	if cmd.ProcessState != nil {
		r.status, _ = cmd.ProcessState.Sys().(syscall.WaitStatus)
	}
	return r
}

func (r result) killed() bool {
	return r.marked && !r.timedOut && r.status.Signaled() && r.status.Signal() == syscall.SIGKILL && r.output == ""
}

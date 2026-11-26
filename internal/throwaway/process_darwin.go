package throwaway

import (
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// ProcessStart uses the kernel's creation timestamp, including microseconds.
func ProcessStart(pid int) (string, error) {
	procs, err := unix.SysctlKinfoProcSlice("kern.proc.pid", pid)
	if errors.Is(err, unix.ESRCH) {
		return "", os.ErrNotExist
	}
	if err != nil {
		return "", err
	}
	if len(procs) == 0 {
		return "", os.ErrNotExist
	}
	if len(procs) != 1 {
		return "", fmt.Errorf("ambiguous process identity for %d", pid)
	}
	p := procs[0]
	return fmt.Sprintf("%d:%d", p.Proc.P_starttime.Sec, p.Proc.P_starttime.Usec), nil
}

// EndGroup waits for the group to become empty or zombie-only (QH23).
func EndGroup(pid int) error {
	deadline := time.Now().Add(5 * time.Second)
	for {
		killErr := unix.Kill(-pid, unix.SIGKILL)
		if killErr != nil && !errors.Is(killErr, unix.ESRCH) && !errors.Is(killErr, unix.EPERM) {
			return killErr
		}
		procs, err := unix.SysctlKinfoProcSlice("kern.proc.pgrp", pid)
		if err != nil {
			return err
		}
		live := false
		for _, p := range procs {
			if p.Proc.P_stat != 5 {
				live = true
			}
		}
		if !live {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.Join(fmt.Errorf("process group %d did not end", pid), killErr)
		}
		time.Sleep(time.Millisecond)
	}
}

func openAt(dir int, name string, flags int, mode uint32) (int, error) {
	return unix.Openat(dir, name, flags, mode)
}

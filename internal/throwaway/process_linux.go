package throwaway

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func processFields(pid int) ([]string, error) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return nil, processLookupError(err)
	}
	i := strings.LastIndexByte(string(b), ')')
	if i < 0 {
		return nil, errors.New("invalid process stat")
	}
	f := strings.Fields(string(b[i+1:]))
	if len(f) < 20 {
		return nil, errors.New("short process stat")
	}
	return f, nil
}

// ProcessStart returns the kernel start tick, independent of the process name.
func ProcessStart(pid int) (string, error) {
	f, err := processFields(pid)
	if err != nil {
		return "", err
	}
	return f[19], nil
}

// EndGroup ignores zombies, which cannot execute, while waiting for QH23.
func EndGroup(pid int) error {
	deadline := time.Now().Add(5 * time.Second)
	for {
		entries, err := os.ReadDir("/proc")
		if err != nil {
			return err
		}
		live := false
		for _, e := range entries {
			id, err := strconv.Atoi(e.Name())
			if err != nil {
				continue
			}
			f, err := processFields(id)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return err
			}
			group, err := strconv.Atoi(f[2])
			if err != nil {
				return err
			}
			if group == pid && f[0] != "Z" && f[0] != "X" {
				live = true
			}
		}
		if !live {
			return nil
		}
		killErr := syscall.Kill(-pid, syscall.SIGKILL)
		if killErr != nil && !errors.Is(killErr, syscall.ESRCH) && !errors.Is(killErr, syscall.EPERM) {
			return killErr
		}
		if time.Now().After(deadline) {
			return errors.Join(fmt.Errorf("process group %d did not end", pid), killErr)
		}
		time.Sleep(time.Millisecond)
	}
}

func openAt(dir int, name string, flags int, mode uint32) (int, error) {
	return syscall.Openat(dir, name, flags, mode)
}

func renameAt(fd int, from, to string) error { return syscall.Renameat(fd, from, fd, to) }

func unlinkAt(fd int, name string) error { return syscall.Unlinkat(fd, name) }

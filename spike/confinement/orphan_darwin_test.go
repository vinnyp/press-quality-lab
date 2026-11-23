//go:build confinementspike && darwin

package confinement

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestOrphanAfterGroupEndSmoke(t *testing.T) {
	t.Log("smoke test: these kills do not isolate the operation rule")
	t.Parallel()
	f := computationFixture(t)
	for _, mode := range []string{"orphan-session", "orphan-group"} {
		for _, confined := range []bool{false, true} {
			name := mode + "/positive"
			if confined {
				name = mode + "/confined"
			}
			runScenario(t, name, func(t *testing.T) {
				repeat(t, func() bool {
					root, err := os.MkdirTemp(f.home(), "orphan-")
					if err != nil {
						t.Fatal("create orphan rendezvous")
					}
					defer os.RemoveAll(root)
					probe, quiet, closeDecoy := networkDecoy(t, f, "tcp")
					defer closeDecoy()
					probe.args = append(probe.args, root)
					p := ""
					if confined {
						p = profile(true)
					}
					queue, err := syscall.Kqueue()
					if err != nil {
						t.Fatal("observe orphan exit")
					}
					defer syscall.Close(queue)
					pid := 0
					probe.firstLine = func(line string) error {
						candidate, err := strconv.Atoi(line)
						if err != nil || candidate <= 1 {
							return errors.New("invalid child pid")
						}
						// The trusted parent has not reaped its child or exited.
						// Registration binds the observer before either can happen.
						changes := []syscall.Kevent_t{{Ident: uint64(candidate), Filter: syscall.EVFILT_PROC, Flags: syscall.EV_ADD | syscall.EV_ONESHOT, Fflags: syscall.NOTE_EXIT | syscall.NOTE_EXITSTATUS}}
						if _, err := syscall.Kevent(queue, changes, nil, nil); err != nil {
							return err
						}
						pid = candidate
						return os.WriteFile(filepath.Join(root, "observed"), nil, 0600)
					}
					r := probe.invoke(p, mode, false)
					if pid <= 1 || !correct(r, strconv.Itoa(pid)+"\n") {
						return false
					}
					// The group and parent have ended. Only now can the orphan
					// write its ready file and attempt the forbidden operation.
					if os.WriteFile(filepath.Join(root, "go"), nil, 0600) != nil {
						t.Fatal("release orphan")
					}
					events := make([]syscall.Kevent_t, 1)
					n, err := waitEvent(queue, nil, events, 3*time.Second)
					// The child's own two-second alarm bounds it even if observation fails.
					if err != nil || n != 1 || events[0].Flags&syscall.EV_ERROR != 0 {
						return false
					}
					status := syscall.WaitStatus(events[0].Data)
					ready, err := os.ReadFile(filepath.Join(root, "ready"))
					if err != nil || string(ready) != "ready" {
						return false
					}
					_, fallbackErr := os.Lstat(filepath.Join(root, "fallback"))
					untouched := quiet()
					if confined {
						return status.Signaled() && status.Signal() == syscall.SIGKILL && os.IsNotExist(fallbackErr) && untouched
					}
					return status.Exited() && status.ExitStatus() == 0 && fallbackErr == nil && !untouched
				})
			})
		}
	}
	t.Log("open limit: children leaving through posix_spawn outlive group cleanup; the orphan stayed confined and had its own two-second alarm")
}

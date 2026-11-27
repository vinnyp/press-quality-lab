package confine

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"

	"github.com/vinnyp/press-quality-lab/internal/throwaway"
	"golang.org/x/sys/unix"
)

// DecoyHidden delegates unconfined workspace socket creation to throwaway.
func DecoyHidden() int { return throwaway.SelfTestDecoyHidden() }

type selfTestDecoy struct {
	lifetime  *os.File
	pid       int
	tcp, udp  int
	workspace GroupWorkspace
}

func startSelfTestDecoy(executable, home string, workspace GroupWorkspace) (_ *selfTestDecoy, err error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, r.Close()) }()
	lifeRead, lifeWrite, err := os.Pipe()
	if err != nil {
		return nil, errors.Join(err, w.Close())
	}
	defer func() { err = errors.Join(err, lifeRead.Close()) }()
	// Use the same launcher stream/session setup, but never sandbox this owned
	// decoy: it must actually own the endpoints the confined witnesses attempt.
	pid, spawnErr := syscall.ForkExec(executable, []string{executable, DecoyMode}, &syscall.ProcAttr{Dir: home, Env: EngineEnvironment(), Files: []uintptr{w.Fd(), w.Fd(), w.Fd(), lifeRead.Fd()}, Sys: &syscall.SysProcAttr{Setsid: true}})
	closeErr := w.Close()
	if spawnErr != nil {
		return nil, errors.Join(spawnErr, closeErr, lifeWrite.Close())
	}
	d := &selfTestDecoy{pid: pid, workspace: workspace, lifetime: lifeWrite}
	defer func() {
		if err != nil {
			err = errors.Join(err, d.close())
		}
	}()
	if closeErr != nil {
		return nil, closeErr
	}
	start, err := throwaway.ProcessStart(pid)
	if err != nil {
		return nil, err
	}
	if err := workspace.RecordGroup(throwaway.Process{PID: pid, StartTime: start}); err != nil {
		return nil, err
	}
	if err := r.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return nil, err
	}
	var b [12]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return nil, err
	}
	if !bytes.Equal(b[:8], []byte("pql-dec:")) {
		return nil, fmt.Errorf("invalid decoy readiness")
	}
	d.tcp, d.udp = int(b[8])<<8|int(b[9]), int(b[10])<<8|int(b[11])
	if d.tcp == 0 || d.udp == 0 {
		return nil, fmt.Errorf("invalid decoy ports")
	}
	return d, nil
}

func (d *selfTestDecoy) close() error {
	killErr := unix.Kill(-d.pid, unix.SIGKILL)
	if killErr == unix.ESRCH {
		killErr = nil
	}
	var status syscall.WaitStatus
	var waitErr error
	for {
		_, waitErr = syscall.Wait4(d.pid, &status, 0, nil)
		if waitErr != syscall.EINTR {
			break
		}
	}
	var forgetErr error
	if waitErr == nil {
		forgetErr = d.workspace.ForgetGroup(d.pid)
		if !status.Signaled() || status.Signal() != syscall.SIGKILL {
			waitErr = fmt.Errorf("decoy exited unexpectedly: %d", status)
		}
	}
	return errors.Join(killErr, waitErr, forgetErr, d.lifetime.Close())
}

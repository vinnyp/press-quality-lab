package throwaway

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// SelfTestFiles owns synthetic QH9 targets, all beneath this run's workspace.
// The engine prepares these before any confined process can see the home.
type SelfTestFiles struct {
	Home, Outside, Create, Clean, Calibration string
	Before                                    time.Time
	workspace                                 *Workspace
	calibrationName                           string
	identity                                  syscall.Stat_t
	outsideIdentity                           syscall.Stat_t
}

func (w *Workspace) PrepareSelfTest() (_ *SelfTestFiles, err error) {
	dir, err := os.MkdirTemp(w.path, "selftest-")
	if err != nil {
		return nil, err
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	if err := root.Mkdir("home", 0700); err != nil {
		return nil, err
	}
	if err := root.Mkdir("home/.pql-tmp", 0700); err != nil {
		return nil, err
	}
	for _, row := range []struct{ name, value string }{{"outside", "outside_value\n"}, {"home/calibration", "calibration_value\n"}} {
		f, err := root.OpenFile(row.name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return nil, err
		}
		_, writeErr := f.WriteString(row.value)
		if err := errors.Join(writeErr, f.Close()); err != nil {
			return nil, err
		}
	}
	mtime := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := root.Chtimes("home/calibration", mtime.Add(-24*time.Hour), mtime); err != nil {
		return nil, err
	}
	info, err := root.Lstat("home/calibration")
	if err != nil {
		return nil, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("invalid calibration file")
	}
	outside, err := root.Lstat("outside")
	if err != nil {
		return nil, err
	}
	outsideStat, ok := outside.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("invalid outside decoy")
	}
	workspacePath, err := filepath.Abs(w.path)
	if err != nil {
		return nil, err
	}
	workspacePath, err = filepath.EvalSymlinks(workspacePath)
	if err != nil {
		return nil, err
	}
	name, err := filepath.Rel(workspacePath, filepath.Join(dir, "home/calibration"))
	if err != nil {
		return nil, err
	}
	return &SelfTestFiles{Home: filepath.Join(dir, "home"), Outside: filepath.Join(dir, "outside"), Create: filepath.Join(dir, "create"), Clean: filepath.Join(dir, "home/clean"), Calibration: filepath.Join(dir, "home/calibration"), Before: time.Unix(stat.Atimespec.Sec, stat.Atimespec.Nsec), workspace: w, calibrationName: name, identity: *stat, outsideIdentity: *outsideStat}, nil
}

// CalibrationAfter inspects metadata without reading the decoy or following
// a replacement link. A replaced or modified decoy cannot calibrate a volume.
func (f *SelfTestFiles) CalibrationAfter() (_ time.Time, err error) {
	root, err := os.OpenRoot(f.workspace.path)
	if err != nil {
		return time.Time{}, err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	info, err := root.Lstat(f.calibrationName)
	if err != nil {
		return time.Time{}, err
	}
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || s.Dev != f.identity.Dev || s.Ino != f.identity.Ino || s.Size != f.identity.Size || s.Mtimespec != f.identity.Mtimespec {
		return time.Time{}, fmt.Errorf("calibration decoy changed")
	}
	return time.Unix(s.Atimespec.Sec, s.Atimespec.Nsec), nil
}

// CheckDecoys enforces QH9's violator rule after the witness suite.
func (f *SelfTestFiles) CheckDecoys() error {
	info, err := os.Lstat(f.Outside)
	if err != nil {
		return err
	}
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || s.Dev != f.outsideIdentity.Dev || s.Ino != f.outsideIdentity.Ino || s.Mode != f.outsideIdentity.Mode || s.Size != f.outsideIdentity.Size || s.Mtimespec != f.outsideIdentity.Mtimespec || s.Ctimespec != f.outsideIdentity.Ctimespec {
		return fmt.Errorf("outside decoy changed")
	}
	b, err := os.ReadFile(f.Outside)
	if err != nil {
		return err
	}
	if string(b) != "outside_value\n" {
		return fmt.Errorf("outside decoy changed")
	}
	if _, err := os.Lstat(f.Create); !os.IsNotExist(err) {
		return errors.Join(fmt.Errorf("create decoy changed"), err)
	}
	return nil
}

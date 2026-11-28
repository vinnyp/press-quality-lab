//go:build darwin && arm64

package confine

import (
	"errors"
	"golang.org/x/sys/unix"
	"path/filepath"
	"syscall"
	"unsafe"
)

// Each named witness attempts only its own operation and never reads the
// planted file. The two read controls have separate, explicit output oracles.
func plantedWitness(name, target string) (bool, int) {
	switch name {
	case "never_reads_secret":
		if err := selfTestOutput("never_reads_value\n"); err != nil {
			return true, LauncherFailed
		}
		return true, 0
	case "real_read":
		if err := selfTestOutput("attempt_read\n"); err != nil {
			return true, LauncherFailed
		}
		fd, err := unix.Open(target, unix.O_RDONLY|unix.O_NOFOLLOW, 0)
		if err != nil {
			return true, selfTestError(err)
		}
		var b [128]byte
		n, readErr := unix.Read(fd, b[:])
		if err := errors.Join(readErr, unix.Close(fd)); err != nil {
			return true, selfTestError(err)
		}
		if n == 0 {
			return true, selfTestError(unix.EIO)
		}
		if err := selfTestOutput("read_value\n"); err != nil {
			return true, LauncherFailed
		}
		return true, 0
	case "forge_utimes", "forge_setattrlist", "forge_write", "forge_rename", "forge_replace", "forge_directory":
	default:
		return false, 0
	}
	if err := selfTestOutput("attempt_" + name + "\n"); err != nil {
		return true, LauncherFailed
	}
	var err error
	switch name {
	case "forge_utimes":
		err = unix.Utimes(target, []unix.Timeval{{Sec: 1}, {Sec: 1}})
	case "forge_setattrlist":
		p, e := unix.BytePtrFromString(target)
		if e != nil {
			return true, selfTestError(e)
		}
		attrs := unix.Attrlist{Bitmapcount: 5, Commonattr: unix.ATTR_CMN_ACCTIME}
		value := unix.Timespec{Sec: 1}
		//lint:ignore SA1019 x/sys has no setattrlist wrapper; this witness attempts that exact call.
		_, _, errno := syscall.Syscall6(unix.SYS_SETATTRLIST, uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&attrs)), uintptr(unsafe.Pointer(&value)), unsafe.Sizeof(value), 0, 0)
		if errno != 0 {
			err = errno
		}
	case "forge_write":
		fd, e := unix.Open(target, unix.O_WRONLY|unix.O_NOFOLLOW, 0)
		err = e
		if err == nil {
			_, e = unix.Write(fd, []byte("forged"))
			err = errors.Join(e, unix.Close(fd))
		}
	case "forge_directory":
		directory := filepath.Dir(target)
		moved := directory + ".renamed"
		err = unix.Rename(directory, moved)
		if err == nil {
			err = unix.Utimes(filepath.Join(moved, filepath.Base(target)), []unix.Timeval{{Sec: 1}, {Sec: 1}})
		}
		if err == nil {
			err = unix.Rename(moved, directory)
		}
	case "forge_rename":
		err = unix.Rename(target, target+".renamed")
	case "forge_replace":
		err = unix.Rename(filepath.Join(filepath.Dir(target), ".pql-tmp", "replacement"), target)
	}
	if err != nil {
		return true, selfTestError(err)
	}
	if err := selfTestOutput("returned_" + name + "\n"); err != nil {
		return true, LauncherFailed
	}
	return true, 0
}

//go:build darwin && macos_evidence

package throwaway

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
)

func setDenyACL(t *testing.T, path string) {
	t.Helper()
	pid, err := syscall.ForkExec("/bin/chmod", []string{"chmod", "+a", "everyone deny read,delete,writeattr,writesecurity,chown", path}, &syscall.ProcAttr{Env: []string{"PATH=/usr/bin:/bin"}, Files: []uintptr{0, 1, 2}})
	if err != nil {
		t.Fatal(err)
	}
	var status syscall.WaitStatus
	if _, err := syscall.Wait4(pid, &status, 0, nil); err != nil {
		t.Fatal(err)
	}
	if status != 0 {
		t.Fatalf("ACL setup: %v", status)
	}
}
func aclBytes(t *testing.T, path string) []byte {
	t.Helper()
	p, err := unix.BytePtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	attrs := unix.Attrlist{Bitmapcount: 5, Commonattr: unix.ATTR_CMN_EXTENDED_SECURITY}
	buf := make([]byte, 4096)
	//lint:ignore SA1019 x/sys has no getattrlist wrapper; the evidence must compare the outside ACL bytes.
	_, _, errno := syscall.Syscall6(unix.SYS_GETATTRLIST, uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&attrs)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), unix.FSOPT_NOFOLLOW, 0)
	if errno != 0 {
		t.Fatal(errno)
	}
	length := int(binary.LittleEndian.Uint32(buf))
	if length < 12 || length > len(buf) {
		t.Fatalf("ACL length %d", length)
	}
	return append([]byte(nil), buf[:length]...)
}
func hostileFixture(t *testing.T, dir bool, flags uint32, deny, locked bool) {
	t.Helper()
	w, err := Create(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(w.Path(), "hostile")
	if dir {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "child"), []byte("child"), 0600); err != nil {
			t.Fatal(err)
		}
	} else {
		if err := os.WriteFile(path, []byte("file"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Positive control from the same fixture, before any hostile metadata.
	if err := w.Remove(); err != nil {
		t.Fatal("clean control", err)
	}
	w, err = Create(filepath.Dir(w.Path()))
	if err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(w.Path(), "hostile")
	if dir {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "child"), []byte("child"), 0600); err != nil {
			t.Fatal(err)
		}
	} else {
		if err := os.WriteFile(path, []byte("file"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		if err := w.Remove(); err != nil {
			t.Error(err)
		}
	})
	if locked {
		if err := os.Chmod(path, 0000); err != nil {
			t.Fatal(err)
		}
	}
	if deny {
		setDenyACL(t, path)
		if len(aclBytes(t, path)) <= 12 {
			t.Fatal("ACL fixture not observed")
		}
	}
	if flags != 0 {
		if err := unix.Chflags(path, int(flags)); err != nil {
			t.Fatal(err)
		}
	}
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		t.Fatal(err)
	}
	if st.Flags&flags != flags {
		t.Fatal("flags not set")
	}
	if locked && st.Mode&0777 != 0 {
		t.Fatal("mode not set")
	}
	if err := w.Remove(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(w.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("workspace remains: %v", err)
	}
}
func TestDarwinUchgFile(t *testing.T) { // QE20, KTD20.
	hostileFixture(t, false, unix.UF_IMMUTABLE, false, false)
}
func TestDarwinUchgDirectory(t *testing.T) { // QE20, KTD20.
	hostileFixture(t, true, unix.UF_IMMUTABLE, false, false)
}
func TestDarwinUappnd(t *testing.T) { // QE20, KTD20.
	t.Run("File", func(t *testing.T) { hostileFixture(t, false, unix.UF_APPEND, false, false) })
	t.Run("Directory", func(t *testing.T) { hostileFixture(t, true, unix.UF_APPEND, false, false) })
}
func TestDarwinDenyFile(t *testing.T) { // QE20, KTD20.
	hostileFixture(t, false, 0, true, false)
}
func TestDarwinDenyDirectory(t *testing.T) { // QE20, KTD20.
	hostileFixture(t, true, 0, true, false)
}
func TestDarwinModeZero(t *testing.T) { // QE20, KTD20.
	t.Run("File", func(t *testing.T) { hostileFixture(t, false, 0, false, true) })
	t.Run("Directory", func(t *testing.T) { hostileFixture(t, true, 0, false, true) })
}
func TestDarwinCombinedFile(t *testing.T) { // QE20, KTD20.
	hostileFixture(t, false, unix.UF_IMMUTABLE, true, true)
}
func TestDarwinCombinedDirectory(t *testing.T) { // QE20, KTD20.
	hostileFixture(t, true, unix.UF_IMMUTABLE, true, true)
}
func TestDarwinFIFO(t *testing.T) { // QE20, KTD20, QH25.
	w, err := Create(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(filepath.Join(w.Path(), "fifo"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := w.Remove(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(w.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("FIFO remains: %v", err)
	}
}
func TestDarwinLeafLinkMetadata(t *testing.T) { // QE20, KTD20, QH25.
	parent := t.TempDir()
	outside := filepath.Join(parent, "outside")
	if err := os.WriteFile(outside, []byte("kept"), 0400); err != nil {
		t.Fatal(err)
	}
	setDenyACL(t, outside)
	if err := unix.Chflags(outside, unix.UF_IMMUTABLE); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unix.Chflags(outside, 0); err != nil {
			t.Error(err)
		}
		setNoACL(t, outside)
	})
	var before unix.Stat_t
	if err := unix.Lstat(outside, &before); err != nil {
		t.Fatal(err)
	}
	acl := aclBytes(t, outside)
	w, err := Create(parent)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(w.Path(), "link")); err != nil {
		t.Fatal(err)
	}
	if err := w.Remove(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(w.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("workspace remains: %v", err)
	}
	var after unix.Stat_t
	if err := unix.Lstat(outside, &after); err != nil {
		t.Fatal(err)
	}
	if before.Mode != after.Mode || before.Flags != after.Flags || !bytes.Equal(acl, aclBytes(t, outside)) {
		t.Fatal("outside metadata changed")
	}
}
func setNoACL(t *testing.T, path string) {
	t.Helper()
	pid, err := syscall.ForkExec("/bin/chmod", []string{"chmod", "-N", path}, &syscall.ProcAttr{Files: []uintptr{0, 1, 2}})
	if err != nil {
		t.Fatal(err)
	}
	var status syscall.WaitStatus
	if _, err := syscall.Wait4(pid, &status, 0, nil); err != nil {
		t.Fatal(err)
	}
	if status != 0 {
		t.Fatalf("ACL teardown: %v", status)
	}
}

func protectRaceOutside(t *testing.T, path string) func() {
	t.Helper()
	pid, err := syscall.ForkExec("/bin/chmod", []string{"chmod", "+a", "everyone deny delete", path}, &syscall.ProcAttr{Files: []uintptr{0, 1, 2}})
	if err != nil {
		t.Fatal(err)
	}
	var status syscall.WaitStatus
	if _, err := syscall.Wait4(pid, &status, 0, nil); err != nil {
		t.Fatal(err)
	}
	if status != 0 {
		t.Fatalf("race ACL setup: %v", status)
	}
	if err := unix.Chflags(path, unix.UF_IMMUTABLE); err != nil {
		t.Fatal(err)
	}
	var before unix.Stat_t
	if err := unix.Lstat(path, &before); err != nil {
		t.Fatal(err)
	}
	acl := aclBytes(t, path)
	t.Cleanup(func() {
		if err := unix.Chflags(path, 0); err != nil {
			t.Error(err)
		}
		setNoACL(t, path)
	})
	return func() {
		var after unix.Stat_t
		if err := unix.Lstat(path, &after); err != nil {
			t.Fatal(err)
		}
		if after.Flags != before.Flags || !bytes.Equal(acl, aclBytes(t, path)) {
			t.Fatal("outside race flags or ACL changed")
		}
	}
}

func clearRaceACL(t *testing.T, path string) {
	t.Helper()
	setNoACL(t, path)
}

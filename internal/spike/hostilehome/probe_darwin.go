//go:build darwin

// Package hostilehome is a disposable U43 experiment, not engine cleanup code.
package hostilehome

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

func result(err error) string {
	if err == nil {
		return "success"
	}
	var e syscall.Errno
	if errors.As(err, &e) {
		return fmt.Sprintf("errno=%d(%s)", e, e)
	}
	return err.Error()
}

// security encodes attrreference_t followed by kauth_filesec (sys/kauth.h).
// The ACE denies everyone delete and read-data/list-directory, without denying
// write-security, so the descriptor removal experiment remains meaningful.
func security(deny bool) []byte {
	n := 44
	if deny {
		n += 24
	}
	b := make([]byte, 8+n)
	binary.LittleEndian.PutUint32(b, 8)
	binary.LittleEndian.PutUint32(b[4:], uint32(n))
	f := b[8:]
	binary.LittleEndian.PutUint32(f, 0x012cc16d)
	if deny {
		binary.LittleEndian.PutUint32(f[36:], 1)
		copy(f[44:], []byte{0xab, 0xcd, 0xef, 0xab, 0xcd, 0xef, 0xab, 0xcd, 0xef, 0xab, 0xcd, 0xef, 0, 0, 0, 12})
		binary.LittleEndian.PutUint32(f[60:], 2)
		binary.LittleEndian.PutUint32(f[64:], 0x2|0x10)
	} else {
		binary.LittleEndian.PutUint32(f[36:], 0xffffffff)
	}
	return b
}

func attrs() unix.Attrlist {
	return unix.Attrlist{Bitmapcount: 5, Commonattr: unix.ATTR_CMN_EXTENDED_SECURITY}
}

// fdAttrs uses Darwin's syscall interface because x/sys has neither
// Fgetattrlist/Fsetattrlist nor acl_get_fd/acl_delete_fd/acl_set_fd wrappers.
func fdAttrs(fd int, set bool, b []byte) error {
	a := attrs()
	//lint:ignore SA1019 U43 spike measures fgetattrlist, missing from x/sys.
	call := uintptr(unix.SYS_FGETATTRLIST)
	if set {
		//lint:ignore SA1019 U43 spike measures fsetattrlist, missing from x/sys.
		call = unix.SYS_FSETATTRLIST
	}
	_, _, errno := syscall.Syscall6(call, uintptr(fd), uintptr(unsafe.Pointer(&a)), uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}

// Hostile attempts one item in its current directory, retaining no descriptor.
func Hostile(w io.Writer, item string) {
	step := func(name string, err error) { fmt.Fprintf(w, "attempt %s %s=%s\n", item, name, result(err)) }
	if item == "symlink" {
		step("create", os.WriteFile(item, []byte("synthetic"), 0600))
		step("replace-remove", unix.Unlink(item))
		step("symlink", unix.Symlink("../outside", item))
		return
	}
	isDir := strings.HasPrefix(item, "dir-")
	if item == "fifo" {
		step("mkfifo", unix.Mkfifo(item, 0600))
		return
	}
	if item == "socket" {
		fd, err := unix.Socket(unix.AF_UNIX, unix.SOCK_STREAM, 0)
		step("socket", err)
		if err == nil {
			step("bind", unix.Bind(fd, &unix.SockaddrUnix{Name: item}))
			step("close", unix.Close(fd))
		}
		return
	}
	if isDir {
		step("mkdir", unix.Mkdir(item, 0700))
		fd, err := unix.Open(item+"/child", unix.O_CREAT|unix.O_EXCL|unix.O_WRONLY|unix.O_NOFOLLOW, 0600)
		step("create-child", err)
		if err == nil {
			step("close-child", unix.Close(fd))
		}
	} else {
		fd, err := unix.Open(item, unix.O_CREAT|unix.O_EXCL|unix.O_WRONLY|unix.O_NOFOLLOW, 0600)
		step("create", err)
		if err != nil {
			return
		}
		step("close", unix.Close(fd))
	}
	if strings.HasSuffix(item, "acl") || strings.HasSuffix(item, "combined") {
		a := attrs()
		step("setattrlist-deny", unix.Setattrlist(item, &a, security(true), unix.FSOPT_NOFOLLOW))
	}
	if strings.HasSuffix(item, "xattr") {
		step("setxattr", unix.Setxattr(item, "org.pql.spike", []byte("synthetic"), 0))
	}
	if strings.HasSuffix(item, "mode") || strings.HasSuffix(item, "combined") {
		step("chmod-0000", unix.Chmod(item, 0))
	}
	if strings.HasSuffix(item, "uchg") || strings.HasSuffix(item, "combined") {
		step("chflags-uchg", unix.Chflags(item, unix.UF_IMMUTABLE))
	}
	if strings.HasSuffix(item, "uappnd") {
		step("chflags-uappnd", unix.Chflags(item, unix.UF_APPEND))
	}
}

// Cleanup records each step, including inability to open mode-0000 entries.
// Only fixed direct children are used; this is not a production race-safe walk.
func Cleanup(w io.Writer, home, item string) error {
	root, err := os.OpenRoot(home)
	if err != nil {
		return err
	}
	defer func() { fmt.Fprintf(w, "root-close %s=%s\n", item, result(root.Close())) }()
	parent, err := unix.Open(home, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer func() { fmt.Fprintf(w, "parent-close %s=%s\n", item, result(unix.Close(parent))) }()
	info, statErr := root.Lstat(item)
	fmt.Fprintf(w, "cleanup %s lstat=%s", item, result(statErr))
	if statErr == nil {
		fmt.Fprintf(w, " mode=%s", info.Mode())
		if st, ok := info.Sys().(*syscall.Stat_t); ok {
			fmt.Fprintf(w, " flags=%#x", st.Flags)
		}
		if info.Mode().IsRegular() || info.IsDir() {
			fd, openErr := unix.Openat(parent, item, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
			fmt.Fprintf(w, " openat=%s", result(openErr))
			if openErr != nil {
				fd, openErr = unix.Openat(parent, item, unix.O_EVTONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
				fmt.Fprintf(w, " openat-event-only=%s", result(openErr))
			}
			if openErr == nil {
				var st unix.Stat_t
				e := unix.Fstat(fd, &st)
				fmt.Fprintf(w, " fstat=%s fd-mode=%#o fd-flags=%#x", result(e), st.Mode, st.Flags)
				b := make([]byte, 4096)
				e = fdAttrs(fd, false, b)
				fmt.Fprintf(w, " fgetattrlist=%s acl-bytes=%x", result(e), b[:80])
				if e == nil {
					offset := 4 + int(int32(binary.LittleEndian.Uint32(b[4:])))
					length := int(binary.LittleEndian.Uint32(b[8:]))
					if offset >= 12 && length >= 44 && offset+length <= len(b) {
						count := binary.LittleEndian.Uint32(b[offset+36:])
						fmt.Fprintf(w, " acl-present=%t acl-count=%d", count != 0 && count != 0xffffffff, count)
					} else {
						fmt.Fprint(w, " acl-present=unknown-invalid-layout")
					}
				} else {
					fmt.Fprint(w, " acl-present=unknown")
				}
				x := make([]byte, 4096)
				n, xe := unix.Flistxattr(fd, x)
				if n < 0 || n > len(x) {
					n = 0
				}
				fmt.Fprintf(w, " xattrs=%q flistxattr=%s", x[:n], result(xe))
				for _, name := range strings.Split(string(x[:n]), "\x00") {
					if name != "" {
						fmt.Fprintf(w, " fremovexattr[%q]=%s", name, result(unix.Fremovexattr(fd, name)))
					}
				}
				fmt.Fprintf(w, " fchflags=%s", result(unix.Fchflags(fd, 0)))
				fmt.Fprintf(w, " fsetattrlist-remove-acl=%s", result(fdAttrs(fd, true, security(false))))
				fmt.Fprintf(w, " fchmod=%s close=%s", result(unix.Fchmod(fd, 0700)), result(unix.Close(fd)))
			} else {
				fmt.Fprint(w, " acl-present=unknown xattrs=unknown undo=unavailable-no-descriptor")
			}
		} else {
			fmt.Fprint(w, " open=skipped-special-file acl-present=unmeasured xattrs=unmeasured")
		}
		// This fallback measures whether the os.Root mode restoration can unlock
		// an entry whose initial descriptor open failed. It never clears flags by path.
		fmt.Fprintf(w, " root-chmod=%s", result(root.Chmod(item, 0700)))
		if info.IsDir() {
			child, e := root.OpenRoot(item)
			fmt.Fprintf(w, " walk-open=%s", result(e))
			if e == nil {
				nested, le := child.Lstat("child")
				fmt.Fprintf(w, " child-lstat=%s", result(le))
				if le == nil {
					fmt.Fprintf(w, " child-mode=%s child-removeall=%s", nested.Mode(), result(child.RemoveAll("child")))
				}
				fmt.Fprintf(w, " walk-root-close=%s", result(child.Close()))
			}
		}
	}
	fmt.Fprintf(w, " removeall=%s", result(root.RemoveAll(item)))
	_, err = root.Lstat(item)
	fmt.Fprintf(w, " gone=%t final-lstat=%s\n", os.IsNotExist(err), result(err))
	return nil
}

// SetDenyACL prepares the unconfined outside-file control.
func SetDenyACL(path string) error {
	a := attrs()
	return unix.Setattrlist(path, &a, security(true), unix.FSOPT_NOFOLLOW)
}

// Snapshot inspects a descriptor retained before the outside file is hardened.
func Snapshot(fd int) (string, error) {
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		return "", err
	}
	b := make([]byte, 4096)
	if err := fdAttrs(fd, false, b); err != nil {
		return "", err
	}
	return fmt.Sprintf("mode=%#o flags=%#x acl-bytes=%x", st.Mode, st.Flags, b), nil
}

// parentAttrs never opens or follows the entry itself.
func parentAttrs(parent int, name string, flags bool) error {
	p, err := unix.BytePtrFromString(name)
	if err != nil {
		return err
	}
	a := attrs()
	b := security(false)
	if flags {
		a.Commonattr = unix.ATTR_CMN_FLAGS
		b = make([]byte, 4)
	}
	//lint:ignore SA1019 U43 spike measures setattrlistat, missing from x/sys.
	_, _, errno := syscall.Syscall6(unix.SYS_SETATTRLISTAT, uintptr(parent), uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&a)), uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), unix.FSOPT_NOFOLLOW)
	if errno != 0 {
		return errno
	}
	return nil
}

// CleanupParent walks through no-follow directory descriptors and removes the
// named entry. It is a bounded-fixture spike, not the U40 racing-tree walker.
func CleanupParent(w io.Writer, home, item string) error {
	parent, err := unix.Open(home, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	fmt.Fprintf(w, "parent-open %s=%s\n", item, result(err))
	if err != nil {
		return err
	}
	defer func() { fmt.Fprintf(w, "parent-close %s=%s\n", item, result(unix.Close(parent))) }()
	return cleanupAt(w, parent, item)
}

func cleanupAt(w io.Writer, parent int, name string) error {
	fmt.Fprintf(w, "cleanup-parent %s setattrlistat-clear-flags=%s", name, result(parentAttrs(parent, name, true)))
	fmt.Fprintf(w, " setattrlistat-clear-acl=%s", result(parentAttrs(parent, name, false)))
	fmt.Fprintf(w, " fchmodat-nofollow=%s", result(unix.Fchmodat(parent, name, 0700, unix.AT_SYMLINK_NOFOLLOW)))
	var st unix.Stat_t
	err := unix.Fstatat(parent, name, &st, unix.AT_SYMLINK_NOFOLLOW)
	fmt.Fprintf(w, " fstatat-nofollow=%s\n", result(err))
	flags := 0
	if err == nil && st.Mode&unix.S_IFMT == unix.S_IFDIR {
		flags = unix.AT_REMOVEDIR
		fd, e := unix.Openat(parent, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		fmt.Fprintf(w, "parent-walk %s openat=%s\n", name, result(e))
		if e == nil {
			dir := os.NewFile(uintptr(fd), name)
			names, re := dir.Readdirnames(-1)
			fmt.Fprintf(w, "parent-walk %s readdir=%s\n", name, result(re))
			for _, child := range names {
				if ce := cleanupAt(w, fd, child); ce != nil {
					fmt.Fprintf(w, "parent-child %s=%s\n", child, result(ce))
				}
			}
			fmt.Fprintf(w, "parent-walk %s close=%s\n", name, result(dir.Close()))
		}
	}
	removeErr := unix.Unlinkat(parent, name, flags)
	finalErr := unix.Fstatat(parent, name, &st, unix.AT_SYMLINK_NOFOLLOW)
	fmt.Fprintf(w, "cleanup-parent %s unlinkat=%s gone=%t final-fstatat=%s\n", name, result(removeErr), errors.Is(finalErr, unix.ENOENT), result(finalErr))
	return removeErr
}

package throwaway

import (
	"errors"
	"io"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// setattrAt is the one raw syscall: x/sys v0.38.0 has no setattrlistat
// wrapper. Every pointer is passed directly in the syscall expression so it
// remains live, as required by Go's unsafe.Pointer syscall rules.
func setattrAt(fd int, name string, attribute uint32, data unsafe.Pointer, size uintptr) error {
	path, err := unix.BytePtrFromString(name)
	if err != nil {
		return err
	}
	attrs := unix.Attrlist{Bitmapcount: 5, Commonattr: attribute}
	//lint:ignore SA1019 x/sys has no setattrlistat wrapper; the hostile-home spike requires this parent-relative no-follow call.
	_, _, errno := syscall.Syscall6(unix.SYS_SETATTRLISTAT, uintptr(fd), uintptr(unsafe.Pointer(path)), uintptr(unsafe.Pointer(&attrs)), uintptr(data), size, unix.FSOPT_NOFOLLOW)
	if errno != 0 {
		return errno
	}
	return nil
}

func prepareAt(fd int, name string) error {
	var st unix.Stat_t
	if err := unix.Fstatat(fd, name, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return err
	}
	// Preserve outside inode metadata. The confinement.md file-link kill
	// prevents subjects from creating these multiply-linked leaves.
	if st.Mode&unix.S_IFMT != unix.S_IFDIR && st.Nlink > 1 {
		return nil
	}
	flags := st.Flags &^ uint32(unix.UF_IMMUTABLE|unix.UF_APPEND)
	if err := setattrAt(fd, name, unix.ATTR_CMN_FLAGS, unsafe.Pointer(&flags), unsafe.Sizeof(flags)); err != nil {
		return err
	}
	// ATTR_CMN_EXTENDED_SECURITY takes an attrreference followed by filesec.
	// KAUTH_FILESEC_NOACL is the uint32 all-ones ACL entry count; the magic
	// and count are from Darwin kauth.h, not a descriptor ACL operation.
	acl := struct {
		Offset int32
		Length uint32
		Magic  uint32
		Owner  [16]byte
		Group  [16]byte
		Count  uint32
		Flags  uint32
	}{Offset: 8, Length: 44, Magic: 0x012cc16d, Count: 0xffffffff}
	if err := setattrAt(fd, name, unix.ATTR_CMN_EXTENDED_SECURITY, unsafe.Pointer(&acl), unsafe.Sizeof(acl)); err != nil {
		return err
	}
	return unix.Fchmodat(fd, name, 0700, unix.AT_SYMLINK_NOFOLLOW)
}

func removeWalkEntry(parent *os.Root, name string, b *budget) (err error) {
	dir, err := parent.OpenFile(".", os.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, dir.Close()) }()
	return removeAt(int(dir.Fd()), name, b)
}
func removeAt(parent int, name string, b *budget) (err error) {
	if err := b.check(true); err != nil {
		return err
	}
	if err := prepareAt(parent, name); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var st unix.Stat_t
	if err := unix.Fstatat(parent, name, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	flags := 0
	if st.Mode&unix.S_IFMT == unix.S_IFDIR {
		if b.beforeDirectoryOpen != nil {
			b.beforeDirectoryOpen(parent, name)
		}
		fd, err := unix.Openat(parent, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		dir := os.NewFile(uintptr(fd), name)
		childErr := removeChildrenAt(dir, b)
		if err := errors.Join(childErr, dir.Close()); err != nil {
			return err
		}
		flags = unix.AT_REMOVEDIR
	}
	if err := b.check(false); err != nil {
		return err
	}
	err = unix.Unlinkat(parent, name, flags)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
func removeChildrenAt(dir *os.File, b *budget) error {
	for {
		if err := b.check(false); err != nil {
			return err
		}
		names, readErr := dir.Readdirnames(1)
		for _, name := range names {
			if err := removeAt(int(dir.Fd()), name, b); err != nil {
				return err
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

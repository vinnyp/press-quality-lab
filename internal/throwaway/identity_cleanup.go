package throwaway

import (
	"errors"
	"io"
	"os"
	"syscall"
)

// removeEntry keeps the publication recoverable if the last unlink fails.
// The directory descriptor pins the original workspace through replacements;
// recovery can never write into a symlink's destination.
func removeEntry(parent *os.Root, name string, b *budget) (err error) {
	root, openErr := parent.OpenRoot(name)
	if openErr != nil {
		return removeWalkEntry(parent, name, b)
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	dir, openErr := root.OpenFile(".", os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if openErr != nil {
		return openErr
	}
	defer func() { err = errors.Join(err, dir.Close()) }()
	data, readErr := readPublication(root)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) && !errors.Is(readErr, errNotRegular) {
		return readErr
	}
	err = removeWalkEntry(parent, name, b)
	if err != nil && len(data) > 0 {
		fd, restoreErr := openAt(int(dir.Fd()), ".process-pending", syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
		if errors.Is(restoreErr, os.ErrExist) {
			return err
		}
		if restoreErr == nil {
			f := os.NewFile(uintptr(fd), ".process-pending")
			_, restoreErr = f.Write(data)
			if restoreErr == nil {
				restoreErr = f.Sync()
			}
			restoreErr = errors.Join(restoreErr, f.Close())
		}
		if restoreErr == nil {
			restoreErr = renameAt(int(dir.Fd()), ".process-pending", "process.json")
		}
		if restoreErr != nil {
			restoreErr = errors.Join(restoreErr, unlinkAt(int(dir.Fd()), ".process-pending"))
		}
		err = errors.Join(err, restoreErr)
	}
	return err
}

var errNotRegular = errors.New("publication is not a bounded regular file")

// Lstat refuses special entries before open; no-follow and nonblocking open
// plus descriptor validation also refuse replacements during that race.
func readPublication(root *os.Root) ([]byte, error) {
	return readPublicationWith(root, nil)
}

// beforeOpen is a test seam for replacement after the no-follow metadata check.
func readPublicationWith(root *os.Root, beforeOpen func()) (data []byte, err error) {
	parent, err := root.OpenFile(".", os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, parent.Close()) }()
	info, err := root.Lstat("process.json")
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 4096 {
		return nil, errNotRegular
	}
	if beforeOpen != nil {
		beforeOpen()
	}
	fd, err := openAt(int(parent.Fd()), "process.json", syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), "process.json")
	info, err = f.Stat()
	if err == nil && (!info.Mode().IsRegular() || info.Size() > 4096) {
		err = errNotRegular
	}
	if err == nil {
		data, err = io.ReadAll(io.LimitReader(f, 4097))
	}
	return data, errors.Join(err, f.Close())
}

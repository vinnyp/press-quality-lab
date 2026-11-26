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
	dir, openErr := parent.OpenFile(name, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if openErr != nil {
		return removeWalkEntry(parent, name, b)
	}
	defer func() { err = errors.Join(err, dir.Close()) }()
	fd, readErr := openAt(int(dir.Fd()), "process.json", syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	var data []byte
	if readErr == nil {
		f := os.NewFile(uintptr(fd), "process.json")
		info, statErr := f.Stat()
		if statErr == nil && info.Mode().IsRegular() && info.Size() <= 4096 {
			data, readErr = io.ReadAll(io.LimitReader(f, 4097))
		}
		readErr = errors.Join(readErr, statErr, f.Close())
	}
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	err = removeWalkEntry(parent, name, b)
	if err != nil && len(data) > 0 {
		fd, restoreErr := openAt(int(dir.Fd()), "process.json", syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
		if errors.Is(restoreErr, os.ErrExist) {
			return err
		}
		if restoreErr == nil {
			f := os.NewFile(uintptr(fd), "process.json")
			_, restoreErr = f.Write(data)
			if restoreErr == nil {
				restoreErr = f.Sync()
			}
			restoreErr = errors.Join(restoreErr, f.Close())
		}
		err = errors.Join(err, restoreErr)
	}
	return err
}

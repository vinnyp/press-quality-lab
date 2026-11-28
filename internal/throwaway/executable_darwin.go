package throwaway

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// CopyExecutable prepares a private, byte-identical executable in its own directory.
// The caller fingerprints the returned file before its first use.
func CopyExecutable(run, source string) (string, error) {
	dir, err := os.MkdirTemp(run, "executable-")
	if err != nil {
		return "", err
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "program")
	in, err := os.OpenFile(source, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return "", err
	}
	info, statErr := in.Stat()
	if statErr != nil {
		return "", errors.Join(statErr, in.Close())
	}
	if !info.Mode().IsRegular() {
		return "", errors.Join(fmt.Errorf("executable is not regular"), in.Close())
	}
	out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return "", errors.Join(err, in.Close())
	}
	_, err = io.Copy(out, in)
	err = errors.Join(err, out.Chmod(info.Mode().Perm()))
	err = errors.Join(err, in.Close(), out.Close())
	if err != nil {
		return "", err
	}
	return path, nil
}

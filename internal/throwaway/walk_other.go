//go:build !darwin

package throwaway

import (
	"errors"
	"io"
	"os"
)

func removeWalkEntry(parent *os.Root, name string, b *budget) (err error) {
	if err = b.check(true); err != nil {
		return err
	}
	info, err := parent.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err = restoreMode(parent, name); err != nil {
			return err
		}
		root, openErr := parent.OpenRoot(name)
		if errors.Is(openErr, os.ErrNotExist) {
			return nil
		}
		if openErr != nil {
			return openErr
		}
		err = removeChildren(root, b)
		err = errors.Join(err, root.Close())
		if err != nil {
			return err
		}
	}
	if err = b.check(false); err != nil {
		return err
	}
	// Only leaves or directories whose children were already removed reach here.
	// In particular, symlinks and special entries are never opened.
	return parent.RemoveAll(name)
}
func removeChildren(root *os.Root, b *budget) (err error) {
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, dir.Close()) }()
	for {
		if err = b.check(false); err != nil {
			return err
		}
		entries, readErr := dir.ReadDir(1)
		for _, entry := range entries {
			if err = removeWalkEntry(root, entry.Name(), b); err != nil {
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

package throwaway

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"io"
	"os"
	"path"
	"path/filepath"
	"syscall"
	"time"
)

// FileFacts records metadata without opening the file's contents.
type FileFacts = observe.FileFacts

// NamedFile is a home-relative regular file supplied by a test harness.
type NamedFile struct {
	Name string
	Data []byte
}

// WriteHomeFiles writes only within a home through a rooted handle.
func WriteHomeFiles(home string, files []NamedFile) (err error) {
	root, err := os.OpenRoot(home)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	for _, file := range files {
		if !filepath.IsLocal(file.Name) || file.Name == "." {
			return fmt.Errorf("invalid home file name")
		}
		if err := root.MkdirAll(filepath.Dir(file.Name), 0700); err != nil {
			return err
		}
		f, err := root.OpenFile(file.Name, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
		if err != nil {
			return err
		}
		_, e := f.Write(file.Data)
		if e = errors.Join(e, f.Close()); e != nil {
			return e
		}
	}
	return nil
}

// EmptyHome creates the home and its private temporary directory.
func EmptyHome(home string) error { return os.MkdirAll(filepath.Join(home, ".pql-tmp"), 0700) }

// PlantGivenCanary replaces exactly one token and returns the written digest.
// It deliberately does not backdate: the before-snapshot must precede that.
func PlantGivenCanary(home, name, token, canary string) (digest string, err error) {
	root, err := os.OpenRoot(home)
	if err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	f, err := openHomeFile(root, name)
	if err != nil {
		return "", err
	}
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() {
		return "", errors.Join(errors.New("canary path is not regular"), e, f.Close())
	}
	data, e := io.ReadAll(f)
	if e = errors.Join(e, f.Close()); e != nil {
		return "", e
	}
	count := 0
	for start := 0; token != "" && start <= len(data)-len(token); start++ {
		if bytes.HasPrefix(data[start:], []byte(token)) {
			count++
		}
	}
	if count != 1 {
		return "", errors.New("token must occur once")
	}
	data = bytes.Replace(data, []byte(token), []byte(canary), 1)
	out, err := root.OpenFile(name, os.O_WRONLY|os.O_TRUNC|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return "", err
	}
	_, e = out.Write(data)
	if e = errors.Join(e, out.Close()); e != nil {
		return "", e
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

// BackdatePlanted changes metadata only, after the before-snapshot.
func BackdatePlanted(home, name string) (facts FileFacts, err error) {
	root, err := os.OpenRoot(home)
	if err != nil {
		return facts, err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	info, err := root.Lstat(name)
	if err != nil {
		return facts, err
	}
	if !info.Mode().IsRegular() {
		return facts, errors.New("planted file is not regular")
	}
	if err := root.Chtimes(name, info.ModTime().Add(-24*time.Hour), info.ModTime()); err != nil {
		return facts, err
	}
	return InspectFile(filepath.Join(home, name))
}

// AppendStoreControl appends one byte only to the declared regular store.
func AppendStoreControl(home, name string) (err error) {
	root, err := os.OpenRoot(home)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_APPEND|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() {
		return errors.Join(errors.New("store is not regular"), e, f.Close())
	}
	_, e = f.Write([]byte{0})
	return errors.Join(e, f.Close())
}

// StoreTarget reports QP9's nonempty target after QP7/QP8 exclusions.
func StoreTarget(snapshot HomeSnapshot, store string, bookkeeping []string) bool {
	if store == "" {
		return false
	}
	for _, e := range snapshot.Entries {
		excluded := e.Path == ".pql-tmp" || isBelow(e.Path, ".pql-tmp") || e.Path == store+"-shm" || (e.Path == store+"-wal" || e.Path == store+"-journal") && e.Size == 0
		for _, b := range bookkeeping {
			excluded = excluded || e.Path == b || isBelow(e.Path, b)
		}
		if !excluded && e.Kind == "file" && e.Size > 0 && !e.Unreadable {
			return true
		}
	}
	return false
}
func isBelow(name, parent string) bool {
	return name != parent && path.Clean(name) == name && len(name) > len(parent) && name[:len(parent)+1] == parent+"/"
}

package throwaway

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"syscall"
	"time"
)

// HomeEntry retains content identity only. Unreadable entries remain visible.
type HomeEntry struct {
	Path, Kind, Digest string
	Size               int64
	Unreadable         bool
}
type HomeSnapshot struct {
	Entries    []HomeEntry
	Unreadable bool
}
type homeOpener func(*os.Root, string) (*os.File, error)

func openHomeFile(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}

func homeNames(root *os.Root, reset bool) (entries []HomeEntry, err error) {
	b := budget{remaining: MaxWalkEntries, deadline: time.Now().Add(MaxWalkDuration), now: time.Now}
	var walk func(string) error
	walk = func(name string) error {
		if err := b.check(true); err != nil {
			return err
		}
		info, err := root.Lstat(name)
		if err != nil {
			return err
		}
		kind := "special"
		switch {
		case info.Mode().IsRegular():
			kind = "file"
		case info.IsDir():
			kind = "directory"
		case info.Mode()&os.ModeSymlink != 0:
			kind = "link"
		}
		if name != "." {
			entries = append(entries, HomeEntry{Path: name, Kind: kind, Size: info.Size()})
		}
		if kind != "directory" {
			return nil
		}
		if reset {
			if err := root.Chmod(name, 0700); err != nil {
				return err
			}
		}
		f, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_DIRECTORY, 0)
		if err != nil {
			return err
		}
		names := []string{}
		for {
			if err = b.check(false); err != nil {
				return errors.Join(err, f.Close())
			}
			batch, readErr := f.Readdirnames(1)
			names = append(names, batch...)
			if len(names) > b.remaining {
				return errors.Join(ErrEntryLimit, f.Close())
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				return errors.Join(readErr, f.Close())
			}
		}
		if err = f.Close(); err != nil {
			return err
		}
		sort.Strings(names)
		for _, child := range names {
			if err := walk(path.Join(name, child)); err != nil {
				return err
			}
		}
		return nil
	}
	err = walk(".")
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, err
}

// SnapshotHome reads names, kinds and regular-file contents, never link targets.
func SnapshotHome(home string) (HomeSnapshot, error) { return snapshotHome(home, openHomeFile) }
func snapshotHome(home string, open homeOpener) (out HomeSnapshot, err error) {
	root, err := os.OpenRoot(home)
	if err != nil {
		return out, err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	out.Entries, err = homeNames(root, true)
	if err != nil {
		return out, err
	}
	deadline := time.Now().Add(MaxWalkDuration)
	for i := range out.Entries {
		if time.Now().After(deadline) {
			return out, ErrTimeLimit
		}
		e := &out.Entries[i]
		if e.Kind != "file" {
			continue
		}
		if root.Chmod(e.Path, 0600) != nil {
			e.Unreadable = true
			out.Unreadable = true
			continue
		}
		f, err := open(root, e.Path)
		if err != nil {
			e.Unreadable = true
			out.Unreadable = true
			continue
		}
		h := sha256.New()
		_, readErr := io.Copy(h, deadlineReader{r: io.LimitReader(f, e.Size), deadline: deadline})
		readErr = errors.Join(readErr, f.Close())
		if readErr != nil {
			e.Unreadable = true
			out.Unreadable = true
			continue
		}
		e.Digest = hex.EncodeToString(h.Sum(nil))
	}
	return out, nil
}

// HomeDelta reports raw changes before the decision's QP7/QP8 exclusions.
type HomeDelta struct{ Path, Kind, Change, Digest string }

func CompareHomes(before, after HomeSnapshot) (changes []HomeDelta, unreadable bool) {
	unreadable = before.Unreadable || after.Unreadable
	old := map[string]HomeEntry{}
	seen := map[string]bool{}
	for _, e := range before.Entries {
		old[e.Path] = e
	}
	for _, e := range after.Entries {
		seen[e.Path] = true
		b, ok := old[e.Path]
		change := "modified"
		if !ok {
			change = "created"
		}
		if !ok || b.Kind != e.Kind || e.Kind == "file" && !e.Unreadable && !b.Unreadable && b.Digest != e.Digest || e.Kind == "link" || e.Kind == "special" {
			changes = append(changes, HomeDelta{e.Path, e.Kind, change, e.Digest})
		}
	}
	for _, e := range before.Entries {
		if !seen[e.Path] {
			changes = append(changes, HomeDelta{e.Path, e.Kind, "removed", ""})
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return changes, unreadable
}

// CopySeed creates a fresh home and copies only regular-file bytes and directories.
func CopySeed(seed, home string) (err error) {
	src, err := os.OpenRoot(seed)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, src.Close()) }()
	names, err := homeNames(src, false)
	if err != nil {
		return err
	}
	if err = os.Mkdir(home, 0700); err != nil {
		return err
	}
	dst, err := os.OpenRoot(home)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, dst.Close()) }()
	// Parents precede children in byte-wise path order.
	for _, e := range names {
		switch e.Kind {
		case "directory":
			if err = dst.Mkdir(e.Path, 0700); err != nil {
				return err
			}
		case "file":
			in, openErr := openHomeFile(src, e.Path)
			if openErr != nil {
				return openErr
			}
			out, openErr := dst.OpenFile(e.Path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if openErr != nil {
				return errors.Join(openErr, in.Close())
			}
			_, copyErr := io.Copy(out, in)
			if err = errors.Join(copyErr, in.Close(), out.Close()); err != nil {
				return err
			}
		default:
			return fmt.Errorf("seed entry %q is not regular", e.Path)
		}
	}
	return nil
}

// PlantCanary replaces a unique token in a fresh home and backdates access time.
func PlantCanary(home, name, token string) (canary string, err error) {
	root, err := os.OpenRoot(home)
	if err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	info, err := root.Lstat(name)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("canary path is not regular")
	}
	f, err := openHomeFile(root, name)
	if err != nil {
		return "", err
	}
	data, readErr := io.ReadAll(f)
	if err = errors.Join(readErr, f.Close()); err != nil {
		return "", err
	}
	return plantBytes(root, name, token, data)
}
func plantBytes(root *os.Root, name, token string, data []byte) (string, error) {
	// Count is checked before generating or writing a canary.
	if token == "" {
		return "", errors.New("empty token")
	}
	return replaceCanary(root, name, token, data, rand.Reader)
}
func replaceCanary(root *os.Root, name, token string, data []byte, random io.Reader) (string, error) {
	if bytes.Count(data, []byte(token)) != 1 {
		return "", errors.New("token must occur once")
	}
	var raw [16]byte
	if _, err := io.ReadFull(random, raw[:]); err != nil {
		return "", err
	}
	canary := "pqlcanary" + hex.EncodeToString(raw[:])
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_TRUNC|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return "", err
	}
	_, writeErr := f.Write(bytes.Replace(data, []byte(token), []byte(canary), 1))
	if err = errors.Join(writeErr, f.Close()); err != nil {
		return "", err
	}
	info, err := root.Stat(name)
	if err != nil {
		return "", err
	}
	err = root.Chtimes(name, info.ModTime().Add(-24*time.Hour), info.ModTime())
	return canary, err
}

// deadlineReader checks the inspection budget between bounded reads.
type deadlineReader struct {
	r        io.Reader
	deadline time.Time
}

func (r deadlineReader) Read(p []byte) (int, error) {
	if !time.Now().Before(r.deadline) {
		return 0, ErrTimeLimit
	}
	return r.r.Read(p[:min(len(p), 64<<10)])
}

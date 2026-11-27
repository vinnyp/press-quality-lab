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
	"strings"
	"syscall"
	"time"
)

// HomeEntry retains content identity only. Unreadable entries remain visible.
type HomeEntry struct {
	Path, Kind, Digest string
	Size               int64
	Unreadable         bool
	info               os.FileInfo
}
type HomeSnapshot struct {
	Entries    []HomeEntry
	Unreadable bool
}
type homeFile interface {
	io.Reader
	Stat() (os.FileInfo, error)
	Close() error
}
type homeOpener func(*os.Root, string) (homeFile, error)
type homeHooks struct {
	entryLimit int
	open       homeOpener
	chmod      func(*os.Root, string, os.FileMode) error
	now        func() time.Time
}

func defaultHomeHooks(open homeOpener) homeHooks {
	return homeHooks{open: open, chmod: func(r *os.Root, n string, m os.FileMode) error { return r.Chmod(n, m) }, now: time.Now, entryLimit: MaxWalkEntries}
}

const MaxHomeDepth = 256

var errHomeUnreadable = errors.New("home entry unreadable")
var errHomeDepth = errors.New("home depth bound reached")

func homeBound(err error) bool {
	return errors.Is(err, ErrEntryLimit) || errors.Is(err, ErrTimeLimit) || errors.Is(err, errHomeDepth)
}
func linked(info os.FileInfo) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	return !ok || st.Nlink > 1
}
func checkedHomeFile(root *os.Root, name string, h homeHooks) (homeFile, error) {
	before, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || linked(before) {
		return nil, errHomeUnreadable
	}
	if err := h.chmod(root, name, 0600); err != nil {
		return nil, err
	}
	f, err := h.open(root, name)
	if err != nil {
		return nil, err
	}
	after, err := f.Stat()
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) || linked(after) {
		return nil, errors.Join(errHomeUnreadable, err, f.Close())
	}
	return f, nil
}

// withParent resolves one component at a time, never re-resolving a full path.
func withParent(root *os.Root, name string, fn func(*os.Root, string) error) (err error) {
	parts := strings.Split(name, "/")
	parent := root
	for _, part := range parts[:len(parts)-1] {
		next, e := parent.OpenRoot(part)
		if parent != root {
			e = errors.Join(e, parent.Close())
		}
		if e != nil {
			if next != nil {
				e = errors.Join(e, next.Close())
			}
			return e
		}
		parent = next
	}
	if parent != root {
		defer func() { err = errors.Join(err, parent.Close()) }()
	}
	return fn(parent, parts[len(parts)-1])
}

func openHomeFile(root *os.Root, name string) (homeFile, error) {
	return root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}

func homeNames(root *os.Root, reset bool) ([]HomeEntry, error) {
	return homeNamesWith(root, reset, defaultHomeHooks(openHomeFile))
}
func homeNamesWith(root *os.Root, reset bool, h homeHooks) (entries []HomeEntry, err error) {
	b := budget{remaining: h.entryLimit, deadline: h.now().Add(MaxWalkDuration), now: h.now}
	var walk func(*os.Root, string, int) error
	walk = func(parent *os.Root, prefix string, depth int) (err error) {
		if depth > MaxHomeDepth {
			return errHomeDepth
		}
		if err := b.check(false); err != nil {
			return err
		}
		if reset {
			if err := h.chmod(parent, ".", 0700); err != nil {
				return errHomeUnreadable
			}
		}
		f, err := parent.OpenFile(".", os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_DIRECTORY, 0)
		if err != nil {
			return errHomeUnreadable
		}
		names := []string{}
		for {
			if e := b.check(false); e != nil {
				return errors.Join(e, f.Close())
			}
			batch, e := f.Readdirnames(1)
			names = append(names, batch...)
			if len(names) > b.remaining {
				return errors.Join(ErrEntryLimit, f.Close())
			}
			if e == io.EOF {
				break
			}
			if e != nil {
				return errors.Join(errHomeUnreadable, f.Close())
			}
		}
		if err := f.Close(); err != nil {
			return errors.Join(errHomeUnreadable, err)
		}
		sort.Strings(names)
		var unreadable error
		for _, name := range names {
			if err := b.check(true); err != nil {
				return errors.Join(unreadable, err)
			}
			info, e := parent.Lstat(name)
			if e != nil {
				unreadable = errHomeUnreadable
				continue
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
			full := path.Join(prefix, name)
			entries = append(entries, HomeEntry{Path: full, Kind: kind, Size: info.Size(), info: info})
			if kind != "directory" {
				continue
			}
			child, e := parent.OpenRoot(name)
			if e != nil {
				unreadable = errHomeUnreadable
				continue
			}
			e = walk(child, full, depth+1)
			e = errors.Join(e, child.Close())
			if homeBound(e) {
				return errors.Join(unreadable, e)
			}
			if e != nil {
				unreadable = errHomeUnreadable
			}
		}
		return unreadable
	}
	err = walk(root, "", 0)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, err
}

// SnapshotHome reads names, kinds and regular-file contents, never link targets.
func SnapshotHome(home string) (HomeSnapshot, error) { return snapshotHome(home, openHomeFile) }
func snapshotHome(home string, open homeOpener) (HomeSnapshot, error) {
	return snapshotHomeWith(home, defaultHomeHooks(open))
}
func snapshotHomeWith(home string, hooks homeHooks) (out HomeSnapshot, err error) {
	root, err := os.OpenRoot(home)
	if err != nil {
		out.Unreadable = true
		return out, nil
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	out.Entries, err = homeNamesWith(root, true, hooks)
	if err != nil {
		out.Unreadable = true
		err = nil
	}
	deadline := hooks.now().Add(MaxWalkDuration)
	for i := range out.Entries {
		if !hooks.now().Before(deadline) {
			out.Unreadable = true
			break
		}
		e := &out.Entries[i]
		if e.Kind != "file" {
			continue
		}
		readErr := withParent(root, e.Path, func(parent *os.Root, name string) error {
			f, err := checkedHomeFile(parent, name, hooks)
			if err != nil {
				return err
			}
			info, statErr := f.Stat()
			if statErr != nil || !os.SameFile(e.info, info) {
				return errors.Join(errHomeUnreadable, statErr, f.Close())
			}
			h := sha256.New()
			_, readErr := io.Copy(h, io.LimitReader(deadlineReader{r: f, deadline: deadline, now: hooks.now}, e.Size))
			readErr = errors.Join(readErr, f.Close())
			if readErr == nil && !hooks.now().Before(deadline) {
				readErr = ErrTimeLimit
			}
			if readErr == nil {
				e.Digest = hex.EncodeToString(h.Sum(nil))
			}
			return readErr
		})
		if readErr != nil {
			e.Unreadable = true
			out.Unreadable = true
		}
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
	count := 0
	for start := 0; start <= len(data)-len(token); start++ {
		if bytes.HasPrefix(data[start:], []byte(token)) {
			count++
		}
	}
	if count != 1 {
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
	now      func() time.Time
}

func (r deadlineReader) Read(p []byte) (int, error) {
	now := r.now
	if now == nil {
		now = time.Now
	}
	if !now().Before(r.deadline) {
		return 0, ErrTimeLimit
	}
	return r.r.Read(p[:min(len(p), 64<<10)])
}

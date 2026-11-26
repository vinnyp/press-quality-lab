package throwaway

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

// Process identifies an engine instance, including protection against PID reuse.
type Process struct {
	PID       int
	StartTime string
}

// ProcessLookup returns a live process's start time. Only os.ErrNotExist means
// the process is gone; permission failures and other uncertainty fail closed.
type ProcessLookup func(pid int) (string, error)

// Publish atomically marks a workspace as an engine-owned run. The caller must
// supply the actual process start time, not the time this file is written.
func (w *Workspace) Publish(p Process) (err error) {
	if p.PID <= 0 || p.StartTime == "" {
		return errors.New("process identity required")
	}
	if err := privateParent(filepath.Dir(w.path)); err != nil {
		return err
	}
	root, err := os.OpenRoot(w.path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	f, err := root.OpenFile(".process-pending", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	data := strictjson.Write(strictjson.Object{"pid": p.PID, "start_time": p.StartTime})
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	err = errors.Join(err, f.Close())
	if err != nil {
		return err
	}
	return root.Rename(".process-pending", "process.json")
}

// Sweep checks the private parent and lists only direct, non-link candidates.
// All workspace inspection and removal stays in this package. The lookup is
// injected so the platform owner can supply kernel process start times.
func Sweep(parent string, lookup ProcessLookup) (err error) {
	if lookup == nil {
		return errors.New("process lookup required")
	}
	if err := privateParent(parent); err != nil {
		return err
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, dir.Close()) }()
	b := &budget{remaining: MaxWalkEntries, deadline: time.Now().Add(MaxWalkDuration), now: time.Now}
	for {
		if err = b.check(false); err != nil {
			return err
		}
		entries, readErr := dir.ReadDir(1)
		for _, entry := range entries {
			if err = b.check(true); err != nil {
				return err
			}
			name := entry.Name()
			if !strings.HasPrefix(name, "pql-run-") {
				continue
			}
			candidate, statErr := root.Lstat(name)
			if statErr != nil {
				return statErr
			}
			if !candidate.IsDir() {
				continue
			}
			process, found, processErr := published(root, name)
			if processErr != nil {
				return fmt.Errorf("inspect workspace %q: %w", filepath.Join(parent, name), processErr)
			}
			if !found {
				continue
			}
			start, lookupErr := lookup(process.PID)
			if lookupErr != nil && !errors.Is(lookupErr, os.ErrNotExist) {
				return lookupErr
			}
			if lookupErr == nil && start == process.StartTime {
				continue
			}
			if err = removeEntry(root, name, b); err != nil {
				return fmt.Errorf("remove workspace %q: %w", filepath.Join(parent, name), err)
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
func published(parent *os.Root, name string) (p Process, found bool, err error) {
	root, err := parent.OpenRoot(name)
	if err != nil {
		return p, false, err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	info, err := root.Lstat("process.json")
	if errors.Is(err, os.ErrNotExist) {
		return p, false, nil
	}
	if err != nil {
		return p, false, err
	}
	if !info.Mode().IsRegular() || info.Size() > 4096 {
		return p, false, nil
	}
	f, err := root.Open("process.json")
	if err != nil {
		return p, false, err
	}
	data, err := io.ReadAll(io.LimitReader(f, 4097))
	err = errors.Join(err, f.Close())
	if err != nil {
		return p, false, err
	}
	v, valid := strictjson.Read(data, strictjson.Bounds{Bytes: 4096, Depth: 2, NumberChars: 20, ExponentDigits: 1}, nil)
	if !valid {
		return p, false, nil
	}
	obj, ok := v.(strictjson.Object)
	if !ok || len(obj) != 2 {
		return p, false, nil
	}
	pid, ok := obj["pid"].(strictjson.Number)
	if !ok {
		return p, false, nil
	}
	p.PID, err = strconv.Atoi(string(pid))
	if err != nil {
		return Process{}, false, nil
	}
	p.StartTime, ok = obj["start_time"].(string)
	return p, ok && p.PID > 0 && p.StartTime != "", nil
}

func privateParent(parent string) error {
	info, err := os.Lstat(parent)
	if err != nil {
		return err
	}
	owner, ok := info.Sys().(*syscall.Stat_t)
	if !info.IsDir() || info.Mode().Perm() != 0700 || !ok || owner.Uid != uint32(os.Getuid()) {
		return errors.New("workspace parent must be user-owned mode 0700 directory")
	}
	return nil
}

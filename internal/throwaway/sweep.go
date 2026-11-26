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
func (w *Workspace) Publish(p Process) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.publish(p, w.groups); err != nil {
		return err
	}
	w.owner = p
	return nil
}

func (w *Workspace) publish(p Process, groups []Process) (err error) {
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
	obj := strictjson.Object{"pid": p.PID, "start_time": p.StartTime}
	if len(groups) > 0 {
		a := strictjson.Array{}
		for _, g := range groups {
			a = append(a, strictjson.Object{"pid": g.PID, "start_time": g.StartTime})
		}
		obj["groups"] = a
	}
	data := strictjson.Write(obj)
	if len(data) > 4096 {
		return errors.Join(errors.New("process publication exceeds bound"), f.Close())
	}
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
func Sweep(parent string, lookup ProcessLookup, options ...SweepOptions) (err error) {
	opts := SweepOptions{End: EndGroup, Report: os.Stderr}
	if len(options) > 0 {
		if options[0].End != nil {
			opts.End = options[0].End
		}
		if options[0].Report != nil {
			opts.Report = options[0].Report
		}
	}
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
			process, groups, found, processErr := published(root, name)
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
			if err = endRecorded(groups, lookup, opts); err != nil {
				return fmt.Errorf("end groups for workspace %q: %w", filepath.Join(parent, name), err)
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
func published(parent *os.Root, name string) (p Process, groups []Process, found bool, err error) {
	root, err := parent.OpenRoot(name)
	if err != nil {
		return p, nil, false, err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	info, err := root.Lstat("process.json")
	if errors.Is(err, os.ErrNotExist) {
		return p, nil, false, nil
	}
	if err != nil {
		return p, nil, false, err
	}
	if !info.Mode().IsRegular() || info.Size() > 4096 {
		return p, nil, false, nil
	}
	f, err := root.Open("process.json")
	if err != nil {
		return p, nil, false, err
	}
	data, err := io.ReadAll(io.LimitReader(f, 4097))
	err = errors.Join(err, f.Close())
	if err != nil {
		return p, nil, false, err
	}
	v, valid := strictjson.Read(data, strictjson.Bounds{Bytes: 4096, Depth: 4, NumberChars: 20, ExponentDigits: 1}, nil)
	if !valid {
		return p, nil, false, nil
	}
	obj, ok := v.(strictjson.Object)
	if !ok || (len(obj) != 2 && len(obj) != 3) {
		return p, nil, false, nil
	}
	pid, ok := obj["pid"].(strictjson.Number)
	if !ok {
		return p, nil, false, nil
	}
	p.PID, err = strconv.Atoi(string(pid))
	if err != nil {
		return Process{}, nil, false, nil
	}
	p.StartTime, ok = obj["start_time"].(string)
	if !ok || p.PID <= 0 || p.StartTime == "" {
		return p, nil, false, nil
	}
	if raw, exists := obj["groups"]; exists {
		a, ok := raw.(strictjson.Array)
		if !ok {
			return p, nil, false, nil
		}
		seen := map[int]bool{}
		for _, v := range a {
			o, ok := v.(strictjson.Object)
			if !ok || len(o) != 2 {
				return p, nil, false, nil
			}
			n, ok := o["pid"].(strictjson.Number)
			if !ok {
				return p, nil, false, nil
			}
			id, e := strconv.Atoi(string(n))
			start, ok := o["start_time"].(string)
			if e != nil || !ok || id <= 0 || start == "" || seen[id] {
				return p, nil, false, nil
			}
			seen[id] = true
			groups = append(groups, Process{PID: id, StartTime: start})
		}
	} else if len(obj) != 2 {
		return p, nil, false, nil
	}
	return p, groups, true, nil
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

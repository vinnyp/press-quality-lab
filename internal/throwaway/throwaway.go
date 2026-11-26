// Package throwaway owns per-run workspace creation and bounded cleanup.
// Callers must end subject processes before cleanup. Darwin cleanup uses
// parent-relative no-follow operations for flags, ACLs, modes and removal.
package throwaway

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MaxWalkEntries bounds metadata work, including the workspace directory.
const MaxWalkEntries = 100000

// MaxWalkDuration bounds elapsed walk time between filesystem operations.
const MaxWalkDuration = 30 * time.Second

var ErrEntryLimit = errors.New("throwaway entry limit reached")
var ErrTimeLimit = errors.New("throwaway time limit reached")

// Workspace is a private directory. Its path remains available after failure
// so the exit funnel can report it and a subsequent sweep can retry removal.
type Workspace struct {
	path    string
	mu      sync.Mutex
	owner   Process
	groups  []Process
	closing bool
}

// Create makes a mode-0700 workspace below the caller's chosen parent.
func Create(parent string) (*Workspace, error) {
	path, err := os.MkdirTemp(parent, "pql-run-")
	if err != nil {
		return nil, err
	}
	return &Workspace{path: path}, nil
}
func (w *Workspace) Path() string { return w.path }

// Remove returns any incomplete cleanup as an error (QE20, errata E-11).
// The caller must turn that error into exit 70, even after a verdict exists.
func (w *Workspace) Remove() error {
	return w.remove(&budget{remaining: MaxWalkEntries, deadline: time.Now().Add(MaxWalkDuration), now: time.Now})
}

type budget struct {
	// beforeDirectoryOpen is a deterministic test seam, scoped to one walk.
	beforeDirectoryOpen func(int, string)
	remaining           int
	deadline            time.Time
	now                 func() time.Time
}

func (b *budget) check(entry bool) error {
	if !b.now().Before(b.deadline) {
		return ErrTimeLimit
	}
	if entry {
		if b.remaining <= 0 {
			return ErrEntryLimit
		}
		b.remaining--
	}
	return nil
}
func restoreMode(root *os.Root, name string) error { return root.Chmod(name, 0700) }
func (w *Workspace) remove(b *budget) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("remove workspace %q: %w", w.path, err)
		}
	}()
	parent, err := os.OpenRoot(filepath.Dir(w.path))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, parent.Close()) }()
	return removeEntry(parent, filepath.Base(w.path), b)
}

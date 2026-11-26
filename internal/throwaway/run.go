package throwaway

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Begin sweeps the canonical temporary parent, then publishes this run. Only
// throwaway accesses this directory; durable records never live beneath it.
func Begin() (*Workspace, error) {
	temp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return nil, err
	}
	parent := filepath.Join(temp, fmt.Sprintf("pql-workspaces-%d", os.Getuid()))
	if err := os.Mkdir(parent, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	if err := Sweep(parent, ProcessStart); err != nil {
		return nil, err
	}
	start, err := ProcessStart(os.Getpid())
	if err != nil {
		return nil, err
	}
	w, err := Create(parent)
	if err != nil {
		return nil, err
	}
	if err := w.Publish(Process{PID: os.Getpid(), StartTime: start}); err != nil {
		return nil, errors.Join(err, w.Remove())
	}
	return w, nil
}

package throwaway

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// SweepOptions supplies process effects and the diagnostic destination. End
// must wait until the verified group has no executing members before returning.
type SweepOptions struct {
	End    func(int) error
	Report io.Writer
}

// RecordGroup atomically records a live leader before handing an invocation to
// its caller. The caller retains the leader unreaped until ForgetGroup.
func (w *Workspace) RecordGroup(p Process) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closing {
		return errors.New("workspace is ending its groups")
	}
	if p.PID <= 0 || p.StartTime == "" {
		return errors.New("group identity required")
	}
	for _, g := range w.groups {
		if g.PID == p.PID {
			return errors.New("group already recorded")
		}
	}
	groups := append(append([]Process(nil), w.groups...), p)
	if err := w.publish(w.owner, groups); err != nil {
		return err
	}
	w.groups = groups
	return nil
}

// ForgetGroup is called only after the invocation ended its group and reaped.
func (w *Workspace) ForgetGroup(pid int) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	groups := make([]Process, 0, len(w.groups))
	for _, g := range w.groups {
		if g.PID != pid {
			groups = append(groups, g)
		}
	}
	if err := w.publish(w.owner, groups); err != nil {
		return err
	}
	w.groups = groups
	return nil
}

func endRecorded(groups []Process, lookup ProcessLookup, opts SweepOptions) error {
	var problems error
	for _, g := range groups {
		start, err := lookup(g.PID)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			problems = errors.Join(problems, err)
			continue
		}
		if err != nil || start != g.StartTime {
			if _, err := fmt.Fprintf(opts.Report, "cannot verify recorded process group %d; not signalled\n", g.PID); err != nil {
				problems = errors.Join(problems, err)
			}
			continue
		}
		if err := opts.End(g.PID); err != nil {
			problems = errors.Join(problems, err)
		}
	}
	return problems
}

// EndGroups applies the same fail-safe identity rule as the startup sweep.
func (w *Workspace) EndGroups(report io.Writer) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closing = true
	if report == nil {
		report = os.Stderr
	}
	return endRecorded(w.groups, ProcessStart, SweepOptions{End: EndGroup, Report: report})
}

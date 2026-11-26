package confine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/throwaway"
)

func TestSelfTestChangedCopyDoesNotStartDecoy(t *testing.T) {
	// QH8: an unconfined decoy must not execute a changed private launcher.
	// The mutant is a short owned script, so even a broken guard never starts
	// sandbox-exec or leaves a helper alive.
	w, err := throwaway.BeginAt(filepath.Join(t.TempDir(), "runs"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := w.Remove(); err != nil {
			t.Error(err)
		}
	}()
	c, err := NewCopies(w.Path(), "/bin/sh", "/bin/sh")
	if err != nil {
		t.Fatal(err)
	}
	files, err := w.PrepareSelfTest()
	if err != nil {
		t.Fatal(err)
	}
	launcher, program := c.CopyPaths()
	if err := os.WriteFile(launcher, []byte("#!/bin/sh\necho started > decoy_started\n"), 0700); err != nil {
		t.Fatal(err)
	}
	runs, err := runProfileSelfTest(c, Invocation{Workspace: w}, launcher, program, files)
	if err != nil || len(runs) != 1 || !runs[0].CopyChanged {
		t.Fatalf("runs=%+v error=%v", runs, err)
	}
	if _, err := os.Stat(filepath.Join(files.Home, "decoy_started")); !os.IsNotExist(err) {
		t.Fatalf("changed decoy executed: %v", err)
	}
}

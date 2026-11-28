package grade

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/throwaway"
	"golang.org/x/sys/unix"
)

func TestImmutableHardLinkCleanup(t *testing.T) {
	// QE20, KTD20, E-11, E-19: an unreachable subject-created hard link stays for retry.
	o, d, _ := fixture(t)
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("kept"), 0400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unix.Chflags(outside, 0); err != nil {
			t.Error(err)
		}
	})
	var w *throwaway.Workspace
	var before unix.Stat_t
	begin := d.Workspace
	d.Workspace = func() (*throwaway.Workspace, error) {
		var err error
		w, err = begin()
		if err != nil {
			return w, err
		}
		home := filepath.Join(w.Path(), "home")
		if err := os.Mkdir(home, 0700); err != nil {
			return w, err
		}
		if err := os.Link(outside, filepath.Join(home, "leaf")); err != nil {
			return w, err
		}
		if err := unix.Chflags(outside, unix.UF_IMMUTABLE); err != nil {
			return w, err
		}
		return w, unix.Stat(outside, &before)
	}
	var output, diagnostic bytes.Buffer
	o.JSON = true
	d.Stderr = &diagnostic
	code, err := Run(o, d, &output)
	if code != 70 || err == nil {
		t.Fatalf("cleanup: %d %v", code, err)
	}
	if !strings.Contains(diagnostic.String(), "cleanup failed; directory left behind: "+Escape(w.Path())+"\n") {
		t.Fatalf("directory unnamed: %q", output.String())
	}
	var report bytes.Buffer
	if err := throwaway.Sweep(filepath.Dir(w.Path()), func(int) (string, error) { return "", os.ErrNotExist }, throwaway.SweepOptions{Report: &report}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report.String(), w.Path()) {
		t.Fatalf("retry unnamed: %q", report.String())
	}
	if _, err := os.Stat(w.Path()); err != nil {
		t.Fatalf("retry lost immutable home: %v", err)
	}
	var after unix.Stat_t
	if err := unix.Stat(outside, &after); err != nil {
		t.Fatal(err)
	}
	if before.Mode != after.Mode || before.Flags != after.Flags || before.Ctim != after.Ctim || before.Nlink != after.Nlink {
		t.Fatalf("outside metadata changed: before=%+v after=%+v", before, after)
	}
	data, err := os.ReadFile(outside)
	if err != nil || string(data) != "kept" {
		t.Fatalf("outside contents: %q %v", data, err)
	}
}

package confine

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/observe"
)

func TestCopiesPerExecutable(t *testing.T) {
	// QH8, E-55: each executable has one persistent copy, checked on every use.
	dir := t.TempDir()
	program := filepath.Join(dir, "subject")
	computation := filepath.Join(dir, "computation")
	for _, name := range []string{program, computation} {
		if err := os.WriteFile(name, []byte("synthetic"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	c, err := NewCopies(t.TempDir(), program, program)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Unchanged() {
		t.Fatal("initial grade-wide copy check")
	}
	paths := testPaths(t)
	sentinel := errors.New("launch seam")
	launched := 0
	invoke := func(executable string) (Process, error) {
		selected := paths
		if executable == "" {
			values := map[string]string{}
			for _, name := range commonNames {
				values[name] = paths.Values[name]
			}
			selected = Paths{Values: values}
		}
		return c.startWith(Invocation{Workspace: fakeWorkspace{}, Computation: executable != "", Executable: executable, Paths: selected}, func(_ string, argv []string, _ string, _ []string, _ string) (Process, error) {
			launched++
			if systemExecutable(executable) {
				found := false
				for _, arg := range argv {
					found = found || arg == "BIN="+executable
				}
				if !found {
					t.Fatalf("in-place BIN missing: %v", argv)
				}
			}
			return nil, sentinel
		})
	}
	for range 2 {
		if p, err := invoke(computation); p != nil || !errors.Is(err, sentinel) {
			t.Fatalf("healthy computation %v %v", p, err)
		}
	}
	if len(c.programs) != 2 {
		t.Fatalf("copy count %d", len(c.programs))
	}
	copied := c.programs[computation].path
	if err := os.WriteFile(copied, []byte("changed"), 0700); err != nil {
		t.Fatal(err)
	}
	if c.Unchanged() {
		t.Fatal("grade-wide check missed changed computation copy")
	}
	p, err := invoke(computation)
	if err != nil || p == nil {
		t.Fatalf("changed computation %v %v", p, err)
	}
	run, err := p.Wait()
	if err != nil || !reflect.DeepEqual(run, observe.Run{CopyChanged: true}) {
		t.Fatalf("refusal %+v %v", run, err)
	}
	if launched != 2 || c.programs[computation].path != copied {
		t.Fatal("changed copy repaired or launched")
	}
	if p, err := invoke(""); p != nil || !errors.Is(err, sentinel) {
		t.Fatalf("unrelated subject %v %v", p, err)
	}
	if p, err := invoke("/usr/bin/true"); p != nil || !errors.Is(err, sentinel) {
		t.Fatalf("system tool %v %v", p, err)
	}
	if len(c.programs) != 2 {
		t.Fatal("system tool copied")
	}
	if err := os.WriteFile(c.launcher.path, []byte(strings.Repeat("x", 20)), 0700); err != nil {
		t.Fatal(err)
	}
	p, err = invoke("/usr/bin/true")
	if err != nil || p == nil {
		t.Fatalf("changed launcher %v %v", p, err)
	}
	run, err = p.Wait()
	if err != nil || !reflect.DeepEqual(run, observe.Run{CopyChanged: true}) {
		t.Fatalf("launcher refusal %+v %v", run, err)
	}
}

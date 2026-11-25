package throwaway

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPublishAndSweep(t *testing.T) {
	// QE20, KTD5, KTD20: only published, demonstrably stale runs are removed.
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	stale, err := Create(parent)
	if err != nil {
		t.Fatal(err)
	}
	if err := stale.Publish(Process{PID: 41, StartTime: "old"}); err != nil {
		t.Fatal(err)
	}
	live, err := Create(parent)
	if err != nil {
		t.Fatal(err)
	}
	if err := live.Publish(Process{PID: 42, StartTime: "live"}); err != nil {
		t.Fatal(err)
	}
	reused, err := Create(parent)
	if err != nil {
		t.Fatal(err)
	}
	if err := reused.Publish(Process{PID: 43, StartTime: "old"}); err != nil {
		t.Fatal(err)
	}
	unpublished, err := Create(parent)
	if err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(parent, "foreign")
	if err := os.Mkdir(foreign, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "process.json"), []byte("{\"pid\":41,\"start_time\":\"old\"}"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(parent, "pql-run-link")
	if err := os.Symlink("foreign", link); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(live.Path(), "process.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{\"pid\":42,\"start_time\":\"live\"}\n" {
		t.Fatalf("publication: %s", data)
	}
	info, err := os.Stat(filepath.Join(live.Path(), "process.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
	err = Sweep(parent, func(pid int) (string, error) {
		switch pid {
		case 41:
			return "", os.ErrNotExist
		case 42:
			return "live", nil
		case 43:
			return "new", nil
		}
		t.Fatalf("unexpected pid %d", pid)
		return "", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{stale.Path(), reused.Path()} {
		if _, err := os.Lstat(p); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stale remains %s: %v", p, err)
		}
	}
	for _, p := range []string{live.Path(), unpublished.Path(), foreign, link} {
		if _, err := os.Lstat(p); err != nil {
			t.Fatalf("protected entry %s: %v", p, err)
		}
	}
}
func TestSweepUnknownProcess(t *testing.T) {
	// QE20, E-11: inability to establish staleness is an error, never removal.
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	w, err := Create(parent)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Publish(Process{PID: 41, StartTime: "old"}); err != nil {
		t.Fatal(err)
	}
	want := errors.New("lookup failed")
	if err := Sweep(parent, func(int) (string, error) { return "", want }); !errors.Is(err, want) {
		t.Fatalf("lookup: %v", err)
	}
	if _, err := os.Stat(w.Path()); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidPublication(t *testing.T) {
	// QE20, KTD20: a malformed, linked, or absent identity is not ownership proof.
	for _, row := range []struct{ name, contents string }{
		{"Malformed", "broken"},
		{"Empty", "{}"},
		{"BadPID", "{\"pid\":0,\"start_time\":\"old\"}"},
		{"BadStart", "{\"pid\":41,\"start_time\":false}"},
		{"Extra", "{\"pid\":41,\"start_time\":\"old\",\"extra\":true}"},
		{"Link", ""},
	} {
		t.Run(row.name, func(t *testing.T) {
			parent := t.TempDir()
			if err := os.Chmod(parent, 0700); err != nil {
				t.Fatal(err)
			}
			w, err := Create(parent)
			if err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(w.Path(), "process.json")
			if row.name == "Link" {
				outside := filepath.Join(parent, "outside")
				if err := os.WriteFile(outside, []byte("{\"pid\":41,\"start_time\":\"old\"}"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, target); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(target, []byte(row.contents), 0600); err != nil {
				t.Fatal(err)
			}
			if err := Sweep(parent, func(int) (string, error) { t.Fatal("lookup for unowned directory"); return "", os.ErrNotExist }); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(w.Path()); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestPublishRequiresIdentity(t *testing.T) {
	// QE20: publication includes both the process id and its kernel start time.
	w, err := Create(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []Process{{PID: 0, StartTime: "old"}, {PID: 41}} {
		if err := w.Publish(p); err == nil {
			t.Fatal("invalid identity published")
		}
	}
}
func TestSweepRequiresPrivateParent(t *testing.T) {
	// QE20, KTD5, KTD20: a parent link or public directory is not a workspace root.
	parent := t.TempDir()
	if err := os.Chmod(parent, 0755); err != nil {
		t.Fatal(err)
	}
	lookup := func(int) (string, error) { return "", os.ErrNotExist }
	if err := Sweep(parent, lookup); err == nil {
		t.Fatal("public parent accepted")
	}
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(parent, link); err != nil {
		t.Fatal(err)
	}
	if err := Sweep(link, lookup); err == nil {
		t.Fatal("parent link accepted")
	}
	if err := Sweep(parent, nil); err == nil {
		t.Fatal("nil lookup accepted")
	}
}

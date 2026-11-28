package confine

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSystemExecutable(t *testing.T) {
	// QH8, E-55: only canonical paths below the two system directories run in place.
	for _, row := range []struct {
		name, path string
		want       bool
	}{
		{"Bin", "/bin/echo", true},
		{"UsrBin", "/usr/bin/true", true},
		{"Relative", "computations/value", false},
		{"Other", "/usr/local/bin/value", false},
		{"Escape", "/bin/../other/value", false},
		{"Prefix", "/usr/binary/value", false},
	} {
		t.Run(row.name, func(t *testing.T) {
			if systemExecutable(row.path) != row.want {
				t.Fatal(row.path)
			}
		})
	}
}

func TestRoundAPlantedDirectories(t *testing.T) {
	// QH11, QH27: home-level files need no ancestor rule; nested files protect each directory below HOME.
	home := t.TempDir()
	for _, row := range []struct {
		name, file string
		want       []string
	}{
		{"Home", "token", nil},
		{"Nested", "a/b/token", []string{filepath.Join(home, "a"), filepath.Join(home, "a", "b")}},
	} {
		t.Run(row.name, func(t *testing.T) {
			got, err := plantedDirectories(home, filepath.Join(home, row.file))
			if err != nil || !reflect.DeepEqual(got, row.want) {
				t.Fatalf("%v %v", got, err)
			}
		})
	}
}

func TestRoundAControlsFromProbe(t *testing.T) {
	// QH6, QH8: a changed copy invalidates copy integrity and never supplies sandbox-exec exit evidence.
	for _, row := range []struct {
		name string
		raw  observe.Run
		want observe.Controls
	}{
		{"Exit", observe.Run{End: observe.End{Code: 64}}, observe.Controls{PrivateCopyUnchanged: true, SandboxExecExit: intPointer(64)}},
		{"Marker", observe.Run{Markers: observe.Markers{Confinement: true}}, observe.Controls{PrivateCopyUnchanged: true, ConfinementApplied: true}},
		{"Changed", observe.Run{CopyChanged: true}, observe.Controls{}},
		{"Signal", observe.Run{End: observe.End{Signal: 9}}, observe.Controls{PrivateCopyUnchanged: true}},
	} {
		t.Run(row.name, func(t *testing.T) {
			if got := ControlsFromProbe(row.raw); !reflect.DeepEqual(got, row.want) {
				t.Fatalf("%+v", got)
			}
		})
	}
}
func intPointer(v int) *int { return &v }

func TestRoundAResolvedExecutable(t *testing.T) {
	// QH8, E-55: symlink resolution cannot enlarge the two system directories; only regular files run in place.
	info, err := os.Stat(filepath.Join(t.TempDir(), "missing"))
	if !os.IsNotExist(err) {
		t.Fatal(info, err)
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "tool")
	if err := os.WriteFile(file, []byte("synthetic"), 0700); err != nil {
		t.Fatal(err)
	}
	regular, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	directory, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		name, resolved string
		info           os.FileInfo
		want           string
		cause          string
	}{
		{"InPlace", "/usr/bin/printf", regular, "/usr/bin/printf", ""},
		{"ResolvedOtherSystem", "/bin/printf", regular, "/bin/printf", ""},
		{"Escape", "/System/tool", regular, "", "system executable outside allowed directories"},
		{"Directory", "/usr/bin/printf", directory, "", "system executable is not regular"},
	} {
		t.Run(row.name, func(t *testing.T) {
			called := ""
			got, err := resolvedSystemExecutable("/usr/bin/printf", func(name string) (string, error) { called = name; return row.resolved, nil }, func(name string) (os.FileInfo, error) {
				if name != row.resolved {
					t.Fatal(name)
				}
				return row.info, nil
			})
			if called != "/usr/bin/printf" || got != row.want || row.cause == "" && err != nil || row.cause != "" && (err == nil || err.Error() != row.cause) {
				t.Fatalf("%q %v", got, err)
			}
		})
	}
}

func TestRoundAAbsentPlanted(t *testing.T) {
	// QH11: a writable home name can never alias the absent profile parameter.
	parent := t.TempDir()
	home := filepath.Join(parent, "home")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	for _, create := range []bool{false, true} {
		if create {
			if err := os.WriteFile(filepath.Join(home, "absent_planted"), []byte("subject"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		w, name, err := absentPlanted(parent)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := w.Remove(); err != nil {
				t.Error(err)
			}
		})
		if filepath.Dir(name) == home {
			t.Fatal("stand-in inside home")
		}
		if _, err := os.Lstat(name); !os.IsNotExist(err) {
			t.Fatalf("stand-in exists: %v", err)
		}
		info, err := os.Stat(filepath.Dir(name))
		if err != nil || info.Mode().Perm() != 0700 {
			t.Fatal(info, err)
		}
	}
}

package grade

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func TestPR8PhysicalSubject(t *testing.T) {
	// QS8: logical parent traversal must not select a decoy binary.
	for _, layout := range []string{"PathParent", "WorkingParent"} {
		t.Run(layout, func(t *testing.T) {
			parent := t.TempDir()
			physical := filepath.Join(parent, "physical")
			logical := filepath.Join(parent, "logical")
			for _, dir := range []string{physical, logical} {
				if e := os.Mkdir(dir, 0700); e != nil {
					t.Fatal(e)
				}
			}
			o, d, _ := fixtureAt(t, physical)
			_, _, _ = fixtureAt(t, logical)
			if e := os.Mkdir(filepath.Join(physical, "child"), 0700); e != nil {
				t.Fatal(e)
			}
			link := filepath.Join(logical, "link")
			if e := os.Symlink(filepath.Join(physical, "child"), link); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(filepath.Join(logical, "bin", "subject"), []byte("decoy"), 0600); e != nil {
				t.Fatal(e)
			}
			var out bytes.Buffer
			if code, e := Run(o, d, &out); code != 4 || e != nil {
				t.Fatalf("control %d %v", code, e)
			}
			if layout == "PathParent" {
				o.SubjectFile = link + "/../subject.json"
			} else {
				t.Chdir(link)
				t.Setenv("PWD", link)
				o.SubjectFile = "../subject.json"
			}
			out.Reset()
			code, e := Run(o, d, &out)
			var refusal *Error
			if code != 2 || !errors.As(e, &refusal) || refusal.Rule != "QS8" || out.Len() != 0 {
				t.Fatalf("%d %v %q", code, e, out.String())
			}
		})
	}
}

func TestPR8AbsentHome(t *testing.T) {
	// QS8, QE16: compare existing ancestors of absent homes; other stat failures are faults.
	for _, row := range []string{"Related", "Unrelated", "StatFault"} {
		t.Run(row, func(t *testing.T) {
			o, d, _ := fixture(t)
			home := filepath.Join(t.TempDir(), "missing", "home")
			d.AccountHome = func() (string, error) { return home, nil }
			var out bytes.Buffer
			if code, e := Run(o, d, &out); code != 4 || e != nil {
				t.Fatalf("control %d %v", code, e)
			}
			if row == "Related" {
				home = filepath.Join(filepath.Dir(o.SubjectFile), "missing", "home")
			}
			if row == "StatFault" {
				d.StatInput = func(name string) (os.FileInfo, error) {
					if name == filepath.Dir(filepath.Dir(home)) {
						return nil, syscall.EACCES
					}
					return os.Stat(name)
				}
			}
			out.Reset()
			code, e := Run(o, d, &out)
			want := 4
			rule := ""
			if row == "Related" {
				want = 2
				rule = "QS8"
			}
			if row == "StatFault" {
				want = 70
				rule = "QE16"
			}
			if code != want {
				t.Fatalf("%d %v", code, e)
			}
			if rule != "" {
				var refusal *Error
				if !errors.As(e, &refusal) || refusal.Rule != rule || out.Len() != 0 {
					t.Fatalf("%v %q", e, out.String())
				}
			} else if e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestPR8AbsoluteKinds(t *testing.T) {
	// QS12, QA6: existing special executables refuse; missing ones are deferred.
	for _, kind := range []string{"Regular", "Directory", "FIFO", "Missing", "Descriptor"} {
		t.Run(kind, func(t *testing.T) {
			o, d, _ := fixture(t)
			dir := t.TempDir()
			name := filepath.Join(dir, "tool")
			if kind == "Directory" {
				if e := os.Mkdir(name, 0700); e != nil {
					t.Fatal(e)
				}
			} else if kind == "FIFO" {
				if e := syscall.Mkfifo(name, 0600); e != nil {
					t.Fatal(e)
				}
			} else if kind != "Missing" {
				if e := os.WriteFile(name, []byte("different"), 0600); e != nil {
					t.Fatal(e)
				}
			}
			direct := strictjson.Object{"binary": "bin/subject", "runbooks": strictjson.Array{strictjson.Object{"steps": strictjson.Array{strictjson.Object{"checks": strictjson.Array{strictjson.Object{"expected": strictjson.Object{"computation": strictjson.Object{"argv": strictjson.Array{name}}}}}}}}}}
			_, directErr := planningInputWith(o, direct, inputAccess{systemPath: func(string) bool { return true }})
			if kind == "Directory" || kind == "FIFO" {
				var refusal *Error
				if !errors.As(directErr, &refusal) || refusal.Rule != "QS12" {
					t.Fatalf("path kind %v", directErr)
				}
			} else if directErr != nil {
				t.Fatal(directErr)
			}

			input := strings.Replace(validSubject, `"runbooks":[]`, `"runbooks":[{"name":"book","steps":[{"name":"step","words":[],"budget_seconds":1,"checks":[{"dimension":"response-correctness","predicate":"stdout_equals","path":null,"expected":{"computation":{"argv":["/bin/echo"],"budget_seconds":1,"declared_files":[],"captures":[]}},"precondition":null,"exit_codes":null}]}]}]`, 1)
			if e := os.WriteFile(o.SubjectFile, []byte(input), 0600); e != nil {
				t.Fatal(e)
			}
			var controlOut bytes.Buffer
			if code, e := Run(o, d, &controlOut); code != 4 || e != nil {
				t.Fatalf("control %d %v", code, e)
			}
			opens := 0
			d.SystemPath = func(string) bool { return true }
			d.OpenSystemFile = func(string) (*os.File, error) {
				opens++
				if kind == "Descriptor" {
					return os.Open("/dev/null")
				}
				return os.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
			}
			// Resolve an existing system path first, then substitute the injected descriptor.
			var out bytes.Buffer
			code, e := Run(o, d, &out)
			want := 4
			if kind == "Descriptor" || kind == "Directory" || kind == "FIFO" {
				want = 2
			}
			if code != want {
				t.Fatalf("%d %v", code, e)
			}
			if want == 2 {
				var refusal *Error
				if !errors.As(e, &refusal) || refusal.Rule != "QS12" || out.Len() != 0 {
					t.Fatalf("%v %q", e, out.String())
				}
			}
			if opens != 1 {
				t.Fatalf("opens %d", opens)
			}
		})
	}
}

func TestPR8SubjectDevice(t *testing.T) {
	// QS2: the pre-open type check refuses devices; the regular control returns bytes.
	TestPR8SubjectRegular(t)
	opened := false
	_, e := openInputWith("/dev/null", func(name string) (*os.File, error) { opened = true; return os.Open(name) })
	if !errors.Is(e, errSpecial) || opened {
		t.Fatalf("opened=%v error=%v", opened, e)
	}
}

func TestPR8AbsoluteMemo(t *testing.T) {
	// QS12: repeated absolute executable names and a symlink are read once.
	o, _, _ := fixture(t)
	dir := t.TempDir()
	name := filepath.Join(dir, "tool")
	alias := filepath.Join(dir, "alias")
	if e := os.WriteFile(name, []byte("different"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(name, alias); e != nil {
		t.Fatal(e)
	}
	checks := strictjson.Array{}
	for _, n := range []string{name, name, alias, alias} {
		checks = append(checks, strictjson.Object{"expected": strictjson.Object{"computation": strictjson.Object{"argv": strictjson.Array{n}}}})
	}
	s := strictjson.Object{"binary": "bin/subject", "runbooks": strictjson.Array{strictjson.Object{"steps": strictjson.Array{strictjson.Object{"checks": checks}}}}}
	opens := 0
	_, e := planningInputWith(o, s, inputAccess{systemPath: func(string) bool { return true }, openAbsolute: func(n string) (*os.File, error) { opens++; return os.Open(n) }})
	if e != nil || opens != 1 {
		t.Fatalf("opens=%d %v", opens, e)
	}
}

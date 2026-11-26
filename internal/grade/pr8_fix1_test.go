package grade

import (
	"bytes"
	"errors"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"github.com/vinnyp/press-quality-lab/internal/subject"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"testing/iotest"
)

func TestPR8UnreadableCategory(t *testing.T) {
	// QS8, E-18: permission denial has a distinct cause.
	e := safeReadFailure("QS8", "named file", syscall.EACCES)
	if e.Cause != "named file cannot be safely read: unreadable" {
		t.Fatal(e)
	}
}
func TestPR8SubjectFIFO(t *testing.T) {
	// QS2: a subject file is a regular file; a FIFO is refused before open.
	path := filepath.Join(t.TempDir(), "pipe")
	if e := syscall.Mkfifo(path, 0600); e != nil {
		t.Fatal(e)
	}
	f, e := openInput(path)
	if e == nil {
		if e := f.Close(); e != nil {
			t.Fatal(e)
		}
		t.Fatal("FIFO accepted")
	}
	if !errors.Is(e, errSpecial) {
		t.Fatal(e)
	}
}
func TestPR8TokenReaders(t *testing.T) {
	// QP10, E-20: every token start counts, across read boundaries and final errors.
	for _, row := range []struct {
		name, data, token string
		want              int
	}{
		{"Boundary", strings.Repeat("x", (32<<10)-3) + "abcdefgh", "abcdefgh", 1},
		{"Overlap", strings.Repeat("x", (32<<10)-4) + "aaaaaaaaa", "aaaaaaaa", 2},
	} {
		t.Run(row.name, func(t *testing.T) {
			for _, reader := range []io.Reader{strings.NewReader(row.data), iotest.OneByteReader(strings.NewReader(row.data)), iotest.DataErrReader(strings.NewReader(row.data))} {
				n, e := tokenOccurrences(reader, []byte(row.token))
				if e != nil || n != row.want {
					t.Fatalf("%d %v", n, e)
				}
			}
		})
	}
}
func TestPR8HomeIdentity(t *testing.T) {
	// QS8, E-8: home and its ancestors are refused by identity, with a valid binary.
	parent := t.TempDir()
	home := filepath.Join(parent, "AccountHome")
	sibling := filepath.Join(parent, "sibling")
	for _, dir := range []string{parent, home, sibling} {
		if e := os.MkdirAll(filepath.Join(dir, "bin"), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(dir, "bin", "subject"), []byte("synthetic"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	if runtime.GOOS == "darwin" {
		variant := filepath.Join(parent, "accounthome")
		if _, e := os.Stat(variant); e == nil {
			_, e := planningInputWith(Options{SubjectFile: filepath.Join(variant, "subject.json")}, subject.File{"binary": "bin/subject"}, inputAccess{home: func() (string, error) { return home, nil }})
			var refusal *Error
			if !errors.As(e, &refusal) || refusal.Cause != "subject directory cannot be safely read: home" {
				t.Fatal(e)
			}
		} else {
			t.Log("case-sensitive volume: variant does not name the home")
		}
	}
	s, e := subject.Parse([]byte(validSubject))
	if e != nil {
		t.Fatal(e)
	}
	for _, row := range []struct {
		name, dir string
		refuse    bool
	}{{"Home", home, true}, {"Ancestor", parent, true}, {"Sibling", sibling, false}} {
		t.Run(row.name, func(t *testing.T) {
			_, e := planningInputWith(Options{SubjectFile: filepath.Join(row.dir, "subject.json")}, s, inputAccess{home: func() (string, error) { return home, nil }})
			if row.refuse {
				var refusal *Error
				if !errors.As(e, &refusal) || refusal.Cause != "subject directory cannot be safely read: home" {
					t.Fatal(e)
				}
			} else if e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestPR8DigestMemo(t *testing.T) {
	// QS8, QS11: repeated mentions of the same resolved file open it once.
	o, _, _ := fixture(t)
	dir := filepath.Dir(o.SubjectFile)
	s, e := subject.Parse([]byte(validSubject))
	if e != nil {
		t.Fatal(e)
	}
	// A second name resolves to the binary; repeated computation declarations must reuse its digest.
	if e := os.Symlink("bin/subject", filepath.Join(dir, "alias")); e != nil {
		t.Fatal(e)
	}
	s["runbooks"] = strictjson.Array{strictjson.Object{"steps": strictjson.Array{strictjson.Object{"checks": strictjson.Array{strictjson.Object{"expected": strictjson.Object{"computation": strictjson.Object{"argv": strictjson.Array{"alias"}, "declared_files": strictjson.Array{"alias", "bin/subject", "alias"}}}}}}}}}
	opens := 0
	_, err := planningInputWith(o, s, inputAccess{open: func(root *os.Root, name string) (*os.File, error) {
		opens++
		return root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	}})
	var refusal *Error
	if !errors.As(err, &refusal) || refusal.Rule != "QS12" || opens != 1 {
		t.Fatalf("opens=%d error=%v", opens, err)
	}
}

func TestPR8SubjectRegular(t *testing.T) {
	// QS2: the regular-file control returns its bytes.
	path := filepath.Join(t.TempDir(), "subject.json")
	if e := os.WriteFile(path, []byte("{}"), 0600); e != nil {
		t.Fatal(e)
	}
	f, e := openInput(path)
	if e != nil {
		t.Fatal(e)
	}
	b, e := io.ReadAll(f)
	e = errors.Join(e, f.Close())
	if e != nil || !bytes.Equal(b, []byte("{}")) {
		t.Fatal(string(b), e)
	}
}

func TestPR8AbsoluteExecutables(t *testing.T) {
	// QS12, QA4, QA6: content equality refuses; missing executes later; other read faults exit 70.
	o, _, _ := fixture(t)
	s, e := subject.Parse([]byte(validSubject))
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	executable := filepath.Join(dir, "tool")
	s["runbooks"] = strictjson.Array{strictjson.Object{"steps": strictjson.Array{strictjson.Object{"checks": strictjson.Array{strictjson.Object{"expected": strictjson.Object{"computation": strictjson.Object{"argv": strictjson.Array{executable}}}}}}}}}
	for _, row := range []struct {
		name, data string
		openError  error
		rule       string
		fault      bool
	}{
		{"Different", "different", nil, "", false},
		{"Equal", "synthetic", nil, "QS12", false},
		{"Missing", "different", nil, "", false},
		{"Unreadable", "different", syscall.EACCES, "", true},
	} {
		t.Run(row.name, func(t *testing.T) {
			comp := inputObject(inputObject(inputArray(inputObject(inputArray(inputObject(inputArray(s["runbooks"])[0])["steps"])[0])["checks"])[0])["expected"])
			name := executable
			if row.name == "Missing" {
				name += "missing"
			}
			inputObject(comp["computation"])["argv"] = strictjson.Array{name}
			if e := os.WriteFile(executable, []byte(row.data), 0600); e != nil {
				t.Fatal(e)
			}
			_, err := planningInputWith(o, s, inputAccess{systemPath: func(name string) bool { return strings.HasPrefix(name, dir+string(os.PathSeparator)) }, openAbsolute: func(name string) (*os.File, error) {
				if row.openError != nil {
					return nil, row.openError
				}
				return os.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
			}})
			if row.fault {
				if !errors.Is(err, row.openError) {
					t.Fatal(err)
				}
				var refusal *Error
				if errors.As(err, &refusal) && refusal.Code == 2 {
					t.Fatal(err)
				}
				return
			}
			if row.rule == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				var refusal *Error
				if !errors.As(err, &refusal) || refusal.Rule != row.rule {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestPR8E9Order(t *testing.T) {
	// QS1, QS3, QS4, QS5, QS6, QS7, QS8, QS9, QS10, QS11, QS12, QP7, QP10, QA4, QA9, E-9:
	// Adjacent pairs suffice to pin the entire deciding order, including file/declaration boundaries.
	check := func(s subject.File) strictjson.Object {
		return inputObject(inputArray(inputObject(inputArray(inputObject(inputArray(s["runbooks"])[0])["steps"])[0])["checks"])[0])
	}
	computation := func(s subject.File) strictjson.Object {
		c := strictjson.Object{"argv": strictjson.Array{"/bin/echo"}, "budget_seconds": strictjson.Number("1"), "declared_files": strictjson.Array{}, "captures": strictjson.Array{}}
		check(s)["expected"] = strictjson.Object{"computation": c}
		return c
	}
	rows := []struct {
		rule   string
		change func(subject.File)
	}{
		{"QS1", func(s subject.File) { s["subject_file_version"] = "other" }},
		{"QS3", func(s subject.File) { delete(s, "prompts") }},
		{"QS4", func(s subject.File) { inputObject(s["facts"])["has_cli"] = "yes" }},
		{"QS5", func(s subject.File) { check(s)["expected"] = strictjson.Object{"placeholder": "Supply expected."} }},
		{"QS6", func(s subject.File) { s["commands"] = append(inputArray(s["commands"]), inputArray(s["commands"])[0]) }},
		{"QS7", func(s subject.File) {
			s["offline"] = strictjson.Array{strictjson.Object{"applies_to": strictjson.Array{strictjson.Array{}}, "args": strictjson.Array{}, "env": strictjson.Object{"HOME": "x"}}}
		}},
		{"QS8", func(s subject.File) { s["binary"] = "missing" }},
		{"QS9", func(s subject.File) { s["store"] = "missing" }},
		{"QS10", func(s subject.File) { s["runbooks"] = append(inputArray(s["runbooks"]), inputArray(s["runbooks"])[0]) }},
		{"QS11", func(s subject.File) {
			a := strictjson.Array{}
			for i := 0; i < 1025; i++ {
				a = append(a, strictjson.Object{"record": "book", "dimension": "response-correctness"})
			}
			s["markers"] = a
		}},
		{"QS12", func(s subject.File) { computation(s)["declared_files"] = strictjson.Array{"bin/subject"} }},
		{"QP7", func(s subject.File) { s["bookkeeping"] = strictjson.Array{"data"} }},
		{"QP10", func(s subject.File) { inputObject(s["secret"])["path"] = "data" }},
		{"QA4", func(s subject.File) { computation(s)["argv"] = strictjson.Array{"/opt/synthetic"} }},
		{"QA9", func(s subject.File) { check(s)["predicate"] = "stdout_empty" }},
	}
	for i := 0; i < len(rows)-1; i++ {
		first, second := rows[i], rows[i+1]
		t.Run(first.rule+"_"+second.rule, func(t *testing.T) {
			o, d, _ := fixture(t)
			dir := filepath.Dir(o.SubjectFile)
			if e := os.Mkdir(filepath.Join(dir, "seed"), 0700); e != nil {
				t.Fatal(e)
			}
			for name, data := range map[string]string{"data": "data", "token": "abcdefgh"} {
				if e := os.WriteFile(filepath.Join(dir, "seed", name), []byte(data), 0600); e != nil {
					t.Fatal(e)
				}
			}
			s, e := subject.Parse([]byte(validSubject))
			if e != nil {
				t.Fatal(e)
			}
			s["seed"] = "seed"
			s["store"] = "data"
			s["secret"] = strictjson.Object{"path": "token", "kind": "credential", "reason": nil, "token": "abcdefgh", "commands": strictjson.Array{strictjson.Array{}}}
			s["runbooks"] = strictjson.Array{strictjson.Object{"name": "book", "steps": strictjson.Array{strictjson.Object{"name": "step", "words": strictjson.Array{}, "budget_seconds": strictjson.Number("1"), "checks": strictjson.Array{strictjson.Object{"dimension": "response-correctness", "predicate": "stdout_equals", "path": nil, "expected": strictjson.Object{"literal": "x"}, "precondition": nil, "exit_codes": nil}}}}}}
			write := func() {
				t.Helper()
				if e := os.WriteFile(o.SubjectFile, strictjson.Write(s), 0600); e != nil {
					t.Fatal(e)
				}
			}
			write()
			var out bytes.Buffer
			code, err := Run(o, d, &out)
			if code != 4 || err != nil {
				t.Fatalf("control %d %v", code, err)
			}
			first.change(s)
			second.change(s)
			write()
			out.Reset()
			code, err = Run(o, d, &out)
			var refusal *Error
			if code != 2 || !errors.As(err, &refusal) || refusal.Rule != first.rule {
				t.Fatalf("%d %v want %s", code, err, first.rule)
			}
		})
	}
}

func TestPR8RelativeLinkedCWD(t *testing.T) {
	// QS8: make the path absolute before resolving links, including links in the cwd.
	parent := t.TempDir()
	real := filepath.Join(parent, "real")
	link := filepath.Join(parent, "linked")
	if e := os.MkdirAll(filepath.Join(real, "subject"), 0700); e != nil {
		t.Fatal(e)
	}
	o, _, _ := fixtureAt(t, filepath.Join(real, "subject"))
	if e := os.Symlink(real, link); e != nil {
		t.Fatal(e)
	}
	t.Chdir(link)
	t.Setenv("PWD", link)
	o.SubjectFile = filepath.Join("subject", filepath.Base(o.SubjectFile))
	s, e := subject.Parse([]byte(validSubject))
	if e != nil {
		t.Fatal(e)
	}
	if _, e := planningInput(o, s); e != nil {
		t.Fatal(e)
	}
}

func TestPR8HomeLinkAncestor(t *testing.T) {
	// QS8, E-8: resolve the account home's links before walking its ancestors.
	o, _, _ := fixture(t)
	parent := filepath.Dir(o.SubjectFile)
	home := filepath.Join(parent, "home")
	if e := os.Mkdir(home, 0700); e != nil {
		t.Fatal(e)
	}
	link := filepath.Join(t.TempDir(), "home")
	if e := os.Symlink(home, link); e != nil {
		t.Fatal(e)
	}
	_, err := planningInputWith(o, subject.File{"binary": "bin/subject"}, inputAccess{home: func() (string, error) { return link, nil }})
	var refusal *Error
	if !errors.As(err, &refusal) || refusal.Cause != "subject directory cannot be safely read: home" {
		t.Fatal(err)
	}
}

func TestPR8AbsoluteDescriptor(t *testing.T) {
	// QS12, QS8: the descriptor, not only the pathname, must be regular.
	o, _, _ := fixture(t)
	dir := t.TempDir()
	name := filepath.Join(dir, "executable")
	pipe := filepath.Join(dir, "pipe")
	if e := os.WriteFile(name, []byte("different"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := syscall.Mkfifo(pipe, 0600); e != nil {
		t.Fatal(e)
	}
	s := subject.File{"binary": "bin/subject", "runbooks": strictjson.Array{strictjson.Object{"steps": strictjson.Array{strictjson.Object{"checks": strictjson.Array{strictjson.Object{"expected": strictjson.Object{"computation": strictjson.Object{"argv": strictjson.Array{name}}}}}}}}}}
	access := inputAccess{systemPath: func(s string) bool { return strings.HasPrefix(s, dir+string(os.PathSeparator)) }}
	if _, e := planningInputWith(o, s, access); e != nil {
		t.Fatal(e)
	}
	access.openAbsolute = func(string) (*os.File, error) { return os.OpenFile(pipe, os.O_RDONLY|syscall.O_NONBLOCK, 0) }
	if _, e := planningInputWith(o, s, access); !errors.Is(e, errSpecial) {
		t.Fatal(e)
	}
}

func TestPR8HomeLookupFault(t *testing.T) {
	// QS8, E-8, QE16: a failed account lookup never falls back to HOME and exits 70.
	o, d, _ := fixture(t)
	home := t.TempDir()
	d.AccountHome = func() (string, error) { return home, nil }
	var out bytes.Buffer
	if code, e := Run(o, d, &out); code != 4 || e != nil {
		t.Fatalf("control %d %v", code, e)
	}
	problem := errors.New("account record unavailable")
	d.AccountHome = func() (string, error) { return "", problem }
	t.Setenv("HOME", filepath.Dir(o.SubjectFile))
	out.Reset()
	code, e := Run(o, d, &out)
	var failure *Error
	if code != 70 || !errors.As(e, &failure) || failure.Cause != problem.Error() || out.Len() != 0 {
		t.Fatalf("%d %v %q", code, e, out.String())
	}
}

func TestPR8AbsoluteReadFaultRun(t *testing.T) {
	// QS12, QA6, QE16: unreadable system executable is an engine fault, not a missing computation.
	o, d, _ := fixture(t)
	input := strings.Replace(validSubject, `"runbooks":[]`, `"runbooks":[{"name":"book","steps":[{"name":"step","words":[],"budget_seconds":1,"checks":[{"dimension":"response-correctness","predicate":"stdout_equals","path":null,"expected":{"computation":{"argv":["/bin/echo"],"budget_seconds":1,"declared_files":[],"captures":[]}},"precondition":null,"exit_codes":null}]}]}]`, 1)
	if e := os.WriteFile(o.SubjectFile, []byte(input), 0600); e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if code, e := Run(o, d, &out); code != 4 || e != nil {
		t.Fatalf("control %d %v", code, e)
	}
	d.OpenSystemFile = func(string) (*os.File, error) { return nil, syscall.EACCES }
	out.Reset()
	code, e := Run(o, d, &out)
	var failure *Error
	if code != 70 || !errors.As(e, &failure) || failure.Rule != "QE16" || failure.Cause != "permission denied" || out.Len() != 0 {
		t.Fatalf("%d %v %q", code, e, out.String())
	}
}

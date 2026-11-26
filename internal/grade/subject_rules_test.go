package grade

import (
	"bytes"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/jcs"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/rulebook"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"github.com/vinnyp/press-quality-lab/internal/subject"
)

func inputSubject(t *testing.T) subject.File {
	t.Helper()
	s, e := subject.Parse([]byte(validSubject))
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func expectInputRule(t *testing.T, o Options, s subject.File, want string) {
	t.Helper()
	if _, err := subject.Parse(strictjson.Write(s)); err != nil {
		t.Fatalf("fixture breaks declaration guard: %v", err)
	}
	_, err := planningInput(o, s)
	if want == "" {
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	var e *Error
	if !errors.As(err, &e) || e.Code != 2 || e.Rule != want {
		t.Fatalf("got %v want exit 2 %s", err, want)
	}
}
func writeInput(t *testing.T, name, contents string) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(name), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(name, []byte(contents), 0600); e != nil {
		t.Fatal(e)
	}
}
func TestNamedFileForms(t *testing.T) {
	// QS8: each of the six refusal forms for each of the three named-file kinds.
	for _, kind := range []string{"Binary", "Executable", "Declared"} {
		for _, form := range []string{"Escape", "Directory", "UnresolvedLink", "Missing", "Special", "Unreadable"} {
			t.Run(kind+"_"+form, func(t *testing.T) {
				o, _, _ := fixture(t)
				dir := filepath.Dir(o.SubjectFile)
				s := inputSubject(t)
				name := "named"
				writeInput(t, filepath.Join(dir, name), "computation")
				assign := func(name string) {
					if kind == "Binary" {
						s["binary"] = name
						return
					}
					argv := strictjson.Array{"/bin/echo"}
					files := strictjson.Array{}
					if kind == "Executable" {
						argv = strictjson.Array{name}
					} else {
						files = strictjson.Array{name}
					}
					s["runbooks"] = strictjson.Array{strictjson.Object{"name": "book", "steps": strictjson.Array{strictjson.Object{"name": "step", "words": strictjson.Array{}, "budget_seconds": strictjson.Number("1"), "checks": strictjson.Array{strictjson.Object{"dimension": "response-correctness", "predicate": "stdout_equals", "path": nil, "expected": strictjson.Object{"computation": strictjson.Object{"argv": argv, "declared_files": files, "budget_seconds": strictjson.Number("1"), "captures": strictjson.Array{}}}, "precondition": nil, "exit_codes": nil}}}}}}
				}
				assign(name)
				expectInputRule(t, o, s, "")
				switch form {
				case "Escape":
					outside := filepath.Join(t.TempDir(), "outside")
					writeInput(t, outside, "outside")
					name = "escape"
					if e := os.Symlink(outside, filepath.Join(dir, name)); e != nil {
						t.Fatal(e)
					}
				case "Directory":
					name = "directory"
					if e := os.Mkdir(filepath.Join(dir, name), 0700); e != nil {
						t.Fatal(e)
					}
				case "UnresolvedLink":
					name = "dangling"
					if e := os.Symlink("missing", filepath.Join(dir, name)); e != nil {
						t.Fatal(e)
					}
				case "Missing":
					name = "missing"
				case "Special":
					name = "pipe"
					if e := syscall.Mkfifo(filepath.Join(dir, name), 0600); e != nil {
						t.Fatal(e)
					}
				case "Unreadable":
					if os.Geteuid() == 0 {
						t.Skip("root bypasses permission bits")
					}
					if e := os.Chmod(filepath.Join(dir, name), 0000); e != nil {
						t.Fatal(e)
					}
				}
				assign(name)
				expectInputRule(t, o, s, "QS8")
			})
		}
	}
}
func TestAccountHome(t *testing.T) {
	// QS8, E-8: the account record, not HOME, defines the protected directory.
	o, _, _ := fixture(t)
	t.Setenv("HOME", filepath.Dir(o.SubjectFile))
	s := inputSubject(t)
	expectInputRule(t, o, s, "")
	account, e := user.Current()
	if e != nil {
		t.Fatal(e)
	}
	for _, dir := range []string{account.HomeDir, filepath.Dir(account.HomeDir)} {
		expectInputRule(t, Options{SubjectFile: filepath.Join(dir, "subject.json")}, s, "QS8")
	}
}
func TestSeedResolvedPaths(t *testing.T) {
	// QS9, E-17, E-18: one fixture per row, always accepted before its mutation.
	for _, row := range []string{"NameLink", "ParentLink", "SubjectDirectoryLink", "ContainedLink", "EscapingNameLink", "FileNameLink", "DanglingNameLink", "Special", "Reserved", "UnreadableFile", "UnreadableDirectory"} {
		t.Run(row, func(t *testing.T) {
			o, _, _ := fixture(t)
			dir := filepath.Dir(o.SubjectFile)
			s := inputSubject(t)
			s["seed"] = "real/seed"
			writeInput(t, filepath.Join(dir, "real", "seed", "data"), "seed")
			expectInputRule(t, o, s, "")
			want := "QS9"
			switch row {
			case "NameLink":
				if e := os.Symlink("real/seed", filepath.Join(dir, "linked")); e != nil {
					t.Fatal(e)
				}
				s["seed"] = "linked"
				want = ""
			case "ParentLink":
				if e := os.Symlink("real", filepath.Join(dir, "linked")); e != nil {
					t.Fatal(e)
				}
				s["seed"] = "linked/seed"
				want = ""
			case "SubjectDirectoryLink":
				parent := t.TempDir()
				link := filepath.Join(parent, "linked")
				if e := os.Symlink(dir, link); e != nil {
					t.Fatal(e)
				}
				o.SubjectFile = filepath.Join(link, "subject.json")
				want = ""
			case "ContainedLink":
				if e := os.Symlink("data", filepath.Join(dir, "real", "seed", "linked")); e != nil {
					t.Fatal(e)
				}
			case "EscapingNameLink":
				outside := t.TempDir()
				if e := os.Symlink(outside, filepath.Join(dir, "linked")); e != nil {
					t.Fatal(e)
				}
				s["seed"] = "linked"
			case "FileNameLink":
				if e := os.Symlink("real/seed/data", filepath.Join(dir, "linked")); e != nil {
					t.Fatal(e)
				}
				s["seed"] = "linked"
			case "DanglingNameLink":
				if e := os.Symlink("absent", filepath.Join(dir, "linked")); e != nil {
					t.Fatal(e)
				}
				s["seed"] = "linked"
			case "Special":
				if e := syscall.Mkfifo(filepath.Join(dir, "real", "seed", "pipe"), 0600); e != nil {
					t.Fatal(e)
				}
			case "Reserved":
				writeInput(t, filepath.Join(dir, "real", "seed", ".pql-tmp", "data"), "data")
			case "UnreadableFile":
				if os.Geteuid() == 0 {
					t.Skip("root bypasses permission bits")
				}
				if e := os.Chmod(filepath.Join(dir, "real", "seed", "data"), 0000); e != nil {
					t.Fatal(e)
				}
			case "UnreadableDirectory":
				if os.Geteuid() == 0 {
					t.Skip("root bypasses permission bits")
				}
				if e := os.Mkdir(filepath.Join(dir, "real", "seed", "locked"), 0000); e != nil {
					t.Fatal(e)
				}
			}
			if row == "EscapingNameLink" {
				// Assert the resolution guard itself as well as the confined reader;
				// os.Root independently refuses escapes and must not mask a missing guard.
				canonical, err := filepath.EvalSymlinks(dir)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := resolveInput(canonical, "real/seed"); err != nil {
					t.Fatal(err)
				}
				if _, err := resolveInput(canonical, "linked"); !errors.Is(err, errEscape) {
					t.Fatalf("resolved containment: %v", err)
				}
			}
			expectInputRule(t, o, s, want)
			if row == "ContainedLink" || row == "Special" {
				_, err := planningInput(o, s)
				var refusal *Error
				cause := "seed cannot be safely read: special"
				if row == "ContainedLink" {
					cause = "seed cannot be safely read: link"
				}
				if !errors.As(err, &refusal) || refusal.Cause != cause {
					t.Fatal(err)
				}
			}
		})
	}
}
func TestFileContentRules(t *testing.T) {
	// QS9, QS12, QP7, QP10: file contents and exclusions, including E-20 overlaps.
	for _, row := range []struct{ name, rule string }{{"StoreAbsent", "QS9"}, {"SeedEqualsBinary", "QS12"}, {"DeclaredEqualsBinary", "QS12"}, {"ExecutableEqualsBinary", "QS12"}, {"BookkeepingStore", "QP7"}, {"BookkeepingSeed", "QP7"}, {"SecretAbsent", "QP10"}, {"SecretEqualsStore", "QP10"}, {"SecretMissingToken", "QP10"}, {"SecretRepeatedToken", "QP10"}, {"SecretOverlappingToken", "QP10"}} {
		t.Run(row.name, func(t *testing.T) {
			o, _, _ := fixture(t)
			dir := filepath.Dir(o.SubjectFile)
			s := inputSubject(t)
			s["seed"] = "seed"
			s["store"] = "data"
			writeInput(t, filepath.Join(dir, "seed", "data"), "aaaaaaaa")
			writeInput(t, filepath.Join(dir, "seed", "token"), "aaaaaaaa")
			s["secret"] = strictjson.Object{"path": "token", "kind": "credential", "reason": nil, "token": "aaaaaaaa", "commands": strictjson.Array{strictjson.Array{}}}
			writeInput(t, filepath.Join(dir, "seed", "copy"), "seed")
			if row.name == "BookkeepingStore" {
				s["seed"] = strictjson.Object{"placeholder": "Supply the pristine seed."}
			}
			if row.name == "DeclaredEqualsBinary" || row.name == "ExecutableEqualsBinary" {
				writeInput(t, filepath.Join(dir, "copy"), "computation")
				argv := strictjson.Array{"/bin/echo"}
				files := strictjson.Array{"copy"}
				if row.name == "ExecutableEqualsBinary" {
					argv = strictjson.Array{"copy"}
					files = strictjson.Array{}
				}
				s["runbooks"] = computationBook(argv, files)
			}
			expectInputRule(t, o, s, "")
			secret := fixtureObject(t, s["secret"])
			switch row.name {
			case "StoreAbsent":
				s["store"] = "missing"
			case "SeedEqualsBinary":
				writeInput(t, filepath.Join(dir, "seed", "copy"), "synthetic")
			case "DeclaredEqualsBinary", "ExecutableEqualsBinary":
				writeInput(t, filepath.Join(dir, "copy"), "synthetic")
			case "BookkeepingStore":
				s["bookkeeping"] = strictjson.Array{"data"}
			case "BookkeepingSeed":
				s["bookkeeping"] = strictjson.Array{"token"}
			case "SecretAbsent":
				secret["path"] = "missing"
			case "SecretEqualsStore":
				secret["path"] = "data"
			case "SecretMissingToken":
				writeInput(t, filepath.Join(dir, "seed", "token"), "absent")
			case "SecretRepeatedToken":
				writeInput(t, filepath.Join(dir, "seed", "token"), "aaaaaaaa aaaaaaaa")
			case "SecretOverlappingToken":
				writeInput(t, filepath.Join(dir, "seed", "token"), "aaaaaaaaa")
			}
			expectInputRule(t, o, s, row.rule)
		})
	}
}
func TestSubjectRefusalFunnel(t *testing.T) {
	// QS8, QS10, GKD8: file failures compete with semantic failures in E-9 order,
	// return exit 2, and never reveal a subject-file value in their cause.
	o, d, _ := fixture(t)
	s := inputSubject(t)
	var out bytes.Buffer
	code, e := Run(o, d, &out)
	if code != 4 || e != nil {
		t.Fatalf("control %d %v", code, e)
	}
	s["binary"] = "PRIVATE_SUBJECT_VALUE"
	s["runbooks"] = strictjson.Array{strictjson.Object{"name": "same", "steps": strictjson.Array{}}, strictjson.Object{"name": "same", "steps": strictjson.Array{}}}
	writeInput(t, o.SubjectFile, string(strictjson.Write(s)))
	out.Reset()
	code, e = Run(o, d, &out)
	if code != 2 || e == nil || e.Error() != "rejected: QS8: invalid subject file" || strings.Contains(e.Error(), "PRIVATE_SUBJECT_VALUE") {
		t.Fatalf("%d %v", code, e)
	}
}

func TestLiteralComputationLookalike(t *testing.T) {
	// QS8 and QA4 apply to declarations, not arbitrary JSON literal keys.
	o, _, _ := fixture(t)
	s := inputSubject(t)
	s["runbooks"] = strictjson.Array{strictjson.Object{"name": "book", "steps": strictjson.Array{strictjson.Object{"name": "step", "words": strictjson.Array{}, "budget_seconds": strictjson.Number("1"), "checks": strictjson.Array{strictjson.Object{"dimension": "response-correctness", "predicate": "stdout_json_equals", "path": nil, "expected": strictjson.Object{"literal": strictjson.Object{"argv": strictjson.Array{"missing"}, "declared_files": strictjson.Array{"missing"}, "budget_seconds": strictjson.Number("1"), "captures": strictjson.Array{}}}, "precondition": nil, "exit_codes": nil}}}}}}
	expectInputRule(t, o, s, "")
	// Moving precisely the same object into a computation declaration names files.
	books := fixtureArray(t, s["runbooks"])
	book := fixtureObject(t, books[0])
	steps := fixtureArray(t, book["steps"])
	step := fixtureObject(t, steps[0])
	checks := fixtureArray(t, step["checks"])
	check := fixtureObject(t, checks[0])
	expected := fixtureObject(t, check["expected"])
	check["expected"] = strictjson.Object{"computation": expected["literal"]}
	expectInputRule(t, o, s, "QS8")
}

func fixtureArray(t *testing.T, v any) strictjson.Array {
	t.Helper()
	a, ok := v.(strictjson.Array)
	if !ok {
		t.Fatal("fixture array missing")
	}
	return a
}
func fixtureObject(t *testing.T, v any) strictjson.Object {
	t.Helper()
	o, ok := v.(strictjson.Object)
	if !ok {
		t.Fatal("fixture object missing")
	}
	return o
}

func computationBook(argv, files strictjson.Array) strictjson.Array {
	return strictjson.Array{strictjson.Object{"name": "book", "steps": strictjson.Array{strictjson.Object{"name": "step", "words": strictjson.Array{}, "budget_seconds": strictjson.Number("1"), "checks": strictjson.Array{strictjson.Object{"dimension": "response-correctness", "predicate": "stdout_equals", "path": nil, "expected": strictjson.Object{"computation": strictjson.Object{"argv": argv, "declared_files": files, "budget_seconds": strictjson.Number("1"), "captures": strictjson.Array{}}}, "precondition": nil, "exit_codes": nil}}}}}}
}

func TestLiteralCheckIDs(t *testing.T) {
	// QS11, QE2: these exact canonical spellings enter the actual check preimage.
	ids := map[string]string{}
	for _, row := range []struct{ raw, want string }{{"0.1", "0.1"}, {"0.10", "0.1"}, {"-0", "0"}, {"0", "0"}, {"0.2", "0.2"}} {
		o, _, _ := fixture(t)
		s := inputSubject(t)
		s["runbooks"] = computationBook(strictjson.Array{"/bin/echo"}, strictjson.Array{})
		check := fixtureObject(t, fixtureArray(t, fixtureObject(t, fixtureArray(t, fixtureObject(t, fixtureArray(t, s["runbooks"])[0])["steps"])[0])["checks"])[0])
		check["predicate"] = "stdout_json_equals"
		check["expected"] = strictjson.Object{"literal": strictjson.Number(row.raw)}
		expectInputRule(t, o, s, "")
		in, e := planningInput(o, s)
		if e != nil {
			t.Fatal(e)
		}
		in.Scopes = []rulebook.Scope{rulebook.ConformanceScope}
		p, e := plan.Build(in.Input)
		if e != nil || len(p.Checks) != 7 {
			t.Fatalf("plan: %v checks=%d", e, len(p.Checks))
		}
		authored := p.Checks[6]
		b, e := jcs.Canonical(authored.Preimage["expected"])
		if e != nil || string(b) != `{"literal":`+row.want+`,"precondition":null}` {
			t.Fatalf("preimage=%s error=%v", b, e)
		}
		ids[row.raw] = authored.ID
	}
	if ids["0.1"] != ids["0.10"] || ids["-0"] != ids["0"] || ids["0.1"] == ids["0.2"] || ids["0.1"] == ids["0"] {
		t.Fatal("literal identity changed")
	}
}

package grade

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestPR8SubjectLayouts(t *testing.T) {
	// QS8, QE4: a file link selects its target directory; hard links retain their own physical parent.
	for _, layout := range []string{"Ordinary", "FileLink", "HardLink", "FileLinkHome"} {
		t.Run(layout, func(t *testing.T) {
			target := t.TempDir()
			o, d, saved := fixtureAt(t, target)
			canonical, err := filepath.EvalSymlinks(o.SubjectFile)
			if err != nil {
				t.Fatal(err)
			}
			if code, e := Run(o, d, io.Discard); code != 4 || e != nil {
				t.Fatalf("control %d %v", code, e)
			}
			if layout != "Ordinary" {
				other := t.TempDir()
				alias := filepath.Join(other, "subject.json")
				if layout == "FileLink" || layout == "FileLinkHome" {
					err = os.Symlink(o.SubjectFile, alias)
				} else {
					_, _, _ = fixtureAt(t, other)
					alias = filepath.Join(other, "hard.json")
					err = os.Link(o.SubjectFile, alias)
					if err != nil {
						t.Fatal(err)
					}
					canonical, err = filepath.EvalSymlinks(alias)
				}
				if err != nil {
					t.Fatal(err)
				}
				o.SubjectFile = alias
			}

			if layout == "FileLinkHome" {
				d.AccountHome = func() (string, error) { return target, nil }
				code, e := Run(o, d, io.Discard)
				var refusal *Error
				if code != 2 || !errors.As(e, &refusal) || refusal.Rule != "QS8" {
					t.Fatalf("%d %v", code, e)
				}
				return
			}
			code, e := Run(o, d, io.Discard)
			if code != 4 || e != nil || saved.SubjectFilePath != canonical || saved.BinaryDigest == nil || *saved.BinaryDigest != "b3cc0475bb78a5026098858e9889acf666d31062d513d303314eca31d36e72f2" {
				t.Fatalf("%d %v path=%s want=%s", code, e, saved.SubjectFilePath, canonical)
			}
		})
	}
}

func TestPR8HomeEdges(t *testing.T) {
	// QS8, QE16: a dangling home link protects its target ancestors; relative account homes are faults.
	for _, row := range []string{"DanglingLink", "Relative"} {
		t.Run(row, func(t *testing.T) {
			o, d, _ := fixture(t)
			home := t.TempDir()
			d.AccountHome = func() (string, error) { return home, nil }
			if code, e := Run(o, d, io.Discard); code != 4 || e != nil {
				t.Fatalf("control %d %v", code, e)
			}
			want, rule := 2, "QS8"
			if row == "DanglingLink" {
				link := filepath.Join(t.TempDir(), "home")
				if e := os.Symlink(filepath.Join(filepath.Dir(o.SubjectFile), "absent", "home"), link); e != nil {
					t.Fatal(e)
				}
				home = link
			} else {
				home = "relative/home"
				want = 70
				rule = "QE16"
			}
			var out bytes.Buffer
			code, e := Run(o, d, &out)
			var refusal *Error
			if code != want || !errors.As(e, &refusal) || refusal.Rule != rule || out.Len() != 0 {
				t.Fatalf("%d %v output length=%d", code, e, out.Len())
			}
		})
	}
}

func TestPR8UnstattableSubject(t *testing.T) {
	// QS2, QS8: a reader without descriptor identity must fail closed.
	o, d, _ := fixture(t)
	if code, e := Run(o, d, io.Discard); code != 4 || e != nil {
		t.Fatalf("control %d %v", code, e)
	}
	d.Open = func(string) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(validSubject)), nil }
	code, e := Run(o, d, io.Discard)
	var refusal *Error
	if code != 2 || !errors.As(e, &refusal) || refusal.Rule != "QS2" {
		t.Fatalf("%d %v", code, e)
	}
}

func TestPR8AbsoluteEdges(t *testing.T) {
	// QS12, QA6: only missing paths defer; failures at each reading stage refuse with exit 2.
	for _, row := range []string{"NotDirectory", "ResolvePermission", "ResolveLoop", "StatPermission", "MidCopy"} {
		t.Run(row, func(t *testing.T) {
			o, d, _ := fixture(t)
			input := strings.Replace(validSubject, `"runbooks":[]`, `"runbooks":[{"name":"book","steps":[{"name":"step","words":[],"budget_seconds":1,"checks":[{"dimension":"response-correctness","predicate":"stdout_equals","path":null,"expected":{"computation":{"argv":["/bin/echo"],"budget_seconds":1,"declared_files":[],"captures":[]}},"precondition":null,"exit_codes":null}]}]}]`, 1)
			if e := os.WriteFile(o.SubjectFile, []byte(input), 0600); e != nil {
				t.Fatal(e)
			}
			if code, e := Run(o, d, io.Discard); code != 4 || e != nil {
				t.Fatalf("control %d %v", code, e)
			}
			switch row {
			case "NotDirectory":
				name := filepath.Join(t.TempDir(), "file")
				if e := os.WriteFile(name, []byte("data"), 0600); e != nil {
					t.Fatal(e)
				}
				d.ResolveSystemFile = func(string) (string, error) { return filepath.EvalSymlinks(name + "/missing") }
			case "ResolvePermission":
				d.ResolveSystemFile = func(string) (string, error) { return "", syscall.EACCES }
			case "ResolveLoop":
				link := filepath.Join(t.TempDir(), "loop")
				if e := os.Symlink("loop", link); e != nil {
					t.Fatal(e)
				}
				d.ResolveSystemFile = func(string) (string, error) { return filepath.EvalSymlinks(link) }
			case "StatPermission":
				d.StatSystemFile = func(string) (os.FileInfo, error) { return nil, syscall.EACCES }
			case "MidCopy":
				d.CopySystemFile = func(w io.Writer, r io.Reader) (int64, error) { return io.Copy(w, &pr8FailReader{r: r}) }
			}
			code, e := Run(o, d, io.Discard)
			if row == "NotDirectory" {
				if code != 4 || e != nil {
					t.Fatalf("%d %v", code, e)
				}
				return
			}
			var refusal *Error
			if code != 2 || !errors.As(e, &refusal) || refusal.Rule != "QS12" {
				t.Fatalf("%d %v", code, e)
			}
		})
	}
}

type pr8FailReader struct {
	r    io.Reader
	read bool
}

func (r *pr8FailReader) Read(p []byte) (int, error) {
	if r.read {
		return 0, syscall.EIO
	}
	r.read = true
	if len(p) > 8 {
		p = p[:8]
	}
	return r.r.Read(p)
}

func TestPR8SubjectIdentity(t *testing.T) {
	// QS8: the directory handle must name the descriptor whose bytes were parsed.
	o, d, _ := fixture(t)
	if code, e := Run(o, d, io.Discard); code != 4 || e != nil {
		t.Fatalf("control %d %v", code, e)
	}
	other, _, _ := fixture(t)
	d.Open = func(string) (io.ReadCloser, error) { return openInput(other.SubjectFile) }
	code, e := Run(o, d, io.Discard)
	var refusal *Error
	if code != 2 || !errors.As(e, &refusal) || refusal.Rule != "QS8" {
		t.Fatalf("%d %v", code, e)
	}
}

func TestPR8SubjectLinkSwap(t *testing.T) {
	// QS8: a link inserted after resolution must not share the parsed file's identity.
	o, d, _ := fixture(t)
	if code, e := Run(o, d, io.Discard); code != 4 || e != nil {
		t.Fatalf("control %d %v", code, e)
	}
	d.Open = func(name string) (io.ReadCloser, error) {
		moved := name + ".moved"
		if e := os.Rename(name, moved); e != nil {
			return nil, e
		}
		if e := os.Symlink(filepath.Base(moved), name); e != nil {
			return nil, e
		}
		return openInput(name)
	}
	code, e := Run(o, d, io.Discard)
	var refusal *Error
	if code != 2 || !errors.As(e, &refusal) || refusal.Rule != "QS8" {
		t.Fatalf("%d %v", code, e)
	}
}

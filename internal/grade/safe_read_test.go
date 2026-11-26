package grade

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/vinnyp/press-quality-lab/internal/store"
)

func TestSafeReads(t *testing.T) {
	// QS8, QS9, GKD8: unsafe inputs end before the pre-check or storage.
	for _, name := range []string{"BinaryEscape", "BinaryAbsolute", "BinaryDirectory", "BinaryMissing", "BinaryFIFO", "BinaryLink", "SeedFIFO", "SeedLink", "DeclaredOversized", "DeclaredEscape", "ExecutableEscape"} {
		t.Run(name, func(t *testing.T) {
			o, d, _ := fixture(t)
			dir := filepath.Dir(o.SubjectFile)
			input := validSubject
			rule := "QS8"
			fifo := ""
			switch name {
			case "BinaryEscape", "BinaryAbsolute":
				// Use an owned sibling beneath another test directory.
				parent := t.TempDir()
				dir = filepath.Join(parent, "subject")
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
				o.SubjectFile = filepath.Join(dir, "subject.json")
				outside := filepath.Join(parent, "outside")
				if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
					t.Fatal(err)
				}
				binary := "../outside"
				if name == "BinaryAbsolute" {
					binary = outside
				}
				input = strings.Replace(input, "bin/subject", binary, 1)
			case "BinaryDirectory":
				input = strings.Replace(input, "bin/subject", "bin", 1)
			case "BinaryMissing":
				input = strings.Replace(input, "bin/subject", "missing", 1)
			case "BinaryFIFO":
				fifo = filepath.Join(dir, "pipe")
				input = strings.Replace(input, "bin/subject", "pipe", 1)
			case "BinaryLink":
				if err := os.Symlink("bin/subject", filepath.Join(dir, "link")); err != nil {
					t.Fatal(err)
				}
				input = strings.Replace(input, "bin/subject", "link", 1)
			case "SeedFIFO", "SeedLink":
				rule = "QS9"
				if err := os.Mkdir(filepath.Join(dir, "seed"), 0700); err != nil {
					t.Fatal(err)
				}
				input = strings.Replace(input, `"seed":null`, `"seed":"seed"`, 1)
				if name == "SeedFIFO" {
					fifo = filepath.Join(dir, "seed", "pipe")
				} else {
					if err := os.WriteFile(filepath.Join(dir, "seed", "regular"), []byte("seed"), 0600); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink("regular", filepath.Join(dir, "seed", "link")); err != nil {
						t.Fatal(err)
					}
				}
			default:
				file, executable := "large", "/bin/echo"
				if name == "DeclaredOversized" {
					if err := os.WriteFile(filepath.Join(dir, file), bytes.Repeat([]byte("x"), (16<<20)+1), 0600); err != nil {
						t.Fatal(err)
					}
				} else if name == "DeclaredEscape" {
					file = "../outside"
				} else {
					executable = "../outside"
					file = "bin/subject"
				}
				input = strings.Replace(input, `"runbooks":[]`, `"runbooks":[{"name":"book","steps":[{"name":"step","words":[],"budget_seconds":1,"checks":[{"dimension":"response-correctness","predicate":"stdout_equals","path":null,"expected":{"computation":{"argv":["`+executable+`"],"budget_seconds":1,"declared_files":["`+file+`"],"captures":[]}},"precondition":null,"exit_codes":null}]}]}]`, 1)
			}
			if fifo != "" {
				if err := syscall.Mkfifo(fifo, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(o.SubjectFile, []byte(input), 0600); err != nil {
				t.Fatal(err)
			}
			writes, judges := 0, 0
			d.Write = func(store.Record) error { writes++; return nil }
			if name != "BinaryLink" {
				d.Judge = func([]byte) ([]byte, int) { judges++; return nil, 0 }
			}
			type result struct {
				code   int
				err    error
				output string
			}
			done := make(chan result, 1)
			go func() { var out bytes.Buffer; code, err := Run(o, d, &out); done <- result{code, err, out.String()} }()
			var got result
			select {
			case got = <-done:
			case <-time.After(2 * time.Second):
				if fifo != "" {
					fd, err := syscall.Open(fifo, syscall.O_WRONLY|syscall.O_NONBLOCK, 0)
					if err == nil {
						if err := syscall.Close(fd); err != nil {
							t.Fatal(err)
						}
					}
					select {
					case <-done:
					case <-time.After(2 * time.Second):
						t.Fatal("read remains blocked")
					}
				}
				t.Fatal("safe read timed out; FIFO opened for reading")
			}
			if name == "BinaryLink" {
				if got.code != 4 || got.err != nil || writes != 1 {
					t.Fatalf("control: %+v writes=%d", got, writes)
				}
				return
			}
			var e *Error
			if got.code != 70 || !errors.As(got.err, &e) || e.Rule != rule || writes != 0 || judges != 0 || got.output != "" {
				t.Fatalf("got %+v writes=%d judges=%d want %s", got, writes, judges, rule)
			}
			if !strings.HasPrefix(got.err.Error(), "rejected: "+rule+":") {
				t.Fatal(got.err)
			}
			if fifo != "" {
				fd, err := syscall.Open(fifo, syscall.O_WRONLY|syscall.O_NONBLOCK, 0)
				if err == nil {
					if err := syscall.Close(fd); err != nil {
						t.Fatal(err)
					}
					t.Fatal("FIFO has a reader")
				}
				if !errors.Is(err, syscall.ENXIO) {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestSpecialFilesNeverOpened(t *testing.T) {
	// QS8, QS9: the shared named-file/seed reader refuses a FIFO before opening it.
	dir := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(dir, "pipe"), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	}()
	type result struct {
		calls int
		err   error
	}
	done := make(chan result, 1)
	go func() {
		calls := 0
		_, err := openRegular(root, "pipe", func(name string) (*os.File, error) {
			calls++
			return root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
		})
		done <- result{calls, err}
	}()
	select {
	case got := <-done:
		if got.err == nil || got.calls != 0 {
			t.Fatalf("opens=%d error=%v", got.calls, got.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("FIFO refusal timed out")
	}
}

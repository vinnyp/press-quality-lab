//go:build scripted

// Package caseassembly supplies the QT6 fixture layout to scripted tests only.
package caseassembly

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Case is the harness-owned part of QT6. The engine never receives this object.
type Case struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Input struct {
		SubjectFile string                                `json:"subject_file"`
		Form        string                                `json:"form"`
		Request     string                                `json:"request"`
		Verifier    string                                `json:"verifier"`
		Host        map[string]any                        `json:"host"`
		Baseline    *string                               `json:"baseline"`
		Fixtures    []struct{ Kind, Path, Detail string } `json:"fixtures"`
	} `json:"input"`
	Invocations []any          `json:"invocations"`
	Expect      map[string]any `json:"expect"`
}

// Read uses an independent JSON decoder, preserving numbers and rejecting tails.
func Read(path string, value any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	d := json.NewDecoder(f)
	d.UseNumber()
	decodeErr := d.Decode(value)
	if decodeErr == nil {
		var tail any
		if err := d.Decode(&tail); err != io.EOF {
			decodeErr = fmt.Errorf("trailing JSON: %v", err)
		}
	}
	closeErr := f.Close()
	if decodeErr != nil {
		return decodeErr
	}
	return closeErr
}

// Layout keeps all ambient paths outside Subject, including the durable store.
type Layout struct{ Subject, Home, Work, Config, Baseline, Trace string }

// Assemble copies only QT6's inputs; oracle files cannot enter Subject.
// Real-verifier execution must replace the sentinel with the built form.
func Assemble(t *testing.T, kit, source string, c Case) Layout {
	t.Helper()
	parent := t.TempDir()
	l := Layout{Subject: filepath.Join(parent, "subject"), Home: filepath.Join(parent, "home"), Work: filepath.Join(parent, "work"), Config: filepath.Join(parent, "config"), Trace: filepath.Join(parent, "trace.json")}
	for _, p := range []string{l.Subject, l.Home, l.Work, l.Config, filepath.Join(l.Subject, "bin")} {
		if err := os.MkdirAll(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(filepath.Join(source, c.Input.SubjectFile))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(l.Subject, c.Input.SubjectFile), b, 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"seed", "computations"} {
		sourcePath := filepath.Join(kit, "subjects", c.Input.Form, name)
		if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
			if err := os.Mkdir(filepath.Join(l.Subject, name), 0700); err != nil {
				t.Fatal(err)
			}
			continue
		} else if err != nil {
			t.Fatal(err)
		}
		if err := os.CopyFS(filepath.Join(l.Subject, name), os.DirFS(sourcePath)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(l.Subject, "bin/subject"), []byte("scripted corpus sentinel: "+c.ID+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, f := range c.Input.Fixtures {
		path := filepath.Join(l.Subject, f.Path)
		switch f.Kind {
		case "link":
			if err := os.Symlink(f.Detail, path); err != nil {
				t.Fatal(err)
			}
		case "special_file":
			if out, err := exec.Command("mkfifo", path).CombinedOutput(); err != nil {
				t.Fatalf("mkfifo: %v: %s", err, out)
			}
		case "oversized_output", "invalid_bytes", "planted_secret":
		default:
			t.Fatalf("unknown fixture kind %q", f.Kind)
		}
	}
	if c.Input.Baseline != nil {
		l.Baseline = filepath.Join(parent, "baseline.json")
		data, err := os.ReadFile(filepath.Join(source, *c.Input.Baseline))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(l.Baseline, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return l
}

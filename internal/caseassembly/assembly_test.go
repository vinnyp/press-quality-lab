//go:build scripted

package caseassembly

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func TestEveryCaseAssembles(t *testing.T) {
	kit := "../../docs/specs/conformance-collection-1.0"
	entries, err := os.ReadDir(filepath.Join(kit, "cases"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 152 {
		t.Fatalf("case count %d", len(entries))
	}
	for _, entry := range entries {
		t.Run(entry.Name(), func(t *testing.T) {
			source := filepath.Join(kit, "cases", entry.Name())
			var c Case
			if err := Read(filepath.Join(source, "case.json"), &c); err != nil {
				t.Fatal(err)
			}
			l := Assemble(t, kit, source, c)
			want := []string{c.Input.SubjectFile, "bin", "bin/subject", "seed", "computations"}
			for _, name := range []string{"seed", "computations"} {
				sourcePath := filepath.Join(kit, "subjects", c.Input.Form, name)
				if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
					continue
				} else if err != nil {
					t.Fatal(err)
				}
				err := filepath.WalkDir(filepath.Join(kit, "subjects", c.Input.Form, name), func(path string, d fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					rel, e := filepath.Rel(filepath.Join(kit, "subjects", c.Input.Form), path)
					if e != nil {
						return e
					}
					if rel != name {
						want = append(want, rel)
					}
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			for _, f := range c.Input.Fixtures {
				if f.Kind == "link" || f.Kind == "special_file" {
					want = append(want, f.Path)
				}
			}
			var got []string
			err := filepath.WalkDir(l.Subject, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				rel, e := filepath.Rel(l.Subject, path)
				if e != nil {
					return e
				}
				if rel != "." {
					got = append(got, rel)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			slices.Sort(want)
			slices.Sort(got)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("entries got %v want %v", got, want)
			}
			sentinel, err := os.ReadFile(filepath.Join(l.Subject, "bin/subject"))
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range got {
				path := filepath.Join(l.Subject, name)
				info, err := os.Lstat(path)
				if err != nil {
					t.Fatal(err)
				}
				if !info.Mode().IsRegular() || name == "bin/subject" {
					continue
				}
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if bytes.Equal(data, sentinel) {
					t.Fatalf("sentinel equals assembled file %s", name)
				}
				original := filepath.Join(kit, "subjects", c.Input.Form, name)
				if name == c.Input.SubjectFile {
					original = filepath.Join(source, name)
				}
				wantData, err := os.ReadFile(original)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(data, wantData) {
					t.Fatalf("copied bytes differ: %s", name)
				}
			}
			for _, name := range []string{"case.json", "input.json", "preimages.json"} {
				if _, err := os.Lstat(filepath.Join(l.Subject, name)); !os.IsNotExist(err) {
					t.Fatalf("oracle %s present or inaccessible: %v", name, err)
				}
			}
			for _, p := range []string{l.Home, l.Work, l.Config} {
				if filepath.Dir(p) != filepath.Dir(l.Subject) || p == l.Subject {
					t.Fatalf("ambient path inside subject: %s", p)
				}
			}
			if c.Input.Baseline != nil {
				if filepath.Dir(l.Baseline) != filepath.Dir(l.Subject) {
					t.Fatal("baseline inside subject")
				}
				gotData, err := os.ReadFile(l.Baseline)
				if err != nil {
					t.Fatal(err)
				}
				wantData, err := os.ReadFile(filepath.Join(source, *c.Input.Baseline))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(gotData, wantData) {
					t.Fatal("baseline copy differs")
				}
			} else if l.Baseline != "" {
				t.Fatal("unexpected baseline")
			}
			if filepath.Dir(l.Trace) != filepath.Dir(l.Subject) {
				t.Fatal("trace inside subject")
			}
			if c.ID == "case-112" {
				target, err := os.Readlink(filepath.Join(l.Subject, "seed/linked.json"))
				if err != nil || target != "data.json" {
					t.Fatalf("fixture %q: %v", target, err)
				}
			}
		})
	}
}

func TestFixtureKinds(t *testing.T) {
	kit := "../../docs/specs/conformance-collection-1.0"
	source := filepath.Join(kit, "cases/case-001")
	var c Case
	if err := Read(filepath.Join(source, "case.json"), &c); err != nil {
		t.Fatal(err)
	}
	c.Input.Fixtures = []struct{ Kind, Path, Detail string }{
		{"special_file", "seed/pipe", ""},
		{"oversized_output", "stdout", "descriptive"},
		{"invalid_bytes", "stderr", "descriptive"},
		{"planted_secret", "descriptive-only", "descriptive"},
	}
	l := Assemble(t, kit, source, c)
	info, err := os.Lstat(filepath.Join(l.Subject, "seed/pipe"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeNamedPipe == 0 {
		t.Fatalf("fixture is not a pipe: %v", info.Mode())
	}
	for _, name := range []string{"stdout", "stderr", "descriptive-only"} {
		if _, err := os.Lstat(filepath.Join(l.Subject, name)); !os.IsNotExist(err) {
			t.Fatalf("descriptive fixture materialized: %s (%v)", name, err)
		}
	}
}

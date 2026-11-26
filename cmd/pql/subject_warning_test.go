package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/grade"
)

func TestComputationWarning(t *testing.T) {
	// QA5: exact warning in both commands' help, terminal output and README.
	const warning = "Warning: grading another person's subject file runs their computations on this machine."
	if grade.ComputationWarning != warning {
		t.Fatal("warning changed")
	}
	for _, name := range []string{"grade", "anchor"} {
		t.Run(name+"Help", func(t *testing.T) {
			var out, err bytes.Buffer
			code := run([]string{name, "--help"}, bytes.NewReader(nil), &out, &err)
			if code != 0 || !strings.Contains(out.String(), warning+"\n") {
				t.Fatalf("%d %q %q", code, out.String(), err.String())
			}
		})
		t.Run(name+"Terminal", func(t *testing.T) {
			var out, err bytes.Buffer
			code := run([]string{name, filepath.Join(t.TempDir(), "missing.json")}, bytes.NewReader(nil), &out, &err)
			if code == 0 || !strings.HasPrefix(err.String(), warning+"\n") {
				t.Fatalf("%d %q", code, err.String())
			}
		})
	}
	b, e := os.ReadFile("../../README.md")
	if e != nil || !strings.Contains(string(b), warning+"\n") {
		t.Fatalf("README warning: %v", e)
	}
}

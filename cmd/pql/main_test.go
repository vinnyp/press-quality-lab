package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

// Rulebook §8 reserves these codes for judge verdicts.
var contractCodes = map[int]bool{0: true, 1: true, 2: true, 3: true, 4: true, 5: true}

func TestRun(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		code   int
		stdout bool
		stderr bool
	}{
		{"judge", []string{"judge"}, 2, true, false},
		{"unknown command", []string{"bogus"}, 64, false, true},
		{"root flag", []string{"--bogus"}, 64, false, true},
		{"judge flag", []string{"judge", "--bogus"}, 64, false, true},
		{"positional", []string{"judge", "extra-arg"}, 64, false, true},
		{"bare", nil, 64, false, true},
		{"help flag", []string{"--help"}, 0, true, false},
		{"short help flag", []string{"-h"}, 0, true, false},
		{"judge short help", []string{"judge", "-h"}, 0, false, true},
		{"judge help", []string{"judge", "--help"}, 0, false, true},
		{"help", []string{"help"}, 0, true, false},
		{"unknown help", []string{"help", "bogus"}, 64, false, true},
		{"extra help argument", []string{"help", "judge", "extra"}, 64, false, true},
		{"help judge", []string{"help", "judge"}, 0, true, false},
		{"completion", []string{cobra.ShellCompRequestCmd, ""}, 64, false, true},
		{"completion without descriptions", []string{cobra.ShellCompNoDescRequestCmd, ""}, 64, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, diagnostic bytes.Buffer
			code := run(tt.args, bytes.NewBufferString("unused"), &out, &diagnostic)
			if code != tt.code {
				t.Errorf("exit = %d, want %d", code, tt.code)
			}
			if (out.Len() > 0) != tt.stdout {
				t.Errorf("stdout = %q", out.String())
			}
			if (diagnostic.Len() > 0) != tt.stderr {
				t.Errorf("stderr = %q", diagnostic.String())
			}
			if tt.name != "judge" && tt.code != 0 && code >= 1 && code <= 5 {
				t.Errorf("error returned verdict code %d", code)
			}
		})
	}
}
func TestNonVerdictCodes(t *testing.T) {
	for _, code := range []int{exitUsage, exitSoftware} {
		if contractCodes[code] {
			t.Errorf("non-verdict code %d is reserved", code)
		}
	}
}
func TestProcessExit(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "pql")
	if output, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	for _, tt := range []struct {
		arg    string
		code   int
		stdout bool
		stderr bool
	}{{"judge", 2, true, false},
		{"bogus", 64, false, true},
		{"--help", 0, true, false}} {
		t.Run(tt.arg, func(t *testing.T) {
			var out, diagnostic bytes.Buffer
			cmd := exec.Command(binary, tt.arg)
			cmd.Stdout = &out
			cmd.Stderr = &diagnostic
			err := cmd.Run()
			var exit *exec.ExitError
			if err != nil && !errors.As(err, &exit) {
				t.Fatalf("expected process failure, got %v", err)
			}
			code := 0
			if exit != nil {
				code = exit.ExitCode()
			}
			if code != tt.code {
				t.Errorf("exit = %d, want %d", code, tt.code)
			}
			if (out.Len() > 0) != tt.stdout {
				t.Errorf("stdout = %q", out.String())
			}
			if (diagnostic.Len() > 0) != tt.stderr {
				t.Errorf("stderr = %q", diagnostic.String())
			}
		})
	}
}

type failingReader struct{}

func (failingReader) Read(p []byte) (int, error) {
	copy(p, []byte("{}"))
	return 2, errors.New("read failed")
}

type failingWriter struct{ calls int }

func (w *failingWriter) Write(p []byte) (int, error) {
	w.calls++
	return len(p) / 2, errors.New("write failed")
}
func TestJudgeDefectAndIO(t *testing.T) {
	original := judging
	defer func() { judging = original }()
	judging = func([]byte) ([]byte, int) { panic("injected defect") }
	var out, diagnostic bytes.Buffer
	if code := run([]string{"judge"}, bytes.NewBufferString("{}"), &out, &diagnostic); code != 70 || out.Len() != 0 || !bytes.Contains(diagnostic.Bytes(), []byte("injected defect")) || !bytes.Contains(diagnostic.Bytes(), []byte("goroutine")) {
		t.Fatalf("defect: %d %s %s", code, out.String(), diagnostic.String())
	}
	calls := 0
	judging = func([]byte) ([]byte, int) { calls++; return []byte("document\n"), 4 }
	out.Reset()
	diagnostic.Reset()
	if code := run([]string{"judge"}, failingReader{}, &out, &diagnostic); code != 70 || calls != 0 || out.Len() != 0 || !bytes.Contains(diagnostic.Bytes(), []byte("read failed")) {
		t.Fatalf("read: %d %s %s", code, out.String(), diagnostic.String())
	}
	w := &failingWriter{}
	diagnostic.Reset()
	if code := run([]string{"judge"}, bytes.NewBufferString("{}"), w, &diagnostic); code != 70 || w.calls != 1 || calls != 1 || !bytes.Contains(diagnostic.Bytes(), []byte("write failed")) {
		t.Fatalf("write: %d calls=%d %s", code, w.calls, diagnostic.String())
	}
}
func TestJudgeProcessDocuments(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "pql")
	if output, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	data, err := os.ReadFile("../../docs/specs/judge-1.0/examples/case-01.input.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		data []byte
		code int
	}{{"verdict", data, 4}, {"rejected", []byte("invalid"), 2}, {"oversized", bytes.Repeat([]byte(" "), 16777217), 2}} {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binary, "judge")
			cmd.Stdin = bytes.NewReader(tt.data)
			var out, diagnostic bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &diagnostic
			err := cmd.Run()
			code := 0
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				code = exit.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			if code != tt.code || diagnostic.Len() != 0 {
				t.Fatalf("code=%d stderr=%s", code, diagnostic.String())
			}
			var document struct {
				ExitCode int `json:"exit_code"`
			}
			if err := json.Unmarshal(out.Bytes(), &document); err != nil {
				t.Fatalf("stdout document: %v", err)
			}
			if document.ExitCode != code {
				t.Fatalf("document exit=%d process=%d", document.ExitCode, code)
			}
		})
	}
}

type countingReader struct{ n int }

func (r *countingReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}
	r.n += len(p)
	return len(p), nil
}
func TestJudgeReadLimitAndSingleWrite(t *testing.T) {
	original := judging
	defer func() { judging = original }()
	r := &countingReader{}
	seen := 0
	judging = func(b []byte) ([]byte, int) { seen = len(b); return []byte("complete document\n"), 3 }
	var out, diagnostic bytes.Buffer
	code := run([]string{"judge"}, r, &out, &diagnostic)
	if code != 3 || r.n != 16777217 || seen != 16777217 || out.String() != "complete document\n" || diagnostic.Len() != 0 {
		t.Fatalf("limit: %d read=%d seen=%d stdout=%q stderr=%q", code, r.n, seen, out.String(), diagnostic.String())
	}
}

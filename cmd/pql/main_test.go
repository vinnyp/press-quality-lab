package main

import (
	"bytes"
	"errors"
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
		{"judge", []string{"judge"}, 70, false, true},
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
			if tt.code != 0 && code >= 1 && code <= 5 {
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
	}{{"judge", 70, false, true},
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

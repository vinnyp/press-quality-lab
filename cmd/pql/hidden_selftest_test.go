package main

import (
	"os"
	"os/exec"
	"strconv"
	"testing"
)

func TestHiddenSelfTestArgumentCount(t *testing.T) {
	t.Chdir(t.TempDir())
	// KTD14: hidden modes have fixed positional layouts before Cobra. Every
	// malformed row returns without attempting a native self-test operation.
	for _, row := range []struct {
		name  string
		words []string
	}{
		{"SelfTestMissing", []string{"__pql_selftest"}},
		{"SelfTestShort", []string{"__pql_selftest", "clean", "target"}},
		{"SelfTestExtra", []string{"__pql_selftest", "clean", "target", "1", "extra"}},
		{"PidZero", []string{"__pql_selftest", "clean", "target", "0"}},
		{"PidNegative", []string{"__pql_selftest", "signal", "target", "-1"}},
		{"PidInvalid", []string{"__pql_selftest", "signal", "target", "x"}},
		{"DecoyExtra", []string{"__pql_decoy", "extra"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			if row.name == "DecoyExtra" {
				cmd := exec.Command(os.Args[0], "-test.run=^TestHiddenDecoyArgumentHelper$")
				cmd.Env = append(builderEnvironment(), "PQL_DECOY_ARGUMENT_HELPER=1")
				output, err := cmd.CombinedOutput()
				e, ok := err.(*exec.ExitError)
				if !ok || e.ExitCode() != 64 {
					t.Fatalf("exit=%v output=%q", err, output)
				}
				return
			}
			code, ok := hidden(row.words)
			if !ok || code != 64 {
				t.Fatalf("got %d %v want 64 true", code, ok)
			}
		})
	}
}

func TestHiddenDecoyArgumentHelper(t *testing.T) {
	// KTD14: run accidental decoy startup in an owned subprocess.
	if os.Getenv("PQL_DECOY_ARGUMENT_HELPER") != "1" {
		return
	}
	code, ok := hidden([]string{"__pql_decoy", "extra"})
	if !ok {
		t.Fatal("hidden mode not recognized: " + strconv.Itoa(code))
	}
	os.Exit(code)
}

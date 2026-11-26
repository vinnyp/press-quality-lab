package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestHiddenSelfTestClean(t *testing.T) {
	// KTD14: the reserved mode reaches the native program before Cobra.
	// This is an unconfined clean control, never a sandbox-exec invocation.
	path := filepath.Join(t.TempDir(), "clean")
	output, err := exec.Command(buildPQL(t), "__pql_selftest", "clean", path, "1").CombinedOutput()
	if err != nil || string(output) != "clean_value\n" {
		t.Fatalf("error=%v output=%q", err, output)
	}
}

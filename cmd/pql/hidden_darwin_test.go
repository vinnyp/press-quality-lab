package main

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCouldNotStartProtocol(t *testing.T) {
	marker := strings.Repeat("a", 64)
	cmd := exec.Command(buildPQL(t), "__pql_launcher", marker, filepath.Join(t.TempDir(), "absent"), "[]", "[]")
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	want := "pql-launch:" + marker + "\npql-could-not-start:" + marker + "\n"
	if !errors.As(err, &exit) || exit.ExitCode() != 72 || string(out) != want {
		t.Fatalf("exec failure: %v output=%q", err, out)
	}
}

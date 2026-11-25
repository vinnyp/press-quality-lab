package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func buildPQL(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "pql")
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, b)
	}
	return binary
}
func TestHiddenProtocol(t *testing.T) {
	binary := buildPQL(t)
	for _, args := range [][]string{{"__pql_unknown"}, {"__pql_fast"}, {"__pql_launcher"}, {"__pql_fast", strings.Repeat("0", 64), "extra"}, {"__pql_launcher", strings.Repeat("0", 64), "program", "invalid-json"}} {
		cmd := exec.Command(binary, args...)
		b, err := cmd.CombinedOutput()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 64 || len(b) != 0 {
			t.Fatalf("args %v: exit=%v output=%q", args, err, b)
		}
	}
}
func TestLauncherInitialisation(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux initialisation baseline")
	}
	cmd := exec.Command(buildPQL(t), "__pql_launcher", strings.Repeat("0", 64), "/nonexistent", "[]")
	cmd.Env = []string{"GODEBUG=inittrace=1"}
	b, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 70 {
		t.Fatalf("unsupported launcher: %v %s", err, b)
	}
	var packages []string
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "init ") {
			packages = append(packages, strings.Fields(line)[1])
		}
	}
	want, err := os.ReadFile("testdata/launcher-linux-init.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, []byte(strings.Join(packages, "\n")+"\n")) {
		t.Fatalf("launcher initialisation changed: %q", packages)
	}
}

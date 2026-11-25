package main

import (
	"errors"
	"fmt"
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
	cmd.Env = append(builderEnvironment(), "CGO_ENABLED=0")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, b)
	}
	return binary
}
func TestHiddenProtocol(t *testing.T) {
	binary := buildPQL(t)
	for i, args := range [][]string{{"__pql_launcher", strings.Repeat("0", 64), "program", "[\"\xff\"]", "[]"}, {"__pql_unknown"}, {"__pql_unknown", strings.Repeat("a", 64), "program", "[]", "[]"}, {"__pql_fast", strings.Repeat("a", 63)}, {"__pql_fast", strings.Repeat("A", 64)}, {"__pql_fast", strings.Repeat("g", 64)}, {"__pql_fast"}, {"__pql_launcher"}, {"__pql_fast", strings.Repeat("0", 64), "extra"}, {"__pql_launcher", strings.Repeat("0", 64), "program", "invalid-json", "[]"},
		{"__pql_launcher", strings.Repeat("0", 64), "program", "[]", "invalid-json"},
		{"__pql_launcher", strings.Repeat("0", 64), "program", `["\u0000"]`, "[]"},
		{"__pql_launcher", strings.Repeat("0", 64), "program", "[]", `["A=\u0000"]`},
		{"__pql_launcher", strings.Repeat("0", 64), "program", "[]", "[\"A=\xff\"]"}} {
		t.Run(fmt.Sprintf("Row%d", i), func(t *testing.T) {
			cmd := exec.Command(binary, args...)
			b, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 64 || len(b) != 0 {
				t.Fatalf("args %v: exit=%v output=%q", args, err, b)
			}
		})
	}
}
func TestLauncherInitialisation(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("initialisation baseline is for linux/amd64")
	}
	cmd := exec.Command(buildPQL(t), "__pql_launcher", strings.Repeat("0", 64), "/nonexistent", "[]", "[]")
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
	if strings.Join(linuxInitialisationList(t), "\n") != strings.Join(packages, "\n") {
		t.Fatalf("launcher initialisation changed: %q", packages)
	}
}

func builderEnvironment() []string {
	var env []string
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "GOFLAGS=") {
			env = append(env, value)
		}
	}
	return append(env, "GOFLAGS=-mod=readonly")
}

func TestBuilderEnvironment(t *testing.T) {
	t.Setenv("GOFLAGS", "-tags=scripted")
	var flags []string
	for _, entry := range builderEnvironment() {
		if strings.HasPrefix(entry, "GOFLAGS=") {
			flags = append(flags, entry)
		}
	}
	if len(flags) != 1 || flags[0] != "GOFLAGS=-mod=readonly" {
		t.Fatalf("builder inherited GOFLAGS: %q", flags)
	}
}

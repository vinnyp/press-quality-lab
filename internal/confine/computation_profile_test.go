package confine

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestComputationProfileRules(t *testing.T) {
	// QH16, QH17, QH18, QH19, QH20: the computation narrows the shared template;
	// in particular the final subject denial must follow system-tool allowances.
	profile := render(true, false, 0, 2, 1)
	lines := []string{
		`(allow process-fork)`,
		`(allow signal (target same-sandbox))`,
		`(deny syscall-unix (syscall-number SYS_setsid SYS_setpgid) (with send-signal SIGKILL))`,
		`(deny process-info* (with send-signal SIGKILL))`,
		`(allow process-info* (target same-sandbox))`,
		`(allow process-exec* file-read-data (subpath (param "BIN_DIR")) (subpath (param "USR_BIN_DIR")))`,
		`(allow file-read-data file-write-data (literal (param "TTY_DEVICE")))`,
		`(allow file-read-data (literal (param "DECLARED_1")))`,
		`(allow file-read-data (literal (param "DECLARED_2")))`,
		`(allow file-read-data (literal (param "SEED_1")))`,
		`(allow file-read* (subpath (param "HOME")))`,
		`(allow file-write* (subpath (param "HOME")))`,
		`(deny file-write* (literal (param "PLANTED")) (with send-signal SIGKILL))`,
		`(deny file-write-unlink file-write-create (literal (param "HOME")) (with send-signal SIGKILL))`,
		`(deny file-read-data process-exec* (literal (param "SUBJECT")) (literal (param "SUBJECT_COPY")) (with send-signal SIGKILL))`,
	}
	previous := -1
	for _, line := range lines {
		at := strings.Index(profile, line+"\n")
		if at <= previous || strings.Count(profile, line+"\n") != 1 {
			t.Fatalf("missing, repeated or reordered rule %q", line)
		}
		previous = at
	}
	if !strings.HasPrefix(profile, "(version 1)\n(deny default (with send-signal SIGKILL))\n") {
		t.Fatal("default network refusal lost")
	}
	for _, line := range strings.Split(profile, "\n") {
		if strings.HasPrefix(line, "(allow network") || strings.Contains(line, "{{") || strings.Contains(line, "_n\"") {
			t.Fatalf("unexpected rule %q", line)
		}
	}
}

func TestComputationSandboxPaths(t *testing.T) {
	// QH16, QH17, QH19: each parameter keeps its own canonical path, including
	// distinct scratch, subject, private-copy, declared-file and seed grants.
	root := t.TempDir()
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	names := []string{
		"ROOT",
		"RUNTIME",
		"VOLUME",
		"HOME",
		"LAUNCHER",
		"BIN",
		"LAUNCHER_PARENT",
		"BIN_PARENT",
		"TRACE_DEVICE",
		"NULL_DEVICE",
		"RANDOM_DEVICE",
		"ZERO_DEVICE",
		"STDOUT_DEVICE",
		"STDERR_DEVICE",
		"ZONEFILE",
		"ZONE_RULES",
		"SERVICES",
		"PROTOCOLS",
		"PLANTED",
		"BIN_DIR",
		"USR_BIN_DIR",
		"TTY_DEVICE",
		"SUBJECT",
		"SUBJECT_COPY",
	}
	paths := Paths{Values: map[string]string{}}
	want := map[string]string{}
	for _, name := range append(append([]string{}, names...), "DECLARED_1", "SEED_1") {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
		want[name] = filepath.Join(resolved, name)
		if name == "DECLARED_1" {
			paths.Declared = []string{filepath.Join(link, name)}
		} else if name == "SEED_1" {
			paths.Seed = []string{filepath.Join(link, name)}
		} else {
			paths.Values[name] = filepath.Join(link, name)
		}
	}
	args, err := Arguments(true, false, paths, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for i := 2; i < len(args)-1; i += 2 {
		if args[i] != "-D" {
			t.Fatalf("argument=%q", args[i])
		}
		name, value, ok := strings.Cut(args[i+1], "=")
		if !ok {
			t.Fatal("parameter without value")
		}
		if _, exists := got[name]; exists {
			t.Fatalf("duplicate %s", name)
		}
		got[name] = value
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parameters=%v want=%v", got, want)
	}
	if args[len(args)-1] != filepath.Join(resolved, "LAUNCHER") {
		t.Fatal("wrong launcher path")
	}
}

package collectionkit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

func subjectHash(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
func subjectGo(t *testing.T, root string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOFLAGS=-mod=readonly")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %v: %s", args, out)
	}
	return out
}
func subjectCopy(t *testing.T, source, destination string) {
	t.Helper()
	if err := os.CopyFS(destination, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}
}
func subjectSourceDigest(t *testing.T, root, tag string) string {
	t.Helper()
	if tag == "form_not_executable" {
		return subjectHash(read(t, filepath.Join(root, "testdata/collection-subject/not-executable.txt")))
	}
	var info struct{ GoFiles []string }
	decode(t, subjectGo(t, root, "list", "-tags", tag, "-json", "./testdata/collection-subject"), &info)
	if len(info.GoFiles) == 0 {
		t.Fatal("no compiled form source")
	}
	slices.Sort(info.GoFiles)
	entries := make([][3]any, 0, len(info.GoFiles))
	for _, name := range info.GoFiles {
		b := read(t, filepath.Join(root, "testdata/collection-subject", name))
		if bytes.Contains(b, []byte("operation-ready")) {
			t.Fatalf("form source contains reserved harness line in %s", name)
		}
		entries = append(entries, [3]any{name, "file", subjectHash(b)})
	}
	// File names and kinds are ASCII; array-only JSON needs no object ordering.
	b, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	return subjectHash(b)
}

type subjectRecording struct {
	FormSHA     string `json:"form_sha256"`
	Invocations []struct {
		Key struct {
			Words       []string `json:"words"`
			OfflineArgs []string `json:"offline_args"`
			Env         []string
			Runbook     *string
			Home        string
			Computation []string
		} `json:"key"`
		Confined []struct {
			Stdout struct {
				Bytes  int
				SHA256 string
			}
			End struct {
				Kind string
				Code int
			}
		}
	} `json:"invocations"`
}

// Reject forbidden content in decoded strings as well as JSON source text.
func subjectRecordingGuard(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	requirement := regexp.MustCompile(`\b(?:R|AE)[0-9]+\b`)
	canary := regexp.MustCompile(`pqlcanary[0-9a-fA-F]{32}`)
	absolute := regexp.MustCompile(`(^|[[:space:]"'=:(])(/[[:alnum:]_.-][^[:space:]"'<>),;]*)`)
	systemExecutable := regexp.MustCompile(`^/(usr/)?bin/[^/]+$`)
	var walk func(any, bool) error
	walk = func(v any, computation bool) error {
		switch x := v.(type) {
		case string:
			if requirement.MatchString(x) {
				return fmt.Errorf("recording contains a bare requirement number")
			}
			for _, s := range canary.FindAllString(x, -1) {
				if s != "pqlcanary"+strings.Repeat("0", 32) {
					return fmt.Errorf("recording contains an unmasked canary")
				}
			}
			if (filepath.IsAbs(x) || absolute.MatchString(x)) && !(computation && systemExecutable.MatchString(x)) {
				return fmt.Errorf("recording contains a forbidden absolute path")
			}
		case []any:
			for i, c := range x {
				if err := walk(c, computation && i == 0); err != nil {
					return err
				}
			}
		case map[string]any:
			for k, c := range x {
				if k == "kept" {
					if encoded, ok := c.(string); ok {
						decoded, err := base64.StdEncoding.DecodeString(encoded)
						if err != nil {
							return fmt.Errorf("invalid kept stream encoding")
						}
						if err := walk(string(decoded), false); err != nil {
							return err
						}
					}
				}
				if err := walk(k, false); err != nil {
					return err
				}
				if err := walk(c, k == "computation"); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(v, false)
}

func TestCollectionSubject(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(root, "testdata/collection-subject/*.go"))
	if err != nil {
		t.Fatal(err)
	}
	tags := map[string]bool{}
	pattern := regexp.MustCompile(`(?m)^//go:build (form_[a-z0-9_]+)$`)
	for _, file := range files {
		if match := pattern.FindSubmatch(read(t, file)); match != nil {
			tags[string(match[1])] = true
		}
	}
	if len(tags) == 0 {
		t.Fatal("no subject forms")
	}
	tags["form_not_executable"] = true
	for tag := range tags {
		t.Run(tag, func(t *testing.T) {
			form := strings.ReplaceAll(strings.TrimPrefix(tag, "form_"), "_", "-")
			digest := subjectSourceDigest(t, root, tag)
			binary := filepath.Join(t.TempDir(), "widgets")
			if form == "not-executable" {
				if err := os.WriteFile(binary, read(t, filepath.Join(root, "testdata/collection-subject/not-executable.txt")), 0644); err != nil {
					t.Fatal(err)
				}
				if err := exec.Command(binary).Run(); err == nil {
					t.Fatal("not-executable started")
				}
			} else {
				var deps []struct {
					ImportPath string
					Standard   bool
				}
				dec := json.NewDecoder(bytes.NewReader(subjectGo(t, root, "list", "-deps", "-tags", tag, "-json", "./testdata/collection-subject")))
				for dec.More() {
					var dep struct {
						ImportPath string
						Standard   bool
					}
					if err := dec.Decode(&dep); err != nil {
						t.Fatal(err)
					}
					deps = append(deps, dep)
				}
				for _, dep := range deps {
					if !dep.Standard && dep.ImportPath != "github.com/vinnyp/press-quality-lab/testdata/collection-subject" {
						t.Fatalf("non-standard subject import: %s", dep.ImportPath)
					}
				}
				subjectGo(t, root, "build", "-tags", tag, "-o", binary, "./testdata/collection-subject")
				if form == "known-good" {
					testKnownGoodSubject(t, binary)
				} else {
					testDefectSubject(t, binary, form)
				}
			}
			recordingFile := filepath.Join(root, "docs/specs/conformance-collection-1.0/behaviour", form+".json")
			b, err := os.ReadFile(recordingFile)
			if os.IsNotExist(err) {
				t.Fatalf("missing recording behaviour/%s.json; record on macOS before committing", form)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := subjectRecordingGuard(b); err != nil {
				t.Fatal(err)
			}
			var recording subjectRecording
			decode(t, b, &recording)
			if err := subjectDigestGuard(digest, recording.FormSHA); err != nil {
				t.Fatal(err)
			}
			if len(recording.Invocations) == 0 {
				t.Fatal("recording has no invocations")
			}
			var plan struct {
				Invocations []struct{ Key map[string]any }
			}
			decode(t, read(t, filepath.Join(root, "testdata/collection-subject/plans", form+".json")), &plan)
			var recorded struct {
				Invocations []struct{ Key map[string]any }
			}
			decode(t, b, &recorded)
			for _, planned := range plan.Invocations {
				found := false
				for _, actual := range recorded.Invocations {
					if corpusEqual(planned.Key, actual.Key) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("missing recording for %s invocation %v", form, planned.Key)
				}
			}
			for _, entry := range recording.Invocations {
				key := entry.Key
				help := len(key.Words) > 0 && key.Words[len(key.Words)-1] == "--help"
				benign := key.Computation == nil && (help || len(key.Words) == 0 || key.Runbook == nil && (key.Words[0] == "list" || key.Words[0] == "count"))
				if !benign {
					continue
				}
				if len(entry.Confined) != 5 {
					t.Fatal("recording needs five confined runs")
				}
				home := t.TempDir()
				if !help {
					subjectCopy(t, filepath.Join(root, "docs/specs/conformance-collection-1.0/subjects", form, "seed"), home)
				}
				first := entry.Confined[0]
				// Signal and budget endings are checked by the macOS recorder.
				if first.End.Kind != "exit" {
					continue
				}
				args := append(slices.Clone(key.Words), key.OfflineArgs...)
				out, _, code := subjectInvoke(t, binary, home, args...)
				if len(out) != first.Stdout.Bytes || subjectHash(out[:min(len(out), 1<<20)]) != first.Stdout.SHA256 || first.End.Kind != "exit" || code != first.End.Code {
					t.Errorf("direct invocation %v differs from recording", args)
				}
			}
		})
	}
}

func subjectInvoke(t *testing.T, binary, home string, args ...string) ([]byte, []byte, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TZ=UTC"}
	var out, diagnostic bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &diagnostic
	err := cmd.Run()
	code := 0
	if err != nil {
		exit, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatal(err)
		}
		code = exit.ExitCode()
	}
	return out.Bytes(), diagnostic.Bytes(), code
}
func testKnownGoodSubject(t *testing.T, binary string) {
	t.Helper()
	testSubjectHelp(t, binary)
	seed := filepath.Join(kit, "subjects/known-good/seed")
	tokenBytes := len(read(t, filepath.Join(seed, ".config/widgets/token")))
	for _, row := range []struct {
		args           []string
		stdout, stderr string
		code           int
		empty          bool
	}{
		{[]string{"account"}, fmt.Sprintf("{\"account\":\"ok\",\"token_bytes\":%d}\n", tokenBytes), "", 0, false},
		{[]string{"count"}, "{\"count\":3}\n", "", 0, false},
		{[]string{"list", "--json", "--tag", "absent"}, "{\"source\":\"local\",\"truncated\":false,\"widgets\":[]}\n", "", 0, false},
		{[]string{"--offline", "count", "--tag", "metal"}, "{\"count\":2}\n", "", 0, false},
		{[]string{"list", "--tag", "brass", "--offline"}, "valve\n", "", 0, false},
		{[]string{"list", "--json", "--limit", "1", "--tag", "metal"}, "{\"source\":\"local\",\"truncated\":true,\"widgets\":[{\"name\":\"gear\",\"tag\":\"metal\"}]}\n", "", 0, false},
		{[]string{"list", "--limit", "0"}, "", "widgets: invalid value for --limit: \"0\"\n", 2, false},
		{[]string{"list", "--bogus"}, "", "widgets: unknown flag: --bogus\n", 2, false},
		{[]string{"add"}, "", "widgets: add needs a name\n", 2, false},
		{[]string{"remove", "gear"}, "", "widgets: refusing to remove \"gear\" without --confirm\n", 3, false},
		{[]string{"account"}, "", "widgets: no token\n", 4, true},
		{[]string{"count"}, "", "widgets: no store\n", 5, true},
	} {
		t.Run(strings.Join(row.args, "_"), func(t *testing.T) {
			home := t.TempDir()
			if !row.empty {
				subjectCopy(t, seed, home)
			}
			out, errout, code := subjectInvoke(t, binary, home, row.args...)
			if string(out) != row.stdout || string(errout) != row.stderr || code != row.code {
				t.Fatalf("got stdout=%q stderr=%q exit=%d", out, errout, code)
			}
		})
	}
	t.Run("writes", func(t *testing.T) {
		home := t.TempDir()
		subjectCopy(t, seed, home)
		original := read(t, filepath.Join(home, "data.json"))
		_, _, code := subjectInvoke(t, binary, home, "remove", "gear")
		if code != 3 || !bytes.Equal(original, read(t, filepath.Join(home, "data.json"))) {
			t.Fatal("unconfirmed remove changed store")
		}
		for i, want := range []string{"{\"added\":true,\"name\":\"sprocket\"}\n", "{\"added\":false,\"name\":\"sprocket\"}\n"} {
			out, _, code := subjectInvoke(t, binary, home, "add", "sprocket", "--tag", "steel")
			if string(out) != want || code != 0 {
				t.Fatalf("add %d: %q exit=%d", i, out, code)
			}
		}
		want := strings.TrimSuffix(string(original), "]}\n")
		want = strings.TrimSuffix(want, "\n") + ",\n{\"name\":\"sprocket\",\"tag\":\"steel\"}\n]}\n"
		if string(read(t, filepath.Join(home, "data.json"))) != want {
			t.Fatal("store rewrite layout or idempotency differs")
		}
		out, _, code := subjectInvoke(t, binary, home, "remove", "gear", "--confirm")
		if string(out) != "{\"removed\":true}\n" || code != 0 {
			t.Fatal("confirmed remove failed")
		}
		subjectInvoke(t, binary, home, "list")
		if string(read(t, filepath.Join(home, ".local/state/widgets/last-command"))) != "list\n" {
			t.Fatal("missing list bookkeeping")
		}
	})
}

// Scratch trees exercise the guard without mutating repository files.
func TestSubjectRecordingGuards(t *testing.T) {
	for _, row := range []struct {
		name, text string
		bad        bool
	}{
		{"absolute", `{"computation":["/private/tool"]}`, true},
		{"embedded", `{"head":"open /private/input: refused"}`, true},
		{"kept", `{"kept":"` + base64.StdEncoding.EncodeToString([]byte("open /private/input")) + `"}`, true},
		{"system", `{"computation":["/bin/sh"]}`, false},
		{"argument", `{"computation":["/bin/sh","/private/input"]}`, true},
		{"requirement", `{"head":"R` + `42"}`, true},
		{"masked", `{"head":"pqlcanary` + strings.Repeat("0", 32) + `"}`, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "recording.json")
			if err := os.WriteFile(path, []byte(row.text), 0600); err != nil {
				t.Fatal(err)
			}
			err := subjectRecordingGuard(read(t, path))
			if (err != nil) != row.bad {
				t.Fatalf("guard error=%v", err)
			}
			if row.bad {
				t.Logf("broken scratch recording rejected: %v", err)
			}
		})
	}
}

func subjectDigestGuard(actual, recorded string) error {
	if actual != recorded {
		return fmt.Errorf("form source digest differs from recording")
	}
	return nil
}
func TestSubjectDigestGuards(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "testdata/collection-subject")
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"main.go", "form_known_good.go"} {
		if err := os.WriteFile(filepath.Join(source, name), read(t, filepath.Join("../../testdata/collection-subject", name)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module subject.test\n\ngo 1.27.3\n"), 0600); err != nil {
		t.Fatal(err)
	}
	original := subjectSourceDigest(t, root, "form_known_good")
	if err := subjectDigestGuard(original, original); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(source, "form_known_good.go")
	b := read(t, path)
	if err := os.WriteFile(path, bytes.Replace(b, []byte(`"local"`), []byte(`"remote"`), 1), 0600); err != nil {
		t.Fatal(err)
	}
	changed := subjectSourceDigest(t, root, "form_known_good")
	if err := subjectDigestGuard(changed, original); err == nil {
		t.Fatal("changed scratch form escaped source guard")
	} else {
		t.Logf("changed scratch form rejected: %v", err)
	}
	var recording struct {
		Digest string `json:"form_sha256"`
	}
	b, err := json.Marshal(map[string]string{"form_sha256": strings.Repeat("0", 64)})
	if err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(root, "recording.json")
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	decode(t, read(t, path), &recording)
	if err := subjectDigestGuard(original, recording.Digest); err == nil {
		t.Fatal("changed scratch recording digest escaped guard")
	} else {
		t.Logf("changed scratch recording digest rejected: %v", err)
	}
}

func testSubjectHelp(t *testing.T, binary string) {
	t.Helper()
	const expected = `widgets keeps a local list of widgets.

Usage:
  widgets [command]

Available Commands:
  account     Show the account the token belongs to
  add         Add a widget
  count       Count widgets
  list        List widgets
  remove      Remove a widget (destructive)

Flags:
  -h, --help   help for widgets
`
	for _, args := range [][]string{{}, {"--help"}, {"widgets", "--help"}} {
		out, stderr, code := subjectInvoke(t, binary, t.TempDir(), args...)
		if string(out) != expected || len(stderr) != 0 || code != 0 {
			t.Fatalf("root help %v differs: stdout=%q stderr=%q code=%d", args, out, stderr, code)
		}
	}
	for _, command := range []string{"account", "add", "count", "list", "remove"} {
		home := t.TempDir()
		out, stderr, code := subjectInvoke(t, binary, home, command, "--help")
		if !bytes.Contains(out, []byte("Usage:\n")) || !bytes.Contains(out, []byte("Flags:\n")) || bytes.Contains(out, []byte("Commands:")) || len(stderr) != 0 || code != 0 {
			t.Fatalf("leaf help %s differs", command)
		}
		entries, err := os.ReadDir(home)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("help %s wrote to empty home", command)
		}
	}
}

func testDefectSubject(t *testing.T, binary, form string) {
	t.Helper()
	switch form {
	case "tries-network", "live-first", "starts-child", "listens", "ignores-home", "cache-outside-home", "reads-key-file", "signals-process", "reads-process-env", "forges-atime", "links-decoy":
		// Only the recorder owns these forms' decoys. Direct checks exercise help;
		// the recorder checks the act and its refusal with disposable targets.
		testSubjectHelp(t, binary)
		return
	}

	home := t.TempDir()
	subjectCopy(t, filepath.Join(kit, "subjects", form, "seed"), home)
	args := []string{"list", "--json"}
	want := ""
	switch form {
	case "drops-last":
		want = `{"source":"local","truncated":false,"widgets":[{"name":"gear","tag":"metal"},{"name":"spring","tag":"metal"}]}` + "\n"
	case "ignores-store":
		want = `{"source":"local","truncated":false,"widgets":[]}` + "\n"
	case "big-number":
		args = []string{"count"}
		want = `{"count":123456789012345678901234567890123}` + "\n"
	case "never-reads-secret", "opens-secret":
		args = []string{"account"}
		want = `{"account":"ok","token_bytes":19}` + "\n"
	case "ignores-help":
		args = []string{"add", "--help"}
		want = `{"added":true,"name":"help"}` + "\n"
	case "invalid-utf8":
		args = []string{"list"}
		want = "gear\xff\nspring\xff\nvalve\xff\n"
	case "huge-output":
		args = []string{"list"}
	case "hides-command", "renames-command", "adds-command", "help-urfave", "help-clap", "help-cobra-group", "help-fails", "help-manpage", "help-no-section", "help-deep", "help-cycle":
		args = []string{"--help"}
	case "crashes", "kills-self":
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, "count")
		cmd.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin"}
		output, err := cmd.CombinedOutput()
		if err == nil || ctx.Err() != nil {
			t.Fatal("count did not signal within budget", err)
		}
		status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
		want := syscall.SIGKILL
		if form == "crashes" {
			want = syscall.SIGABRT
		}
		if !ok || !status.Signaled() || status.Signal() != want || len(output) != 0 {
			t.Fatalf("count signal=%v output=%q", status, output)
		}
		return
	case "overruns":
		return // The recorder checks the budget ending.
	default:

		if strings.HasPrefix(form, "leak-") {
			args = []string{"account"}
		}
	}
	out, diagnostic, code := subjectInvoke(t, binary, home, args...)
	expectedCode := 0
	if form == "help-fails" {
		expectedCode = 1
	}
	if code != expectedCode || len(diagnostic) != 0 {
		t.Fatalf("form %s: code=%d diagnostic=%q", form, code, diagnostic)
	}
	if want != "" && string(out) != want {
		t.Fatalf("form %s output=%q", form, out)
	}
	switch form {
	case "huge-output":
		line := []byte(strings.Repeat("w", 63) + "\n")
		if len(out) != 17<<20 || !bytes.Equal(out, bytes.Repeat(line, (17<<20)/64)) {
			t.Fatal("huge-output differs from exact line pattern or size")
		}
	case "creates-file":
		if string(read(t, filepath.Join(home, "notes.txt"))) != "seen\n" {
			t.Fatal("notes differs")
		}
	case "creates-link":
		target, err := os.Readlink(filepath.Join(home, "latest"))
		if err != nil || target != "data.json" {
			t.Fatal("link differs", err)
		}
	case "writes-wal", "writes-empty-wal", "writes-shm":
		name, size := "data.json-wal", 8
		if form == "writes-empty-wal" {
			size = 0
		}
		if form == "writes-shm" {
			name = "data.json-shm"
			size = 32768
		}
		if len(read(t, filepath.Join(home, name))) != size {
			t.Fatal("side-file size differs")
		}
	case "writes-store":
		var state struct{ Widgets []struct{ Name string } }
		decode(t, read(t, filepath.Join(home, "data.json")), &state)
		if len(state.Widgets) != 3 || state.Widgets[0].Name != "valve" || state.Widgets[2].Name != "gear" {
			t.Fatal("store was not reversed")
		}
	case "hides-command":
		if bytes.Contains(out, []byte("  count ")) {
			t.Fatal("count still listed")
		}
	case "renames-command":
		if !bytes.Contains(out, []byte("  tally ")) || bytes.Contains(out, []byte("  count ")) {
			t.Fatal("renamed help differs")
		}
		count, errout, exit := subjectInvoke(t, binary, home, "count")
		if exit != 2 || len(count) != 0 || string(errout) != "widgets: unknown command: count\n" {
			t.Fatal("old name still works")
		}
		count, _, exit = subjectInvoke(t, binary, home, "tally")
		if exit != 0 || string(count) != "{\"count\":3}\n" {
			t.Fatal("new name differs")
		}
	case "adds-command":
		if !bytes.Contains(out, []byte("  export      Export widgets\n")) {
			t.Fatal("export not listed")
		}
		exported, _, exit := subjectInvoke(t, binary, home, "export")
		listed, _, _ := subjectInvoke(t, binary, home, "list", "--json")
		if exit != 0 || !bytes.Equal(exported, listed) {
			t.Fatal("export differs from JSON list")
		}
	}
	if strings.HasPrefix(form, "help-") {
		want := []string{"account", "add", "count", "list", "remove"}
		switch form {
		case "help-cobra-group":
			want = []string{"add", "count", "list", "remove", "account"}
		case "help-manpage", "help-no-section":
			want = nil
		case "help-deep":
			want = []string{"a"}
		case "help-cycle":
			want = []string{"x"}
		}
		if got := subjectHelpCommands(string(out)); !slices.Equal(got, want) {
			t.Fatalf("help commands=%v want=%v", got, want)
		}
		if form == "help-deep" || form == "help-cycle" {
			next := []string{"a", "a", "--help"}
			expected := "a"
			if form == "help-cycle" {
				next = []string{"x", "y", "--help"}
				expected = "x"
			}
			nested, _, exit := subjectInvoke(t, binary, t.TempDir(), next...)
			if exit != 0 || !slices.Equal(subjectHelpCommands(string(nested)), []string{expected}) {
				t.Fatal("recursive help stopped or chose wrong child")
			}
		}
	}
	if form == "drops-last" {
		limited, _, exit := subjectInvoke(t, binary, home, "list", "--json", "--limit", "2")
		if exit != 0 || string(limited) != `{"source":"local","truncated":true,"widgets":[{"name":"gear","tag":"metal"},{"name":"spring","tag":"metal"}]}`+"\n" {
			t.Fatal("drop changed pre-drop truncation or removed a widget before the store's last")
		}
		plain, _, exit := subjectInvoke(t, binary, home, "list", "--tag", "brass")
		if exit != 0 || len(plain) != 0 {
			t.Fatal("filtered plain list kept the last widget")
		}
	}
	if strings.HasPrefix(form, "leak-") {
		token := read(t, filepath.Join(home, ".config/widgets/token"))
		switch form {
		case "leak-file":
			if !bytes.Equal(read(t, filepath.Join(home, ".cache/widgets/debug.log")), token) {
				t.Fatal("debug file differs")
			}
		case "leak-past-bound":
			data := read(t, filepath.Join(home, ".cache/widgets/big.log"))
			if len(data) != (16<<20)+len(token) || !bytes.Equal(data[:16<<20], bytes.Repeat([]byte("x"), 16<<20)) || !bytes.Equal(data[16<<20:], token) {
				t.Fatal("past-bound file differs")
			}
		default:
			var account struct{ Debug string }
			decode(t, out, &account)
			text := strings.TrimSuffix(string(token), "\n")
			var decoded []byte
			var err error
			switch strings.TrimPrefix(form, "leak-") {
			case "plain":
				decoded = []byte(account.Debug)
			case "base64":
				decoded, err = base64.StdEncoding.DecodeString(account.Debug)
			case "base64-nopad":
				decoded, err = base64.RawStdEncoding.DecodeString(account.Debug)
			case "base64url":
				decoded, err = base64.URLEncoding.DecodeString(account.Debug)
			case "base64url-nopad":
				decoded, err = base64.RawURLEncoding.DecodeString(account.Debug)
			case "hex-lower", "hex-upper":
				decoded, err = hex.DecodeString(account.Debug)
			case "percent-lower", "percent-upper":
				decoded, err = hex.DecodeString(strings.ReplaceAll(account.Debug, "%", ""))
			}
			if err != nil || string(decoded) != text {
				t.Fatal("debug encoding differs", err)
			}
		}
	}
}

func TestSubjectPlanCaseSubset(t *testing.T) {
	var plan struct {
		Invocations []struct{ Key map[string]any }
	}
	decode(t, read(t, "../../testdata/collection-subject/plans/known-good.json"), &plan)
	keys := map[string]bool{}
	for _, entry := range plan.Invocations {
		b, err := json.Marshal(entry.Key)
		if err != nil {
			t.Fatal(err)
		}
		if keys[string(b)] {
			t.Fatal("duplicate plan key")
		}
		keys[string(b)] = true
	}
	var c struct{ Invocations []map[string]any }
	decode(t, read(t, filepath.Join(kit, "cases/case-001/case.json")), &c)
	for _, key := range c.Invocations {
		b, err := json.Marshal(key)
		if err != nil {
			t.Fatal(err)
		}
		if !keys[string(b)] {
			t.Fatalf("case invocation absent from plan: %s", b)
		}
	}
	var subject struct{ Runbooks []struct{ Name string } }
	decode(t, read(t, filepath.Join(kit, "cases/case-001/subject.json")), &subject)
	for _, book := range subject.Runbooks {
		if book.Name == "oracles" {
			t.Fatal("case acquired oracle runbook")
		}
	}
}

func subjectHelpCommands(text string) []string {
	var commands []string
	section := false
	for _, line := range strings.Split(text, "\n") {
		if line == "" || !strings.HasPrefix(line, " ") {
			section = strings.HasSuffix(line, " Commands:") || line == "COMMANDS:" || line == "Commands:" || line == "SUBCOMMANDS:"
			continue
		}
		if !section {
			continue
		}
		words := strings.Fields(line)
		if len(words) == 0 {
			continue
		}
		command := strings.TrimSuffix(words[0], ",")
		if command != "help" && command != "completion" {
			commands = append(commands, command)
		}
	}
	return commands
}

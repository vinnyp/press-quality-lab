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
	for tag := range tags {
		t.Run(tag, func(t *testing.T) {
			form := strings.ReplaceAll(strings.TrimPrefix(tag, "form_"), "_", "-")
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
			binary := filepath.Join(t.TempDir(), "widgets")
			subjectGo(t, root, "build", "-tags", tag, "-o", binary, "./testdata/collection-subject")
			digest := subjectSourceDigest(t, root, tag)
			if form == "known-good" {
				testKnownGoodSubject(t, binary)
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
				args := append(slices.Clone(key.Words), key.OfflineArgs...)
				out, _, code := subjectInvoke(t, binary, home, args...)
				first := entry.Confined[0]
				if len(out) != first.Stdout.Bytes || subjectHash(out) != first.Stdout.SHA256 || first.End.Kind != "exit" || code != first.End.Code {
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

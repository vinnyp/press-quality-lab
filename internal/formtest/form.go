// Package formtest builds synthetic forms after checking their sealed source digest.
// It is shared test support; production code must not import it.
package formtest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func hash(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
func read(t testing.TB, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func goRun(t testing.TB, root string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOFLAGS=-mod=readonly", "CGO_ENABLED=0")
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("form build %v: %v\n%s", args, err, b)
	}
	return b
}

// Build returns a private binary and the original recording bytes. root is the
// repository root; form uses the recording's hyphenated name.
func Build(t testing.TB, root, form string) (string, []byte) {
	t.Helper()
	recording := read(t, filepath.Join(root, "docs/specs/conformance-collection-1.0/behaviour", form+".json"))
	var record struct {
		FormSHA string `json:"form_sha256"`
	}
	if err := json.Unmarshal(recording, &record); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "testdata/collection-subject")
	binary := filepath.Join(t.TempDir(), "subject")
	var digest string
	if form == "not-executable" {
		b := read(t, filepath.Join(source, "not-executable.txt"))
		digest = hash(b)
		if err := os.WriteFile(binary, b, 0600); err != nil {
			t.Fatal(err)
		}
	} else {
		tag := "form_" + strings.ReplaceAll(form, "-", "_")
		var info struct{ GoFiles []string }
		if err := json.Unmarshal(goRun(t, root, "list", "-tags", tag, "-json", "./testdata/collection-subject"), &info); err != nil {
			t.Fatal(err)
		}
		if len(info.GoFiles) == 0 {
			t.Fatal("no form source")
		}
		slices.Sort(info.GoFiles)
		entries := make([][3]string, 0, len(info.GoFiles))
		for _, name := range info.GoFiles {
			entries = append(entries, [3]string{name, "file", hash(read(t, filepath.Join(source, name)))})
		}
		b, err := json.Marshal(entries)
		if err != nil {
			t.Fatal(err)
		}
		digest = hash(b)
		if digest != record.FormSHA {
			t.Fatal("form source digest differs from recording")
		}
		goRun(t, root, "build", "-tags", tag, "-o", binary, "./testdata/collection-subject")
	}
	if digest != record.FormSHA {
		t.Fatal("form source digest differs from recording")
	}
	return binary, recording
}

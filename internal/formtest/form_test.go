package formtest

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBuildKnownGood(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	binary, recording := Build(t, root, "known-good")
	info, err := os.Stat(binary)
	if err != nil || info.Size() == 0 || len(recording) == 0 {
		t.Fatalf("form build: %v", err)
	}
}
func TestChangedFormRejected(t *testing.T) {
	if root := os.Getenv("PQL_FORM_TEST_ROOT"); root != "" {
		Build(t, root, "known-good")
		return
	}
	root := t.TempDir()
	for _, name := range []string{"go.mod", "testdata/collection-subject/main.go", "testdata/collection-subject/form_known_good.go", "docs/specs/conformance-collection-1.0/behaviour/known-good.json"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join("../..", name))
		if err != nil {
			t.Fatal(err)
		}
		if name == "testdata/collection-subject/main.go" {
			b = append(b, []byte("\n// altered synthetic source\n")...)
		}
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestChangedFormRejected$")
	cmd.Env = append(os.Environ(), "PQL_FORM_TEST_ROOT="+root)
	b, err := cmd.CombinedOutput()
	if err == nil || !bytes.Contains(b, []byte("form source digest differs from recording")) {
		t.Fatalf("changed form accepted or not evaluated: %v %s", err, b)
	}
}

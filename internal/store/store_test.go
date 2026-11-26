package store

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"github.com/vinnyp/press-quality-lab/internal/throwaway"
)

func TestMandatoryRoot(t *testing.T) {
	// QE17, KTD5: no implicit access to an account's real store.
	if _, err := New(""); !errors.Is(err, ErrRootRequired) {
		t.Fatalf("empty root: %v", err)
	}
	if _, err := New("relative"); !errors.Is(err, ErrRootRequired) {
		t.Fatalf("relative root: %v", err)
	}
}
func TestModesAndRejectedRecord(t *testing.T) {
	// QE17, QE15, KTD8, E-5: rejected input and supplied host facts are retained privately.
	root := filepath.Join(t.TempDir(), "store")
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	r := Record{Host: strictjson.Object{"os_version": "test_os", "os_build": "test_build", "arch": "test_arch"}, SubjectFilePath: "/synthetic/subject.json", SubjectFileDigest: strings.Repeat("0", 64), Request: "grade", Scope: strictjson.Array{"conformance"}, JudgeInput: strictjson.Object{"invalid": true}, JudgeOutput: strictjson.Object{"rejected": true}, JudgeRejected: true, ExitCode: 70}
	path, err := s.Write(r)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(filepath.Dir(path)) != "7fdc36b005c5c71ad63bca74bfa2f6b039141fe3aff991f8b4877b1c7cf5ee41" {
		t.Fatalf("incorrect subject path key: %s", path)
	}
	for p, want := range map[string]os.FileMode{root: 0700, filepath.Dir(path): 0700, path: 0600} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != want {
			t.Errorf("%s mode %o, want %o", p, info.Mode().Perm(), want)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"judge_rejected":true`, `"judge_input":{"invalid":true}`, `"record_version":"1.0"`, `"spec_version":"1.0"`, `"host":{"arch":"test_arch","os_build":"test_build","os_version":"test_os"}`, `"subject_file_path":"/synthetic/subject.json"`, `"subject_file_digest":"0000000000000000000000000000000000000000000000000000000000000000"`, `"binary_digest":null`, `"seed_digest":null`, `"request":"grade"`, `"scope":["conformance"]`, `"exit_code":70`, `"judge_output":{"rejected":true}`, `"next_actions":[]`, `"offline_settings":[]`, `"bookkeeping_excluded":[]`} {
		if !strings.Contains(string(data), field) {
			t.Errorf("missing %s in %s", field, data)
		}
	}
	if data[len(data)-1] != '\n' {
		t.Fatal("missing contract newline")
	}
}
func TestWrongDirectoryMode(t *testing.T) {
	// QE17, KTD5: an existing unsafe store is refused, not silently repaired.
	root := t.TempDir()
	if err := os.Chmod(root, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := New(root); err == nil {
		t.Fatal("accepted unsafe store")
	}
}
func TestWriteAfterDecisionFailure(t *testing.T) {
	// QE17, E-4: the funnel receives a storage error and its scorecard note.
	root := filepath.Join(t.TempDir(), "store")
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(root); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root, nil, 0600); err != nil {
		t.Fatal(err)
	}
	_, err = s.Write(Record{SubjectFilePath: "/synthetic/subject.json", ExitCode: 0})
	if !errors.Is(err, ErrNotStored) || !strings.Contains(err.Error(), "record was not stored") {
		t.Fatalf("write error: %v", err)
	}
}

func TestWorkspaceDelegation(t *testing.T) {
	// QE20, KTD20: a separately owned workspace delegates its whole lifecycle.
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	w, err := Workspace(parent, throwaway.Process{PID: 17, StartTime: "synthetic"})
	if err != nil {
		t.Fatal(err)
	}
	if err := Sweep(parent, func(int) (string, error) { return "", os.ErrNotExist }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(w.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("workspace not swept: %v", err)
	}
}

func TestApplicationSupportLocation(t *testing.T) {
	// QE17, KTD5: the caller's resolver is isolated from the account's real store.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	got, err := ApplicationSupportRoot()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "config", "pql")
	if runtime.GOOS == "darwin" {
		want = filepath.Join(home, "Library", "Application Support", "pql")
	}
	if got != want {
		t.Fatalf("location %q, want %q", got, want)
	}
}
func TestNonNullDigests(t *testing.T) {
	// QE17: envelope keeps the supplied executable and seed digests.
	binary, seed := "binary_digest_value", "seed_digest_value"
	r := Record{BinaryDigest: &binary, SeedDigest: &seed}
	data := string(strictjson.Write(r.Document()))
	for _, field := range []string{`"binary_digest":"binary_digest_value"`, `"seed_digest":"seed_digest_value"`} {
		if !strings.Contains(data, field) {
			t.Fatalf("missing %s in %s", field, data)
		}
	}
}

func TestWorkspacePrivateParent(t *testing.T) {
	// QE20, KTD5, KTD20: publication refuses a non-private parent and cleans up.
	parent := t.TempDir()
	if err := os.Chmod(parent, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := Workspace(parent, throwaway.Process{PID: 17, StartTime: "synthetic"}); err == nil {
		t.Fatal("unsafe workspace published")
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("failed publication left %d entries", len(entries))
	}
}

func TestCanonicalPath(t *testing.T) {
	// QE17: require a clean absolute subject path before creating any record.
	s, err := New(filepath.Join(t.TempDir(), "store"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"subject.json", "/synthetic/../subject.json", "/synthetic/./subject.json"} {
		if _, err := s.Write(Record{SubjectFilePath: path}); !errors.Is(err, ErrNotStored) || !strings.Contains(err.Error(), "canonical subject path required") {
			t.Fatalf("%q: %v", path, err)
		}
	}
	path, err := s.Write(Record{SubjectFilePath: "/synthetic/subject.json"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"subject_file_path":"/synthetic/subject.json"`) {
		t.Fatalf("control: %s", b)
	}
}

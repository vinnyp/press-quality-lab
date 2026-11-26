package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/grade"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/store"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

const pipelineSubject = `{"subject_file_version":"1.0","binary":"bin/subject","facts":{"read_only":false,"has_destructive_ops":false,"store_backed":false,"has_cli":true,"bounding":"declared"},"commands":[{"words":[],"class":"safe","writes":false,"exit_codes":[0],"byte_bound":4096,"budget_seconds":10}],"declared_count":0,"seed":null,"store":null,"offline":[],"bookkeeping":[],"secret":null,"runbooks":[],"markers":[],"prompts":{"version":"none"}}`

type watchedSource struct {
	observe.Source
	observed bool
	events   []string
}

func (s *watchedSource) Observe(observe.Key, observe.Spec) (observe.Run, error) {
	s.observed = true
	panic("released Observe called")
}
func (s *watchedSource) Required(any) error { s.events = append(s.events, "required"); return nil }
func (s *watchedSource) Home(string) error  { s.events = append(s.events, "home"); return nil }

func TestReleasedProcessSentinels(t *testing.T) {
	temp := t.TempDir()
	t.Setenv("TMPDIR", temp)
	if os.TempDir() != temp {
		t.Fatal("test resolved real workspace parent")
	}
	// KTD21, QO1, E-15: neither subject nor computation starts, on any host.
	// Positive controls execute each sentinel and assert its written bytes and stdout.
	dir := t.TempDir()
	write := func(name, value string, mode os.FileMode) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(value), mode); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "computations"), 0700); err != nil {
		t.Fatal(err)
	}
	subjectMarker, computationMarker := filepath.Join(dir, "subject_ran"), filepath.Join(dir, "computation_ran")
	t.Setenv("PQL_SUBJECT_SENTINEL", subjectMarker)
	t.Setenv("PQL_COMPUTATION_SENTINEL", computationMarker)
	write("bin/subject", "#!/bin/sh\nprintf 'subject' > \"$PQL_SUBJECT_SENTINEL\"\nprintf 'subject\\n'\n", 0700)
	write("computations/value", "#!/bin/sh\nprintf 'computation' > \"$PQL_COMPUTATION_SENTINEL\"\nprintf '1\\n'\n", 0700)
	runbooks := `[{"name":"book","steps":[{"name":"step","words":[],"budget_seconds":10,"checks":[{"dimension":"response-correctness","predicate":"row_count","path":null,"expected":{"computation":{"argv":["computations/value"],"budget_seconds":10,"declared_files":[],"captures":[]}},"precondition":null,"exit_codes":null}]}]}]`
	write("subject.json", strings.Replace(pipelineSubject, `"runbooks":[]`, `"runbooks":`+runbooks, 1), 0600)
	old := collectionDependencies
	t.Cleanup(func() { collectionDependencies = old })
	watch := &watchedSource{Source: grade.ReleasedSource{}}
	var saved store.Record
	collectionDependencies = func(commandOptions) grade.Dependencies {
		return grade.Dependencies{
			IDs:    func() (int, int) { return 1000, 1000 },
			Source: func() (observe.Source, func() error, error) { return watch, func() error { return nil }, nil },
			Write:  func(r store.Record) error { saved = r; return nil },
		}
	}
	var out, diagnostic bytes.Buffer
	code := run([]string{"grade", filepath.Join(dir, "subject.json")}, bytes.NewReader(nil), &out, &diagnostic)
	if code != 4 || diagnostic.Len() != 0 || !strings.Contains(out.String(), `"exit_code":4`) || watch.observed || strings.Join(watch.events, ",") != "required" {
		t.Fatalf("code=%d stdout=%q stderr=%q trace=%v observed=%v", code, out.String(), diagnostic.String(), watch.events, watch.observed)
	}
	input, ok := saved.JudgeInput.(strictjson.Object)
	if !ok {
		t.Fatal("missing stored input")
	}
	evidence, ok := input["evidence"].(strictjson.Array)
	if !ok || len(evidence) == 0 {
		t.Fatal("missing evidence")
	}
	for _, v := range evidence {
		e, ok := v.(strictjson.Object)
		if !ok || e["reason"] != "network_unverifiable" {
			t.Fatal(v)
		}
	}
	for _, path := range []string{subjectMarker, computationMarker} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("sentinel started: %s %v", filepath.Base(path), err)
		}
	}
	for _, row := range []struct{ binary, marker, want, stdout string }{
		{"bin/subject", subjectMarker, "subject", "subject\n"},
		{"computations/value", computationMarker, "computation", "1\n"},
	} {
		got, err := exec.Command(filepath.Join(dir, row.binary)).Output()
		if err != nil || string(got) != row.stdout {
			t.Fatalf("positive control: %v stdout=%q", err, got)
		}
		data, err := os.ReadFile(row.marker)
		if err != nil || string(data) != row.want {
			t.Fatalf("positive control: %v bytes=%q", err, data)
		}
	}
}

func TestPipelineErrorEscaping(t *testing.T) {
	temp := t.TempDir()
	t.Setenv("TMPDIR", temp)
	if os.TempDir() != temp {
		t.Fatal("test resolved real workspace parent")
	}
	// KTD16, GKD8: an error carrying subject-file bytes reaches stderr escaped.
	dir := t.TempDir()
	path := filepath.Join(dir, "subject.json")
	text := strings.Replace(pipelineSubject, `"prompts":{"version":"none"}`, `"prompts":{"version":"bad\u001b[31m"}`, 1)
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	old := collectionDependencies
	t.Cleanup(func() { collectionDependencies = old })
	collectionDependencies = func(commandOptions) grade.Dependencies {
		return grade.Dependencies{
			IDs:  func() (int, int) { return 1000, 1000 },
			Open: func(string) (io.ReadCloser, error) { return nil, errors.New("subject\x1b[31m") },
		}
	}
	var out, diagnostic bytes.Buffer
	code := run([]string{"grade", path}, bytes.NewReader(nil), &out, &diagnostic)
	if code != 2 || out.Len() != 0 || diagnostic.String() != "pql: rejected: QS2: subject file unreadable: subject\\u{001b}[31m\n" {
		t.Fatalf("%d %q %q", code, out.String(), diagnostic.String())
	}
	// A recovered panic can carry any subject-derived value; it uses the same funnel.
	collectionDependencies = func(commandOptions) grade.Dependencies {
		return grade.Dependencies{IDs: func() (int, int) { panic("subject\x1b[31m") }}
	}
	diagnostic.Reset()
	code = run([]string{"grade", path}, bytes.NewReader(nil), &out, &diagnostic)
	if code != 70 || diagnostic.String() != `pql: rejected: QE16: engine panic: subject\u{001b}[31m`+"\n" {
		t.Fatalf("%d %q", code, diagnostic.String())
	}
}

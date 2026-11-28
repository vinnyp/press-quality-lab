package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/grade"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/throwaway"
)

func TestGradeScopeRefusals(t *testing.T) {
	// E-7, X1: CLI scope tokens reach the judge unchanged and refuse before serving.
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "binary"), []byte("synthetic"), 0700); err != nil {
		t.Fatal(err)
	}
	subject := `{"subject_file_version":"1.0","binary":"binary","facts":{"read_only":false,"has_destructive_ops":false,"store_backed":false,"has_cli":true,"bounding":"none"},"commands":[],"declared_count":0,"seed":null,"store":null,"offline":[],"bookkeeping":[],"secret":null,"runbooks":[],"markers":[],"prompts":{"version":"none"}}`
	path := filepath.Join(directory, "subject.json")
	if err := os.WriteFile(path, []byte(subject), 0600); err != nil {
		t.Fatal(err)
	}
	original := collectionDependencies
	t.Cleanup(func() { collectionDependencies = original })
	collectionDependencies = func(commandOptions) grade.Dependencies {
		return grade.Dependencies{
			IDs:       func() (int, int) { return 1000, 1000 },
			Workspace: func() (*throwaway.Workspace, error) { return throwaway.BeginAt(filepath.Join(directory, "workspaces")) },
			Source:    func() (observe.Source, func() error, error) { t.Fatal("source reached"); return nil, nil, nil },
		}
	}
	for _, row := range []struct{ name, scope string }{{"DestructiveAlone", "destructive"}, {"Unknown", "conformance,unknown"}, {"Repeated", "conformance,conformance"}} {
		t.Run(row.name, func(t *testing.T) {
			var out, diagnostic bytes.Buffer
			code := run([]string{"grade", "--json", path, "--scope", row.scope}, bytes.NewReader(nil), &out, &diagnostic)
			if code != 2 || out.Len() != 0 || !strings.Contains(diagnostic.String(), "X1: user caused pre-check rejection") {
				t.Fatal(code, out.String(), diagnostic.String())
			}
		})
	}
}

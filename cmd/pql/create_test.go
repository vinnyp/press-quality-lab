package main

import (
	"bytes"
	"encoding/json"
	"github.com/vinnyp/press-quality-lab/internal/subject"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCreate(t *testing.T) {
	// QS5: output expectations are independently enumerated from allowed locations.
	var out, diagnostic bytes.Buffer
	if code := run([]string{"create", "bin/subject"}, bytes.NewReader(nil), &out, &diagnostic); code != 0 {
		t.Fatalf("exit %d: %s", code, diagnostic.String())
	}
	if _, err := subject.Parse(out.Bytes()); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["binary"] != "bin/subject" || got["subject_file_version"] != "1.0" {
		t.Fatal(got)
	}
	for _, key := range []string{"declared_count", "seed", "store", "secret"} {
		assertPlaceholder(t, got[key])
	}
	facts, ok := got["facts"].(map[string]any)
	if !ok {
		t.Fatal("facts absent or not an object")
	}
	if len(facts) != 5 {
		t.Fatal(facts)
	}
	for _, key := range []string{"read_only", "has_destructive_ops", "store_backed", "has_cli", "bounding"} {
		assertPlaceholder(t, facts[key])
	}
	for _, key := range []string{"commands", "runbooks", "markers", "offline", "bookkeeping"} {
		if !reflect.DeepEqual(got[key], []any{}) {
			t.Fatalf("%s: %#v", key, got[key])
		}
	}
	if !reflect.DeepEqual(got["prompts"], map[string]any{"version": ""}) || len(got) != 13 {
		t.Fatal(got)
	}
}
func assertPlaceholder(t *testing.T, v any) {
	t.Helper()
	o, ok := v.(map[string]any)
	if !ok || len(o) != 1 {
		t.Fatalf("placeholder: %#v", v)
	}
	sentence, ok := o["placeholder"].(string)
	if !ok || sentence == "" {
		t.Fatalf("placeholder: %#v", v)
	}
}
func TestCreateNeverRuns(t *testing.T) {
	// QS5: create never executes its binary, proved against a direct-run control.
	dir := t.TempDir()
	marker := filepath.Join(dir, "ran")
	binary := filepath.Join(dir, "subject")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nprintf ran > '"+marker+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	var out, diagnostic bytes.Buffer
	if code := run([]string{"create", binary}, bytes.NewReader(nil), &out, &diagnostic); code != 0 {
		t.Fatalf("exit %d: %s", code, diagnostic.String())
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("sentinel after create: %v", err)
	}
	if output, err := exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("control: %s %v", output, err)
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "ran" {
		t.Fatalf("control: %q %v", data, err)
	}
}

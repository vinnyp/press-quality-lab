package main

import (
	"debug/macho"
	"errors"
	"github.com/vinnyp/press-quality-lab/internal/grade"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDarwinSourceReleaseBoundary(t *testing.T) {
	// QH8, KTD21: a Darwin release still refuses, and never links the test constructor or producer.
	_, err := (grade.ReleasedSource{}).Observe(nil, observe.Spec{})
	var refused *grade.Error
	if !errors.As(err, &refused) || refused.Code != 70 || refused.Rule != "QO1" || refused.Cause != "released source must never observe" {
		t.Fatalf("released refusal %v", err)
	}
	for _, name := range []string{"example/internal/confine.NewSourceForTest", "example/internal/confine.(*Source).Observe", "example/internal/confine.(*Source).Controls"} {
		if !sourceReleaseSymbol(name) {
			t.Fatalf("planted source symbol allowed: %s", name)
		}
	}
	if sourceReleaseSymbol("main.main") {
		t.Fatal("ordinary symbol refused")
	}
	path := filepath.Join(t.TempDir(), "pql")
	cmd := exec.Command("go", "build", "-o", path, ".")
	cmd.Env = append(builderEnvironment(), "GOOS=darwin", "GOARCH=arm64", "CGO_ENABLED=0")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Darwin build: %v %s", err, b)
	}
	f, err := macho.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	}()
	if f.Symtab == nil || len(f.Symtab.Syms) == 0 {
		t.Fatal("missing release symbols")
	}
	foundMain := false
	for _, sym := range f.Symtab.Syms {
		foundMain = foundMain || sym.Name == "main.main"
		if sourceReleaseSymbol(sym.Name) {
			t.Errorf("released source symbol %s", sym.Name)
		}
	}
	if !foundMain {
		t.Fatal("symbol inspection did not find main")
	}
}

func sourceReleaseSymbol(name string) bool {
	return strings.Contains(name, "confine.NewSourceForTest") || strings.Contains(name, "confine.(*Source).")
}

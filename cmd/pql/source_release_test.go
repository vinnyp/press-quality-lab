package main

import (
	"debug/macho"
	"encoding/json"
	"errors"
	"github.com/vinnyp/press-quality-lab/internal/grade"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"go/ast"
	"go/parser"
	"go/token"
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
	for _, arch := range []string{"arm64", "amd64"} {
		for _, tags := range []string{"", "scripted"} {
			t.Run(arch+"_"+strings.ReplaceAll(tags, ",", "_"), func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "pql")
				args := []string{"build", "-o", path}
				if tags != "" {
					args = append(args, "-tags", tags)
				}
				args = append(args, ".")
				cmd := exec.Command("go", args...)
				cmd.Env = append(builderEnvironment(), "GOOS=darwin", "GOARCH="+arch, "CGO_ENABLED=0")
				if b, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("Darwin build: %v %s", err, b)
				}
				inspectSourceRelease(t, path)
			})
		}
	}
}

func inspectSourceRelease(t *testing.T, path string) {
	t.Helper()
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
	return strings.Contains(name, "confine.NewSourceForTest") || strings.Contains(name, "confine.(*Source).") || strings.Contains(name, "collect.Serve") || strings.Contains(name, "collect.CollectWithOptions")
}

func TestCollectionTestAPIBoundary(t *testing.T) {
	// KTD21, KTD20: released and scripted package files expose neither replay nor fixture injection APIs.
	for _, tags := range []string{"", "scripted"} {
		args := []string{"list", "-json"}
		if tags != "" {
			args = append(args, "-tags", tags)
		}
		args = append(args, "github.com/vinnyp/press-quality-lab/internal/collect")
		command := exec.Command("go", args...)
		command.Env = builderEnvironment()
		data, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("go list %v %s", err, data)
		}
		var selected struct {
			Dir     string
			GoFiles []string
		}
		if err := json.Unmarshal(data, &selected); err != nil {
			t.Fatal(err)
		}
		foundOptions := false
		for _, name := range selected.GoFiles {
			file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(selected.Dir, name), nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			ast.Inspect(file, func(node ast.Node) bool {
				switch n := node.(type) {
				case *ast.FuncDecl:
					if n.Name.Name == "Serve" || n.Name.Name == "CollectWithOptions" {
						t.Errorf("test API included: %s", n.Name.Name)
					}
				case *ast.TypeSpec:
					foundOptions = foundOptions || n.Name.Name == "Options"
				case *ast.Field:
					for _, field := range n.Names {
						if field.Name == "HomeFiles" || field.Name == "AfterObserve" {
							t.Error("fixture option included")
						}
					}
				}
				return true
			})
		}
		if !foundOptions {
			t.Fatal("Options was not inspected")
		}
	}
}

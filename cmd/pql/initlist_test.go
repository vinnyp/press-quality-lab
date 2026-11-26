package main

import (
	"debug/elf"
	"encoding/binary"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Cross-check the linux/amd64 linker's ordered init tasks on any host. The Linux
// runtime test separately checks what inittrace actually reports.
func TestLinuxInitialisationTasks(t *testing.T) {
	// KTD12: the released binary contains no scripted-only or test-only symbol.
	path := filepath.Join(t.TempDir(), "pql")
	cmd := exec.Command("go", "build", "-o", path, ".")
	cmd.Env = append(builderEnvironment(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Linux build: %v %s", err, b)
	}
	f, err := elf.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	}()
	symbols, err := f.Symbols()
	if err != nil {
		t.Fatal(err)
	}
	forbidden := exclusiveSymbols(t)
	for _, sym := range symbols {
		if forbiddenReleaseSymbol(sym.Name, forbidden) {
			t.Errorf("released symbol: %s", sym.Name)
		}
	}
	names := map[uint64]string{}
	tasks := map[string]elf.Symbol{}
	for _, sym := range symbols {
		names[sym.Value] = sym.Name
		if sym.Name == "go:runtime.inittasks" || sym.Name == "go:main.inittasks" {
			tasks[sym.Name] = sym
		}
	}
	var packages []string
	seen := map[string]bool{}
	for _, name := range []string{"go:runtime.inittasks", "go:main.inittasks"} {
		sym, ok := tasks[name]
		if !ok {
			t.Fatalf("missing %s", name)
		}
		section := f.Sections[sym.Section]
		b, err := section.Data()
		if err != nil {
			t.Fatal(err)
		}
		b = b[sym.Value-section.Addr : sym.Value-section.Addr+sym.Size]
		for len(b) > 0 {
			address := binary.LittleEndian.Uint64(b)
			b = b[8:]
			task := strings.TrimSuffix(names[address], "..inittask")
			if task == "" {
				t.Fatal("unnamed init task")
			}
			if !seen[task] {
				packages = append(packages, task)
				seen[task] = true
			}
		}
	}
	if strings.Join(linuxInitialisationList(t), "\n") != strings.Join(packages, "\n") {
		t.Fatalf("Linux init task list changed: %q", packages)
	}
}

func linuxInitialisationList(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile("testdata/launcher-linux-init.txt")
	if err != nil {
		t.Fatal(err)
	}
	var packages []string
	for _, line := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		if !strings.HasPrefix(line, "#") {
			packages = append(packages, line)
		}
	}
	return packages
}

// exclusiveSymbols derives the private names from source declarations rather than
// maintaining a list that could miss a newly added test helper or tagged entry.
func exclusiveSymbols(t *testing.T) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	release := map[string]bool{}
	private := map[string]bool{}
	context := build.Default
	context.GOOS = "linux"
	context.GOARCH = "amd64"
	context.BuildTags = nil
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		matches, err := context.MatchFile(".", name)
		if err != nil {
			t.Fatal(err)
		}
		target := private
		if matches && !strings.HasSuffix(name, "_test.go") {
			target = release
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				target[declaration.Name.Name] = true
			case *ast.GenDecl:
				for _, spec := range declaration.Specs {
					switch spec := spec.(type) {
					case *ast.TypeSpec:
						target[spec.Name.Name] = true
					case *ast.ValueSpec:
						for _, name := range spec.Names {
							target[name.Name] = true
						}
					}
				}
			}
		}
	}
	for name := range release {
		delete(private, name)
	}
	return private
}

func forbiddenReleaseSymbol(name string, private map[string]bool) bool {
	if strings.Contains(name, "/internal/scripted.") {
		return true
	}
	if !strings.HasPrefix(name, "main.") && !strings.HasPrefix(name, "type:.eq.main.") {
		return false
	}
	for _, part := range strings.FieldsFunc(name, func(r rune) bool {
		return r == '.' || r == '(' || r == ')' || r == '*' || r == '[' || r == ']'
	}) {
		if private[part] {
			return true
		}
	}
	return false
}

func TestReleaseSymbolControl(t *testing.T) {
	// KTD12: planted symbols must be detected, and ordinary symbols allowed.
	private := exclusiveSymbols(t)
	for _, name := range []string{"validateScriptedEntry", "checkCLI", "TestGradeMalformed"} {
		if !private[name] {
			t.Fatalf("missing private declaration %s", name)
		}
	}
	for _, name := range []string{"main.TestPlanted", "main.scriptedPlanted", "main.(*testPlanted).Read", "example/internal/scripted.Read"} {
		if !forbiddenReleaseSymbol(name, map[string]bool{"TestPlanted": true, "scriptedPlanted": true, "testPlanted": true}) {
			t.Fatalf("missed %s", name)
		}
	}
	if forbiddenReleaseSymbol("main.run", map[string]bool{"TestPlanted": true}) {
		t.Fatal("ordinary symbol rejected")
	}
}

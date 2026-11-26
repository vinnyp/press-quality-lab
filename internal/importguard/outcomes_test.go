package importguard

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// exportImporter uses the compiler's types, including aliases and dot imports;
// source spelling and the caller's import name do not define the boundary.
func exportImporter(t *testing.T) types.Importer {
	t.Helper()
	cmd := exec.Command("go", "list", "-export", "-deps", "-json", "../observe", "../decide")
	data, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	exports := map[string]string{}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	for {
		var p struct{ ImportPath, Export string }
		err := dec.Decode(&p)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		exports[p.ImportPath] = p.Export
	}
	return importer.ForCompiler(token.NewFileSet(), "gc", func(path string) (io.ReadCloser, error) {
		name, ok := exports[path]
		if !ok || name == "" {
			return nil, fmt.Errorf("missing export for %s", path)
		}
		return os.Open(name)
	})
}
func restrictedOutcomeFile(path string, f *ast.File) bool {
	path = filepath.ToSlash(path)
	if path == "internal/scripted/reader.go" {
		return false
	}
	name := filepath.Base(path)
	stem := strings.TrimSuffix(strings.TrimSuffix(name, ".go"), "_test")
	parts := strings.Split(stem, "_")
	for _, part := range parts[1:] {
		switch part {
		case "aix", "android", "darwin", "dragonfly", "freebsd", "hurd", "illumos", "ios", "js", "linux", "netbsd", "openbsd", "plan9", "solaris", "wasip1", "windows", "zos":
			return true
		}
	}
	if strings.Contains(name, "_darwin") || strings.Contains(name, "_other") {
		return true
	}
	for _, group := range f.Comments {
		for _, c := range group.List {
			if strings.HasPrefix(c.Text, "//go:build ") || strings.HasPrefix(c.Text, "// +build ") {
				return true
			}
		}
	}
	return false
}
func decisionType(typ types.Type) bool {
	switch x := typ.(type) {
	case *types.Alias:
		return decisionType(types.Unalias(x))
	case *types.Named:
		obj := x.Obj()
		if obj.Pkg() == nil {
			return false
		}
		p := obj.Pkg().Path()
		return (p == module+"/internal/observe" || p == module+"/internal/decide") && (obj.Name() == "Outcome" || obj.Name() == "Reason" || obj.Name() == "Attribution")
	case *types.Pointer:
		return decisionType(x.Elem())
	case *types.Slice:
		return decisionType(x.Elem())
	case *types.Array:
		return decisionType(x.Elem())
	case *types.Map:
		return decisionType(x.Key()) || decisionType(x.Elem())
	}
	return false
}
func outcomeProblems(path, source string, imports types.Importer, peers ...string) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, source, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	if !restrictedOutcomeFile(path, f) {
		return nil, nil
	}
	files := []*ast.File{f}
	for i, source := range peers {
		peer, err := parser.ParseFile(fset, fmt.Sprintf("peer%d.go", i), source, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		if peer.Name.Name == f.Name.Name {
			files = append(files, peer)
		}
	}
	info := &types.Info{Uses: map[*ast.Ident]types.Object{}, Types: map[ast.Expr]types.TypeAndValue{}}
	// Check one platform file independently: local references supplied by other
	// files may be unresolved, but imported names retain their compiler types.
	config := types.Config{Importer: imports, Error: func(error) {}}
	_, checkErr := config.Check("boundaryfixture", fset, files, info)
	if checkErr != nil && len(info.Uses) == 0 {
		return nil, fmt.Errorf("no type evidence for %s: %w", path, checkErr)
	}
	lines := map[int]bool{}
	for ident, obj := range info.Uses {
		if decisionType(obj.Type()) && fset.Position(ident.Pos()).Filename == path {
			lines[fset.Position(ident.Pos()).Line] = true
		}
	}
	for expr, tv := range info.Types {
		if decisionType(tv.Type) && fset.Position(expr.Pos()).Filename == path {
			lines[fset.Position(expr.Pos()).Line] = true
		}
	}
	var problems []string
	var ordered []int
	for n := range lines {
		ordered = append(ordered, n)
	}
	sort.Ints(ordered)
	for _, n := range ordered {
		problems = append(problems, fmt.Sprintf("%s:%d: decision value in source-specific file", path, n))
	}
	return problems, nil
}
func TestOutcomeBoundary(t *testing.T) {
	// KTD13: no source-specific file may name a decision value or its type.
	imports := exportImporter(t)
	var problems []string
	err := filepath.WalkDir("../..", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel("../..", path)
		if err != nil {
			return err
		}
		var peers []string
		entries, err := os.ReadDir(filepath.Dir(path))
		if err != nil {
			return err
		}
		for _, entry := range entries {
			other := filepath.Join(filepath.Dir(path), entry.Name())
			if other == path || !strings.HasSuffix(other, ".go") {
				continue
			}
			b, err := os.ReadFile(other)
			if err != nil {
				return err
			}
			peers = append(peers, string(b))
		}
		found, err := outcomeProblems(filepath.ToSlash(rel), string(data), imports, peers...)
		if err != nil {
			return err
		}
		problems = append(problems, found...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("%v", problems)
	}
}
func TestOutcomeBoundaryControls(t *testing.T) {
	// KTD13: planted tagged, darwin, unsupported, renamed and dot-import defects.
	imports := exportImporter(t)
	for _, row := range []struct {
		name, path, source string
		bad                bool
	}{
		{"Tagged", "internal/fake/source.go", "//go:build scripted\npackage fake\nimport d \"" + module + "/internal/decide\"\nvar _ = d.NetworkUnverifiable\n", true},
		{"Darwin", "internal/fake/source_darwin.go", "package fake\nimport d \"" + module + "/internal/decide\"\ntype Local = d.Reason\n", true},
		{"Unsupported", "internal/fake/source_other.go", "package fake\nimport . \"" + module + "/internal/decide\"\nvar _ Outcome\n", true},
		{"Linux", "internal/fake/source_linux.go", "package fake\nimport d \"" + module + "/internal/decide\"\nvar _ d.Reason\n", true},
		{"Untagged", "internal/fake/source.go", "package fake\nimport d \"" + module + "/internal/decide\"\nvar _ = d.NotObserved\n", false},
		{"Reader", "internal/scripted/reader.go", "//go:build scripted\npackage scripted\nimport d \"" + module + "/internal/decide\"\nvar _ = d.NotObserved\n", false},
		{"RawFacts", "internal/fake/source_darwin.go", "package fake\nimport o \"" + module + "/internal/observe\"\nvar _ o.Run\n", false},
	} {
		t.Run(row.name, func(t *testing.T) {
			got, err := outcomeProblems(row.path, row.source, imports)
			if err != nil {
				t.Fatal(err)
			}
			var want []string
			if row.bad {
				want = []string{row.path + ":3: decision value in source-specific file"}
				if row.name == "Tagged" {
					want = []string{row.path + ":4: decision value in source-specific file"}
				}
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %v want %v", got, want)
			}
		})
	}
}

func TestOutcomeBoundaryLocalAlias(t *testing.T) {
	// KTD13: same-package aliases cannot hide an imported decision type.
	imports := exportImporter(t)
	got, err := outcomeProblems("source_darwin.go", "package fake\nvar _ Local\n", imports, "package fake\nimport d \""+module+"/internal/decide\"\ntype Local = d.Reason\n")
	want := []string{"source_darwin.go:2: decision value in source-specific file"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v err %v want %v", got, err, want)
	}
}

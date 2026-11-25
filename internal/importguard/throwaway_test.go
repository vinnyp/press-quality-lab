package importguard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// Existing non-workspace responsibilities remain permitted. Tests are fixtures
// and harnesses, not collection code; production files of every build are read.
var filesystemAllowed = map[string]bool{
	"throwaway":    true, // KTD20: sole owner of unconfined workspace operations.
	"confine":      true, // Private executable copies, launcher paths and /dev/null.
	"caseassembly": true, // Corpus harness reads the sealed case inputs.
	"formtest":     true, // Test support reads schema files; never a live collector.
}

func filesystemProblems(path string, source any) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), path, source, 0)
	if err != nil {
		return nil, err
	}
	imports := map[string]string{}
	for _, imp := range f.Imports {
		name, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil, err
		}
		alias := filepath.Base(name)
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		if name == "os" || name == "path/filepath" {
			imports[alias] = name
		}
	}
	var problems []string
	forbidden := func(pkg, name string) bool {
		// KTD16: reads only the user's inputs and never a workspace.
		if filepath.ToSlash(path) == "grade/inputs.go" && pkg == "os" && (name == "Open" || name == "OpenRoot") {
			return false
		}
		return pkg == "os" && (strings.HasPrefix(name, "Remove") || strings.HasPrefix(name, "Open") || name == "ReadFile" || name == "Chmod" || name == "Lstat") || pkg == "path/filepath" && strings.HasPrefix(name, "Walk")
	}
	ast.Inspect(f, func(n ast.Node) bool {
		// Include function values as well as direct calls, so aliasing cannot evade it.
		switch n := n.(type) {
		case *ast.SelectorExpr:
			if id, ok := n.X.(*ast.Ident); ok && forbidden(imports[id.Name], n.Sel.Name) {
				problems = append(problems, fmt.Sprintf("%s: forbidden %s.%s", path, imports[id.Name], n.Sel.Name))
			}
		case *ast.Ident:
			if forbidden(imports["."], n.Name) {
				problems = append(problems, fmt.Sprintf("%s: forbidden %s.%s", path, imports["."], n.Name))
			}
		}
		return true
	})
	return problems, nil
}

func filesystemTreeProblems(base string) ([]string, error) {
	var all []string
	err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		pkg := strings.Split(filepath.ToSlash(rel), "/")[0]
		if filesystemAllowed[pkg] {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		problems, err := filesystemProblems(filepath.ToSlash(rel), data)
		if err != nil {
			return err
		}
		all = append(all, problems...)
		return nil
	})
	return all, err
}
func TestThrowawayFilesystemGuard(t *testing.T) {
	// KTD20: collection code delegates unconfined workspace access.
	problems, err := filesystemTreeProblems("..")
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("%v", problems)
	}
}
func TestThrowawayFilesystemTreeControl(t *testing.T) {
	// KTD20: a planted collection call must fail the same traversal as production.
	base := t.TempDir()
	dir := filepath.Join(base, "collect")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "planted.go")
	if err := os.WriteFile(path, []byte("package collect\nimport \"os\"\nfunc planted(){ os.RemoveAll(\"fixture\") }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := filesystemTreeProblems(base)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"collect/planted.go: forbidden os.RemoveAll"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestThrowawayFilesystemControl(t *testing.T) {
	// KTD20: each forbidden API family has a planted-defect control.
	for _, row := range []struct{ name, pkg, call string }{
		{"Remove", "os", "RemoveAll"}, {"Open", "os", "OpenRoot"}, {"ReadFile", "os", "ReadFile"}, {"Chmod", "os", "Chmod"}, {"Lstat", "os", "Lstat"}, {"Walk", "path/filepath", "WalkDir"},
	} {
		t.Run(row.name, func(t *testing.T) {
			source := fmt.Sprintf("package collect\nimport x %q\nfunc planted() { x.%s(\"fixture\") }\n", row.pkg, row.call)
			got, err := filesystemProblems("collect/planted.go", source)
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"collect/planted.go: forbidden " + row.pkg + "." + row.call}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %v, want %v", got, want)
			}
		})
	}
}

func TestGradeInputException(t *testing.T) {
	// KTD16, KTD20: the exception grants only two APIs in exactly one file.
	for _, row := range []struct {
		path, call string
		allowed    bool
	}{
		{"grade/inputs.go", "Open", true},
		{"grade/inputs.go", "OpenRoot", true},
		{"grade/inputs.go", "RemoveAll", false},
		{"grade/other.go", "Open", false},
		{"grade/other.go", "OpenRoot", false},
	} {
		t.Run(strings.ReplaceAll(row.path, "/", "_")+"_"+row.call, func(t *testing.T) {
			got, err := filesystemProblems(row.path, fmt.Sprintf(`package grade
import "os"
var planted = os.%s
`, row.call))
			if err != nil {
				t.Fatal(err)
			}
			var want []string
			if !row.allowed {
				want = []string{row.path + ": forbidden os." + row.call}
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %v want %v", got, want)
			}
		})
	}
}

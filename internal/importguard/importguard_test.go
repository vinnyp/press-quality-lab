package importguard

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const module = "github.com/vinnyp/press-quality-lab"

// necessity_test is KTD7's test-only renderer tag. Evidence is compiled, never run.
var tagSets = []string{"", "scripted", "necessity_test", "scripted,necessity_test", "macos_evidence", "scripted,macos_evidence", "necessity_test,macos_evidence", "scripted,necessity_test,macos_evidence"}

type listedPackage struct {
	ImportPath string
	Imports    []string
	Deps       []string
	CgoFiles   []string
}

func packagesFrom(r io.Reader) ([]listedPackage, error) {
	var result []listedPackage
	dec := json.NewDecoder(r)
	for {
		var p listedPackage
		if err := dec.Decode(&p); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		result = append(result, p)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("empty dependency list")
	}
	return result, nil
}

func packageName(s string) string {
	return strings.SplitN(s, " [", 2)[0]
}

func dependencyProblems(packages []listedPackage, target string) []string {
	var problems []string
	for _, p := range packages {
		name := packageName(p.ImportPath)
		if !strings.HasPrefix(name, module+"/") {
			continue
		}
		for _, raw := range p.Imports {
			dep := packageName(raw)
			if name == module+"/internal/confine" && (dep == "net" || dep == "os/exec") {
				problems = append(problems, name+": forbidden spawn dependency "+dep)
			}
			if strings.HasPrefix(dep, "github.com/spf13/") && !strings.HasPrefix(name, module+"/cmd/") {
				problems = append(problems, name+": Cobra outside cmd")
			}
			if (dep == "golang.org/x/sys" || strings.HasPrefix(dep, "golang.org/x/sys/")) && target != "darwin" {
				problems = append(problems, name+": x/sys outside darwin")
			}
			if name == module+"/internal/collect" && (dep == module+"/internal/store" || dep == module+"/internal/judge") {
				problems = append(problems, name+": forbidden dependency "+dep)
			}
			if (dep == module+"/internal/scripted" || dep == module+"/internal/confine") && !strings.HasPrefix(name, module+"/cmd/pql") && name != dep && name != dep+".test" && !strings.HasSuffix(name, "_test") {
				problems = append(problems, name+": source selected outside cmd/pql")
			}
		}
		for _, dep := range p.Deps {
			if slices.Contains(p.Imports, dep) {
				continue
			}
			if name == module+"/internal/confine" && (dep == "net" || dep == "os/exec") {
				problems = append(problems, name+": forbidden spawn dependency "+dep)
			}
			if name == module+"/internal/collect" && (dep == module+"/internal/store" || dep == module+"/internal/judge") {
				problems = append(problems, name+": forbidden dependency "+dep)
			}
		}
		// Check transitive dependencies too: a wrapper must not bypass the ban.
		if !strings.HasPrefix(name, module+"/cmd/") {
			for _, dep := range p.Deps {
				if strings.HasPrefix(dep, "github.com/spf13/") && !slices.Contains(p.Imports, dep) {
					problems = append(problems, name+": transitive Cobra outside cmd")
				}
			}
		}
	}
	// x/sys anywhere in the resolved graph is a violation on Linux, including
	// dependencies of third-party packages.
	if target != "darwin" {
		for _, p := range packages {
			if p.ImportPath == "golang.org/x/sys" || strings.HasPrefix(p.ImportPath, "golang.org/x/sys/") {
				problems = append(problems, p.ImportPath+": x/sys outside darwin")
			}
		}
	}
	slices.Sort(problems)
	return slices.Compact(problems)
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func goOutput(t *testing.T, root, target, tags, cgo string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	// os.Environ may already hold any of these keys. Strip them before adding
	// the explicit configuration so nested invocations cannot inherit tags.
	cmd.Env = scrubbedEnv()
	flags := "-mod=readonly"
	if tags != "" {
		flags += " -tags=" + tags
	}
	cmd.Env = append(cmd.Env, "GOOS="+target, "GOARCH=arm64", "CGO_ENABLED="+cgo, "GOFLAGS="+flags, "GOWORK=off", "GOTOOLCHAIN=local", "GOPROXY=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %v (%s tags=%q cgo=%s): %v\n%s", args, target, tags, cgo, err, out)
	}
	return out
}

func TestDependencyMatrix(t *testing.T) {
	root := moduleRoot(t)
	for _, target := range []string{"linux", "darwin"} {
		for _, tags := range tagSets {
			t.Run(target+"/"+tags, func(t *testing.T) {
				out := goOutput(t, root, target, tags, "0", "list", "-deps", "-test", "-json", "./...")
				packages, err := packagesFrom(strings.NewReader(string(out)))
				if err != nil {
					t.Fatal(err)
				}
				if got := dependencyProblems(packages, target); len(got) != 0 {
					t.Fatalf("dependency violations: %v", got)
				}
			})
		}
	}
}

func dependencyDifference(plain, tagged []string) []string {
	var problems []string
	for _, pair := range [][2][]string{{plain, tagged}, {tagged, plain}} {
		for _, dep := range pair[0] {
			if dep != module+"/internal/scripted" && !slices.Contains(pair[1], dep) {
				problems = append(problems, dep)
			}
		}
	}
	slices.Sort(problems)
	return problems
}

func forbiddenDependencies(deps []string) []string {
	var problems []string
	for _, dep := range deps {
		// pflag (required by Cobra) imports net; the launcher inittrace check
		// covers its initialisation.
		if slices.Contains([]string{"crypto/x509", "crypto/tls", "net/http"}, dep) {
			problems = append(problems, dep)
		}
	}
	slices.Sort(problems)
	return problems
}

func TestCommandDependencies(t *testing.T) {
	root := moduleRoot(t)
	for _, target := range []string{"linux", "darwin"} {
		plain := strings.Fields(string(goOutput(t, root, target, "", "0", "list", "-deps", "./cmd/pql")))
		for _, base := range []string{"", "necessity_test", "macos_evidence", "necessity_test,macos_evidence"} {
			untagged := strings.Fields(string(goOutput(t, root, target, base, "0", "list", "-deps", "./cmd/pql")))
			tagged := strings.Fields(string(goOutput(t, root, target, strings.Trim(base+",scripted", ","), "0", "list", "-deps", "./cmd/pql")))
			if got := dependencyDifference(untagged, tagged); len(got) != 0 {
				t.Errorf("%s %s scripted dependency difference: %v", target, base, got)
			}
			if got := scriptedBuildProblem(untagged, tagged); got != "" {
				t.Error(got)
			}
		}
		if target == "darwin" {
			want, err := os.ReadFile("testdata/pql-darwin-deps.txt")
			if err != nil {
				t.Fatal(err)
			}
			if !dependencyListMatches(want, plain) {
				t.Error("darwin dependency list changed; update requires launcher evidence")
			}
			if got := forbiddenDependencies(plain); len(got) != 0 {
				t.Errorf("forbidden darwin dependencies: %v", got)
			}
		}
	}
}

func scriptedBuildProblem(plain, _ []string) string {
	if slices.Contains(plain, module+"/internal/scripted") {
		return "scripted package present in untagged build"
	}
	return ""
}

func scrubbedEnv(extraKeys ...string) []string {
	keys := append([]string{"GOOS", "GOARCH", "CGO_ENABLED", "GOFLAGS", "GOWORK", "GOTOOLCHAIN", "GOPROXY"}, extraKeys...)
	var env []string
	for _, e := range os.Environ() {
		if !slices.Contains(keys, strings.SplitN(e, "=", 2)[0]) {
			env = append(env, e)
		}
	}
	return env
}

func gateStep(t *testing.T, gate, prefix string) string {
	t.Helper()
	var commands []string
	for _, line := range strings.Split(gate, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			commands = append(commands, line)
		}
	}
	if len(commands) != 1 {
		t.Fatalf("want one gate step %q, got %d", prefix, len(commands))
	}
	return commands[0]
}

func dependencyListMatches(want []byte, deps []string) bool {
	return string(want) == strings.Join(deps, "\n")+"\n"
}

func cgoProblems(packages []listedPackage) []string {
	var problems []string
	for _, p := range packages {
		for _, file := range p.CgoFiles {
			problems = append(problems, p.ImportPath+": "+file)
		}
	}
	slices.Sort(problems)
	return problems
}

func TestNoDarwinCgo(t *testing.T) {
	for _, tags := range tagSets {
		// CGO_ENABLED=1 is essential: cross builds otherwise silently omit CgoFiles.
		out := goOutput(t, moduleRoot(t), "darwin", tags, "1", "list", "-test", "-json", "./...")
		packages, err := packagesFrom(strings.NewReader(string(out)))
		if err != nil {
			t.Fatal(err)
		}
		if got := cgoProblems(packages); len(got) != 0 {
			t.Errorf("darwin cgo files: %v", got)
		}
	}
}

func TestCgoListMutation(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":  "module example.invalid/cgoprobe\n\ngo 1.27.4\n",
		"base.go": "package probe\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	check := func(want []string) {
		t.Helper()
		out := goOutput(t, root, "darwin", "", "1", "list", "-test", "-json", "./...")
		packages, err := packagesFrom(strings.NewReader(string(out)))
		if err != nil {
			t.Fatal(err)
		}
		if got := cgoProblems(packages); !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v; want %v", got, want)
		}
	}
	check(nil)
	if err := os.WriteFile(filepath.Join(root, "probe_darwin.go"), []byte("package probe\nimport \"C\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	check([]string{"example.invalid/cgoprobe: probe_darwin.go"})
}

// Inspect source rather than the host's selected build files: this covers
// unsupported platforms, additional tags, and files hidden by cgo selection.
func sourceProblems(root string) ([]string, error) {
	var problems []string
	var selectors []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			// Like go list ./..., skip testdata: synthetic subjects are
			// separate programs, not packages of this module.
			if entry.Name() == ".git" || entry.Name() == "vendor" || entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			if path != root {
				if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
					return filepath.SkipDir
				} else if !os.IsNotExist(err) {
					return err
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == "init" {
				problems = append(problems, filepath.ToSlash(rel)+": func init")
			}
		}
		selects := false
		for _, imp := range file.Imports {
			dep, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			if dep == "C" {
				problems = append(problems, filepath.ToSlash(rel)+": import C")
			}
			if dep == module+"/internal/scripted" || dep == module+"/internal/confine" {
				selects = true
			}
		}
		if selects && strings.HasPrefix(filepath.ToSlash(rel), "cmd/pql/") && !strings.HasSuffix(rel, "_test.go") {
			selectors = append(selectors, rel)
		}
		return nil
	})
	if len(selectors) > 2 {
		problems = append(problems, "cmd/pql: source selection in more than two files")
	}
	slices.Sort(problems)
	return problems, err
}

func TestModuleSource(t *testing.T) {
	problems, err := sourceProblems(moduleRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("source violations: %v", problems)
	}
}

func TestDependencyClassifier(t *testing.T) {
	for _, tc := range []struct {
		name, json, target string
		want               []string
	}{
		{"transitive spawn", `{"ImportPath":"` + module + `/internal/confine","Deps":["os/exec","net"]}`, "darwin", []string{module + "/internal/confine: forbidden spawn dependency net", module + "/internal/confine: forbidden spawn dependency os/exec"}},
		{"transitive collect", `{"ImportPath":"` + module + `/internal/collect","Deps":["` + module + `/internal/judge","` + module + `/internal/store"]}`, "darwin", []string{module + "/internal/collect: forbidden dependency " + module + "/internal/judge", module + "/internal/collect: forbidden dependency " + module + "/internal/store"}},
		{"pflag", `{"ImportPath":"` + module + `/internal/a","Imports":["github.com/spf13/pflag"]}`, "darwin", []string{module + "/internal/a: Cobra outside cmd"}},
		{"transitive pflag", `{"ImportPath":"` + module + `/internal/a","Deps":["github.com/spf13/pflag"]}`, "darwin", []string{module + "/internal/a: transitive Cobra outside cmd"}},
		{"clean command", `{"ImportPath":"` + module + `/cmd/pql","Imports":["github.com/spf13/cobra"]}`, "linux", nil},
		{"spawn exec", `{"ImportPath":"` + module + `/internal/confine","Imports":["os/exec"]}`, "darwin", []string{module + "/internal/confine: forbidden spawn dependency os/exec"}},
		{"spawn net", `{"ImportPath":"` + module + `/internal/confine","Imports":["net"]}`, "darwin", []string{module + "/internal/confine: forbidden spawn dependency net"}},
		{"darwin cobra", `{"ImportPath":"` + module + `/internal/confine","Imports":["github.com/spf13/cobra"]}`, "darwin", []string{module + "/internal/confine: Cobra outside cmd"}},
		{"test variant cobra", `{"ImportPath":"` + module + `/internal/a [` + module + `/internal/a.test]","Imports":["github.com/spf13/cobra"]}`, "darwin", []string{module + "/internal/a: Cobra outside cmd"}},
		{"transitive cobra", `{"ImportPath":"` + module + `/internal/a","Deps":["github.com/spf13/cobra"]}`, "linux", []string{module + "/internal/a: transitive Cobra outside cmd"}},
		{"linux syscall", `{"ImportPath":"golang.org/x/sys/unix"}`, "linux", []string{"golang.org/x/sys/unix: x/sys outside darwin"}},
		{"darwin syscall", `{"ImportPath":"golang.org/x/sys/unix"}`, "darwin", nil},
		{"collect store", `{"ImportPath":"` + module + `/internal/collect","Imports":["` + module + `/internal/store"]}`, "linux", []string{module + "/internal/collect: forbidden dependency " + module + "/internal/store"}},
		{"collect judge", `{"ImportPath":"` + module + `/internal/collect","Imports":["` + module + `/internal/judge"]}`, "linux", []string{module + "/internal/collect: forbidden dependency " + module + "/internal/judge"}},
		{"collect inputs", `{"ImportPath":"` + module + `/internal/collect","Imports":["` + module + `/internal/observe","` + module + `/internal/plan","` + module + `/internal/decide"]}`, "linux", nil},
		{"confine test driver", `{"ImportPath":"` + module + `/internal/confine.test","Imports":["` + module + `/internal/confine"]}`, "darwin", nil},
		{"source selection", `{"ImportPath":"` + module + `/internal/grade","Imports":["` + module + `/internal/confine"]}`, "darwin", []string{module + "/internal/grade: source selected outside cmd/pql"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			packages, err := packagesFrom(strings.NewReader(tc.json))
			if err != nil {
				t.Fatal(err)
			}
			if got := dependencyProblems(packages, tc.target); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v; want %v", got, tc.want)
			}
		})
	}
	for _, input := range []string{"", "not json", "{} trailing"} {
		if _, err := packagesFrom(strings.NewReader(input)); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
}

func TestSourceMutations(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		want         []string
	}{
		{"clean", "package fixture\n// func init() is only a comment\n", nil},
		{"init", "package fixture\nfunc init() {}\n", []string{"probe.go: func init"}},
		{"cgo", "//go:build darwin && unexpected_tag\n\npackage fixture\nimport `C`\n", []string{"probe.go: import C"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "probe.go"), []byte(tc.source), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := sourceProblems(root)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v; want %v", got, tc.want)
			}
		})
	}

	for _, tc := range []struct {
		name  string
		files int
		want  []string
	}{
		{"TwoSelectors", 2, nil},
		{"ThreeSelectors", 3, []string{"cmd/pql: source selection in more than two files"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "cmd", "pql")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < tc.files; i++ {
				path := filepath.Join(dir, fmt.Sprintf("selector%d.go", i))
				if err := os.WriteFile(path, []byte("package main\nimport _ \""+module+"/internal/confine\"\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := sourceProblems(root)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v; want %v", got, tc.want)
			}
		})
	}
}

func TestSourceScope(t *testing.T) {
	root := t.TempDir()
	for name, source := range map[string]string{
		"testdata/subject/probe.go": "package main\nfunc init() {}\n",
		"probe_test.go":             "//go:build unexpected_tag\n\npackage probe\nfunc init() {}\n",
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := sourceProblems(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"probe_test.go: func init"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v; want %v", got, want)
	}
}

func TestDependencyListMutations(t *testing.T) {
	for _, tc := range []struct {
		name          string
		plain, tagged []string
		want          string
	}{
		{"ScriptedBoth", []string{module + "/internal/scripted"}, []string{module + "/internal/scripted"}, "scripted package present in untagged build"},
		{"ScriptedTaggedOnly", nil, []string{module + "/internal/scripted"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := scriptedBuildProblem(tc.plain, tc.tagged); got != tc.want {
				t.Fatalf("scripted build problem: got %q want %q", got, tc.want)
			}
		})
	}

	if !dependencyListMatches([]byte("fmt\nos\n"), []string{"fmt", "os"}) {
		t.Fatal("clean dependency list rejected")
	}
	for _, changed := range []string{"fmt\n", "fmt\nos\nnet\n", "os\nfmt\n"} {
		if dependencyListMatches([]byte(changed), []string{"fmt", "os"}) {
			t.Fatalf("accepted changed dependency list %q", changed)
		}
	}
	if got := dependencyDifference([]string{"fmt"}, []string{"fmt", module + "/internal/scripted"}); len(got) != 0 {
		t.Fatal(got)
	}
	if got := dependencyDifference([]string{"fmt"}, []string{"fmt", "os"}); !reflect.DeepEqual(got, []string{"os"}) {
		t.Fatal(got)
	}
	for _, dep := range []string{"crypto/x509", "crypto/tls", "net/http"} {
		if got := forbiddenDependencies([]string{"fmt", dep}); !reflect.DeepEqual(got, []string{dep}) {
			t.Fatal(got)
		}
	}
	if got := forbiddenDependencies([]string{"fmt", "net"}); len(got) != 0 {
		t.Fatal(got)
	}
}

// These controls use throwaway modules, never the real tree. They exercise the
// same vet and compile-only commands as the gate without running any binary.
func TestToolMutations(t *testing.T) {
	for _, tc := range []struct {
		name, target, tag, filename string
		args                        []string
		clean, defect, diagnostic   string
	}{
		{"scripted vet", "linux", "scripted", "probe.go", []string{"vet", "./..."}, "//go:build scripted\n\npackage probe\nimport \"fmt\"\nfunc Probe() string { return fmt.Sprintf(\"%d\", 1) }\n", "//go:build scripted\n\npackage probe\nimport \"fmt\"\nfunc Probe() string { return fmt.Sprintf(\"%d\", \"bad\") }\n", "format %d has arg"},
		{"darwin vet", "darwin", "", "probe_darwin.go", []string{"vet", "./..."}, "package probe\nimport \"fmt\"\nfunc Probe() string { return fmt.Sprintf(\"%d\", 1) }\n", "package probe\nimport \"fmt\"\nfunc Probe() string { return fmt.Sprintf(\"%d\", \"bad\") }\n", "format %d has arg"},
		{"evidence compile", "darwin", "macos_evidence", "probe_test.go", []string{"test", "-c", "-vet=off", "-o", "probe.test", "."}, "//go:build macos_evidence\n\npackage probe\nimport \"testing\"\nfunc TestEvidence(t *testing.T) { t.Log(\"compiled only\") }\n", "//go:build macos_evidence\n\npackage probe\nimport \"testing\"\nfunc TestEvidence(t *testing.T) { missingEvidenceSymbol() }\n", "undefined: missingEvidenceSymbol"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for name, content := range map[string]string{"go.mod": "module example.invalid/gateprobe\n\ngo 1.27.3\n", "base.go": "package probe\n", tc.filename: tc.clean} {
				if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			run := func() ([]byte, error) {
				cmd := exec.Command("go", tc.args...)
				cmd.Dir = root
				cmd.Env = scrubbedEnv()
				cmd.Env = append(cmd.Env, "GOOS="+tc.target, "GOARCH=arm64", "CGO_ENABLED=0", "GOFLAGS=-mod=readonly -tags="+tc.tag, "GOWORK=off", "GOTOOLCHAIN=local", "GOPROXY=off")
				return cmd.CombinedOutput()
			}
			if out, err := run(); err != nil {
				t.Fatalf("clean control: %v\n%s", err, out)
			}
			if err := os.WriteFile(filepath.Join(root, tc.filename), []byte(tc.defect), 0600); err != nil {
				t.Fatal(err)
			}
			out, err := run()
			if err == nil || !strings.Contains(string(out), tc.diagnostic) {
				t.Fatalf("defect was not caught by expected check: %v\n%s", err, out)
			}
		})
	}
}

// Read the actual gate steps so changes to their flags exercise these controls.
// Only the step wrapper is replaced; bash expands the gate's command verbatim.
func TestTaggedLintControls(t *testing.T) {
	gate, err := os.ReadFile(filepath.Join(moduleRoot(t), "gate.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, variant := range []struct{ target, tags string }{
		{"linux", "scripted"},
		{"darwin", ""},
		{"darwin", "scripted"},
		{"darwin", "necessity_test"},
		{"darwin", "scripted,necessity_test"},
	} {
		for _, lint := range []struct{ tool, clean, defect, diagnostic string }{
			{"vet", `return fmt.Errorf("%d", 1)`, `return fmt.Errorf("%d", "bad")`, "format %d has arg"},
			{"errcheck", "if err := F(); err != nil { return err }; return nil", "F(); return nil", "F()"},
			{"staticcheck", "return nil", "if v == v { return nil }; return nil", "SA4000"},
			{"exhaustive", "switch v { case First, Second: return nil }; return nil", "switch v { case First: return nil }; return nil", "missing cases in expression switch: Second"},
		} {
			t.Run(variant.target+"/"+variant.tags+"/"+lint.tool, func(t *testing.T) {
				prefix := "step \"" + lint.tool + "-$tags\" "
				if variant.target == "darwin" {
					prefix = "step \"darwin-" + lint.tool + "-$label\" "
				}
				command := gateStep(t, string(gate), prefix)
				root := t.TempDir()
				for name, content := range map[string]string{
					"go.mod":  "module example.invalid/lintprobe\n\ngo 1.27.3\n",
					"base.go": "package probe\n",
				} {
					if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
						t.Fatal(err)
					}
				}
				constraint := strings.ReplaceAll(variant.tags, ",", " && ")
				if variant.target == "darwin" {
					constraint = strings.TrimSuffix("darwin && "+constraint, " && ")
				}
				header := "//go:build " + constraint + "\n\npackage probe\nimport \"fmt\"\nvar _ = fmt.Sprint\ntype Value int\nconst (First Value = iota; Second)\nfunc F() error { return nil }\nfunc Probe(v Value) error { "
				for _, control := range []struct{ name, body string }{{"clean", lint.clean}, {"defect", lint.defect}} {
					if err := os.WriteFile(filepath.Join(root, "probe.go"), []byte(header+control.body+" }\n"), 0600); err != nil {
						t.Fatal(err)
					}
					cmd := exec.Command("bash", "-eu", "-c", `
step() { shift; "$@"; }
flags="-mod=readonly${tags:+ -tags=$tags}"
label="${tags:-plain}"
export PATH="${GATE_TOOLS_BIN:-$HOME/.gate-tools/go/bin}:$PATH"
`+command)
					cmd.Dir = root
					cmd.Env = scrubbedEnv("tags")
					cmd.Env = append(cmd.Env, "GOOS="+variant.target, "GOARCH=arm64", "CGO_ENABLED=0", "GOFLAGS=-mod=readonly", "GOWORK=off", "GOTOOLCHAIN=local", "GOPROXY=off", "tags="+variant.tags)
					out, err := cmd.CombinedOutput()
					if control.name == "clean" {
						if err != nil || len(out) != 0 {
							t.Fatalf("clean control: %v\n%s", err, out)
						}
					} else if err == nil || !strings.Contains(string(out), lint.diagnostic) {
						t.Fatalf("defect was not caught by %s: %v\n%s", lint.tool, err, out)
					}
				}
			})
		}
	}
}

func TestGateTagSets(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(moduleRoot(t), "gate.sh"))
	if err != nil {
		t.Fatal(err)
	}
	var lists []string
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "for tags in ") {
			lists = append(lists, line)
		}
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.Contains(line, "GOOS=darwin") && !strings.Contains(line, "GOARCH=arm64") {
			t.Fatalf("darwin architecture not pinned: %s", line)
		}
	}
	want := []string{"for tags in scripted; do", "for tags in '' scripted necessity_test scripted,necessity_test; do"}
	if !reflect.DeepEqual(lists, want) {
		t.Fatalf("gate tag sets: got %q want %q", lists, want)
	}
}

// Execute the actual script's test and compile-only steps in a throwaway module.
func TestGateExecutionControls(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(moduleRoot(t), "gate.sh"))
	if err != nil {
		t.Fatal(err)
	}
	gate := string(b)
	for _, kind := range []string{"plain", "test", "evidence"} {
		t.Run(kind, func(t *testing.T) {
			prefix := `step "test-$tags" `
			if kind == "plain" {
				prefix = `step test `
			}
			if kind == "evidence" {
				prefix = `step "darwin-evidence-compile-$label" `
			}
			command := gateStep(t, gate, prefix)
			prelude := `step() { shift; "$@"; }
export GOPROXY=off GOTOOLCHAIN=local CGO_ENABLED=0 GOFLAGS=-mod=readonly
tags=scripted
label=scripted
`
			tag := "scripted"
			if kind == "plain" {
				tag = ""
			}
			if kind == "evidence" {
				start := strings.Index(gate, "compile_evidence() {")
				end := strings.Index(gate[start:], "\n}\n")
				if start < 0 || end < 0 {
					t.Fatal("missing compile_evidence function")
				}
				prelude += gate[start:start+end+3] + "\n"
				tag = "scripted && macos_evidence"
			}
			root := t.TempDir()
			for name, content := range map[string]string{"go.mod": "module example.invalid/executionprobe\n\ngo 1.27.4\n", "base.go": "package probe\n"} {
				if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			for _, defect := range []bool{false, true} {
				body := `t.Log("clean")`
				diagnostic := ""
				if defect {
					if kind != "evidence" {
						body = `t.Fatal("planted test failure")`
						diagnostic = "planted test failure"
					} else {
						body = `missingEvidenceSymbol()`
						diagnostic = "undefined: missingEvidenceSymbol"
					}
				}
				constraint := ""
				if tag != "" {
					constraint = "//go:build " + tag + "\n\n"
				}
				source := constraint + "package probe\nimport \"testing\"\nfunc TestProbe(t *testing.T){" + body + "}\n"
				if err := os.WriteFile(filepath.Join(root, "probe_test.go"), []byte(source), 0600); err != nil {
					t.Fatal(err)
				}
				cmd := exec.Command("bash", "-eu", "-c", prelude+command)
				cmd.Dir = root
				cmd.Env = scrubbedEnv()
				target, arch := "darwin", "arm64"
				if kind != "evidence" {
					target, arch = runtime.GOOS, runtime.GOARCH
				}
				cmd.Env = append(cmd.Env, "GOOS="+target, "GOARCH="+arch)
				out, err := cmd.CombinedOutput()
				if !defect && err != nil {
					t.Fatalf("clean gate execution: %v %s", err, out)
				}
				if defect && (err == nil || !strings.Contains(string(out), diagnostic)) {
					t.Fatalf("gate execution missed defect: %v %s", err, out)
				}
			}
		})
	}
}

package docrefs

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/build"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type ruleProof struct {
	Kind     string `json:"kind"`
	Section  string `json:"section"`
	Route    string `json:"route"`
	File     string `json:"file"`
	Function string `json:"function"`
}

// Definitions are the sealed Markdown's bullet followed by a bold id and dot,
// never an inline citation. collectionDefinition is shared with root-doc checks.
func ledgerRules(root string) (map[string]bool, error) {
	files, err := filepath.Glob(filepath.Join(root, "docs/specs/conformance-collection-1.0/*.md"))
	if err != nil {
		return nil, err
	}
	rules := map[string]bool{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		for _, match := range collectionDefinition.FindAllStringSubmatch(string(data), -1) {
			if rules[match[1]] {
				return nil, fmt.Errorf("duplicate rule %s", match[1])
			}
			rules[match[1]] = true
		}
	}
	return rules, nil
}

// Require an actual build constraint that cannot admit a build without the
// evidence tag. Evaluate every assignment of the other tags, not a substring
// that could be a comment or one side of an OR.
func evidenceOnly(source string) bool {
	// The parser locates the package token even with leading block comments or
	// unusual whitespace; constraints in declarations or raw strings are not headers.
	fset := token.NewFileSet()
	if file, err := parser.ParseFile(fset, "proof.go", source, parser.PackageClauseOnly); err == nil {
		source = source[:fset.Position(file.Package).Offset]
	}
	for _, line := range strings.Split(source, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "package ") {
			break
		}
		if !strings.HasPrefix(line, "//go:build ") {
			continue
		}
		expr, err := constraint.Parse(line)
		if err != nil {
			return false
		}
		tags := map[string]bool{}
		for _, tag := range regexp.MustCompile(`[A-Za-z0-9_]+`).FindAllString(expr.String(), -1) {
			tags[tag] = true
		}
		if !tags["macos_evidence"] || len(tags) > 16 {
			return false
		}
		var others []string
		for tag := range tags {
			if tag != "macos_evidence" {
				others = append(others, tag)
			}
		}
		sort.Strings(others)
		satisfiable := false
		for bits := 0; bits < 1<<len(others); bits++ {
			enabled := map[string]bool{}
			for i, tag := range others {
				enabled[tag] = bits&(1<<i) != 0
			}
			if expr.Eval(func(tag string) bool { return enabled[tag] }) {
				return false
			}
			enabled["macos_evidence"] = true
			satisfiable = satisfiable || expr.Eval(func(tag string) bool { return enabled[tag] })
		}
		return satisfiable
	}
	return false
}

// GR15: derive the not-cited list from the sealed text. These six additional
// ids come from the process/profile/system-version limits in Not expressible.
var gr15Additional = []string{"QH1", "QH2", "QH11", "QH13", "QH18", "QH23"}

func gr15Rules(cases string) map[string]bool {
	out := map[string]bool{}
	parts := strings.SplitN(cases, "and the cases do not cite them:", 2)
	if len(parts) == 2 {
		for _, m := range regexp.MustCompile(`(?m)^- (Q[A-Z]+[0-9]+):`).FindAllStringSubmatch(parts[1], -1) {
			out[m[1]] = true
		}
	}
	for _, id := range gr15Additional {
		out[id] = true
	}
	return out
}

func listEntries(data string) map[string]bool {
	out := map[string]bool{}
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			out[line] = true
		}
	}
	return out
}

func proofExists(root, cases, id string, p ruleProof) bool {
	if p.Kind == "limit" {
		if p.Section != "Not expressible as a case" {
			return false
		}
		section := strings.SplitN(cases, "## "+p.Section+"\n", 2)
		if len(section) != 2 || !strings.HasPrefix(p.Route, "- "+id+": ") {
			return false
		}
		body := strings.SplitN(section[1], "\n## ", 2)[0]
		for _, line := range strings.Split(body, "\n") {
			if line == p.Route {
				return true
			}
		}
		return false
	}
	if p.Kind != "test" && p.Kind != "macos_evidence" {
		return false
	}
	if !filepath.IsLocal(p.File) || !strings.HasSuffix(p.File, "_test.go") {
		return false
	}
	data, err := os.ReadFile(filepath.Join(root, p.File))
	if err != nil {
		return false
	}
	if p.Kind == "test" {
		builds := false
		for _, tags := range [][]string{nil, {"scripted"}} {
			ctx := build.Default
			ctx.GOOS = "linux"
			ctx.GOARCH = "amd64"
			ctx.CgoEnabled = false
			ctx.BuildTags = tags
			ok, err := ctx.MatchFile(filepath.Join(root, filepath.Dir(p.File)), filepath.Base(p.File))
			builds = builds || (err == nil && ok)
		}
		if !builds {
			return false
		}
	}
	if p.Kind == "macos_evidence" {
		if !evidenceOnly(string(data)) {
			return false
		}
		gate, err := os.ReadFile(filepath.Join("../..", "gate.sh"))
		if err != nil {
			return false
		}
		loops := regexp.MustCompile(`(?ms)^for tags in ([^\n]+); do\n(.*?)^done$`).FindAllStringSubmatch(string(gate), -1)
		builds := false
		for _, loop := range loops {
			if !strings.Contains(loop[2], "compile_evidence") {
				continue
			}
			for _, tags := range strings.Fields(loop[1]) {
				tags = strings.Trim(tags, "'")
				ctx := build.Default
				ctx.GOOS, ctx.GOARCH, ctx.CgoEnabled = "darwin", "arm64", false
				ctx.BuildTags = nil
				if tags != "" {
					ctx.BuildTags = strings.Split(tags, ",")
				}
				without, err := ctx.MatchFile(filepath.Join(root, filepath.Dir(p.File)), filepath.Base(p.File))
				if err != nil || without {
					return false
				}
				ctx.BuildTags = append(ctx.BuildTags, "macos_evidence")
				with, err := ctx.MatchFile(filepath.Join(root, filepath.Dir(p.File)), filepath.Base(p.File))
				builds = builds || (err == nil && with)
			}
		}
		if !builds {
			return false
		}
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, p.File, data, parser.ParseComments)
	if err != nil {
		return false
	}
	for _, decl := range file.Decls {
		f, ok := decl.(*ast.FuncDecl)
		if !ok || f.Recv != nil || f.Name.Name != p.Function || !strings.HasPrefix(p.Function, "Test") || f.Type.Results != nil || f.Type.Params == nil || len(f.Type.Params.List) != 1 {
			continue
		}
		star, ok := f.Type.Params.List[0].Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		sel, ok := star.X.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		pkg, ok := sel.X.(*ast.Ident)
		if ok && pkg.Name == "testing" && sel.Sel.Name == "T" {
			cites := func(text string) bool {
				for _, cited := range clauseToken.FindAllString(text, -1) {
					if cited == id {
						return true
					}
				}
				return false
			}
			if cites(f.Name.Name) {
				return true
			}
			for _, group := range file.Comments {
				if group == f.Doc || (group.Pos() >= f.Pos() && group.End() <= f.End()) {
					if cites(group.Text()) {
						return true
					}
				}
			}
			citedSubtest := false
			ast.Inspect(f.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || len(call.Args) == 0 {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "Run" {
					return true
				}
				name, ok := call.Args[0].(*ast.BasicLit)
				if ok && name.Kind == token.STRING && cites(name.Value) {
					citedSubtest = true
				}
				return true
			})
			return citedSubtest
		}
	}
	return false
}

func ledgerProblems(root, cases string, rules map[string]bool, mapping map[string][]ruleProof, pending map[string]bool) []string {
	var problems []string
	gr15 := gr15Rules(cases)
	for id := range rules {
		proofs := mapping[id]
		if len(proofs) == 0 && !pending[id] {
			problems = append(problems, id+": neither mapped nor listed")
		}
		if len(proofs) > 0 && pending[id] {
			problems = append(problems, id+": mapped and listed")
		}
		if gr15[id] && !pending[id] {
			evidence := false
			for _, p := range proofs {
				evidence = evidence || (p.Kind == "macos_evidence" && proofExists(root, cases, id, p))
			}
			if !evidence {
				problems = append(problems, id+": macos_evidence required")
			}
		}
		for _, p := range proofs {
			namesRule := false
			for _, token := range clauseToken.FindAllString(p.Route, -1) {
				if token == id {
					namesRule = true
				}
			}
			if !proofExists(root, cases, id, p) || (p.Kind == "limit" && !namesRule) {
				problems = append(problems, id+": invalid proof")
			}
		}
	}
	for id := range mapping {
		if !rules[id] {
			problems = append(problems, id+": unknown mapping")
		}
	}
	for id := range pending {
		if !rules[id] {
			problems = append(problems, id+": unknown listed rule")
		}
	}
	sort.Strings(problems)
	return problems
}

// QS1: ledger control fixture cites a binding rule without claiming its proof.
func TestRuleLedger(t *testing.T) {
	root := "../.."
	rules, err := ledgerRules(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 103 {
		t.Fatalf("extracted %d binding rules, want 103", len(rules))
	}
	data, err := os.ReadFile("rule-ledger.json")
	if err != nil {
		t.Fatal(err)
	}
	var ledger struct {
		Rules map[string][]ruleProof `json:"rules"`
	}
	if err := json.Unmarshal(data, &ledger); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile("unmapped-rules.txt")
	if err != nil {
		t.Fatal(err)
	}
	pending := listEntries(string(data))
	cases, err := os.ReadFile(filepath.Join(root, "docs/specs/conformance-collection-1.0/CASES.md"))
	if err != nil {
		t.Fatal(err)
	}
	gr15 := gr15Rules(string(cases))
	checked, err := os.ReadFile("gr15-rules.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(gr15) != 13 || !reflect.DeepEqual(gr15, listEntries(string(checked))) {
		t.Fatalf("GR15 set differs: %v", gr15)
	}
	if len(pending)+len(ledger.Rules) != 103 {
		t.Fatal("ledger must account for all 103 rules")
	}
	if ps := ledgerProblems(root, string(cases), rules, ledger.Rules, pending); len(ps) != 0 {
		t.Fatalf("%v", ps)
	}
}

func TestLedgerControls(t *testing.T) {
	root := t.TempDir()
	for name, source := range map[string]string{
		"valid_test.go":    "package fixture\nimport \"testing\"\n// QS1 QH13\nfunc TestProof(t *testing.T) {}\n",
		"uncited_test.go":  "package fixture\nimport \"testing\"\nfunc TestProof(t *testing.T) { _ = \"QS1\" }\n",
		"hidden_test.go":   "//go:build never_ci\n\npackage fixture\nimport \"testing\"\n// QS1\nfunc TestProof(t *testing.T) {}\n",
		"named_test.go":    "package fixture\nimport \"testing\"\nfunc TestQS1(t *testing.T) {}\n",
		"subtest_test.go":  "package fixture\nimport \"testing\"\nfunc TestProof(t *testing.T) { t.Run(\"QS1\",func(t *testing.T){}) }\n",
		"scripted_test.go": "//go:build scripted\n\npackage fixture\nimport \"testing\"\n// QS1\nfunc TestProof(t *testing.T) {}\n",

		"never_evidence_test.go": "//go:build macos_evidence && never_ci\n\npackage fixture\nimport \"testing\"\n// QS1\nfunc TestProof(t *testing.T) {}\n",
		"linux_evidence_test.go": "//go:build linux && macos_evidence\n\npackage fixture\nimport \"testing\"\n// QS1\nfunc TestProof(t *testing.T) {}\n",
		"evidence_linux_test.go": "//go:build macos_evidence\n\npackage fixture\nimport \"testing\"\n// QS1\nfunc TestProof(t *testing.T) {}\n",
		"raw_test.go":            "package fixture\nimport \"testing\"\nvar text = `\n//go:build macos_evidence\n`\n// QS1\nfunc TestProof(t *testing.T) {}\n",
		"evidence_test.go":       "//go:build macos_evidence\n\npackage fixture\nimport \"testing\"\n// QH13\nfunc TestProof(t *testing.T) {}\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cases := "## Stated limits\n- QS1: stated\n## Not expressible as a case\n- QS1: valid\n- QH13: starts no process\n- QH1: cites QT4\n"
	proof := ruleProof{Kind: "test", File: "valid_test.go", Function: "TestProof"}
	for _, tc := range []struct {
		name, id string
		proofs   []ruleProof
		pending  map[string]bool
		want     []string
	}{
		{"never_evidence", "QS1", []ruleProof{{Kind: "macos_evidence", File: "never_evidence_test.go", Function: proof.Function}}, nil, []string{"QS1: invalid proof"}},
		{"linux_evidence", "QS1", []ruleProof{{Kind: "macos_evidence", File: "linux_evidence_test.go", Function: proof.Function}}, nil, []string{"QS1: invalid proof"}},
		{"linux_filename", "QS1", []ruleProof{{Kind: "macos_evidence", File: "evidence_linux_test.go", Function: proof.Function}}, nil, []string{"QS1: invalid proof"}},
		{"raw_constraint", "QS1", []ruleProof{{Kind: "macos_evidence", File: "raw_test.go", Function: proof.Function}}, nil, []string{"QS1: invalid proof"}},
		{"inline_limit", "QT4", []ruleProof{{Kind: "limit", Section: "Not expressible as a case", Route: "- QH1: cites QT4"}}, nil, []string{"QT4: invalid proof"}},
		{"existing", "QS1", []ruleProof{proof}, nil, nil},
		{"missing_test", "QS1", []ruleProof{{Kind: "test", File: proof.File, Function: "TestAbsent"}}, nil, []string{"QS1: invalid proof"}},
		{"removed_mapping", "QS1", nil, nil, []string{"QS1: neither mapped nor listed"}},
		{"both", "QS1", []ruleProof{proof}, map[string]bool{"QS1": true}, []string{"QS1: mapped and listed"}},
		{"unknown", "QS1", nil, map[string]bool{"QS1": true, "QZ99": true}, []string{"QZ99: unknown listed rule"}},
		{"gr15_limit", "QH13", []ruleProof{{Kind: "limit", Section: "Not expressible as a case", Route: "- QH13: starts no process"}}, nil, []string{"QH13: macos_evidence required"}},
		{"route_not_line", "QS1", []ruleProof{{Kind: "limit", Section: "Not expressible as a case", Route: "QS1: valid"}}, nil, []string{"QS1: invalid proof"}},
		{"stated_limit", "QS1", []ruleProof{{Kind: "limit", Section: "Stated limits", Route: "- QS1: stated"}}, nil, []string{"QS1: invalid proof"}},
		{"route_other_rule", "QS1", []ruleProof{{Kind: "limit", Section: "Not expressible as a case", Route: "- QH13: starts no process"}}, nil, []string{"QS1: invalid proof"}},
		{"untagged_evidence", "QS1", []ruleProof{{Kind: "macos_evidence", File: proof.File, Function: proof.Function}}, nil, []string{"QS1: invalid proof"}},
		{"hidden_test", "QS1", []ruleProof{{Kind: "test", File: "hidden_test.go", Function: proof.Function}}, nil, []string{"QS1: invalid proof"}},
		{"uncited_test", "QS1", []ruleProof{{Kind: "test", File: "uncited_test.go", Function: proof.Function}}, nil, []string{"QS1: invalid proof"}},
		{"named_test", "QS1", []ruleProof{{Kind: "test", File: "named_test.go", Function: "TestQS1"}}, nil, nil},
		{"subtest", "QS1", []ruleProof{{Kind: "test", File: "subtest_test.go", Function: proof.Function}}, nil, nil},
		{"scripted_test", "QS1", []ruleProof{{Kind: "test", File: "scripted_test.go", Function: proof.Function}}, nil, nil},

		{"valid_evidence", "QH13", []ruleProof{{Kind: "macos_evidence", File: "evidence_test.go", Function: proof.Function}}, nil, nil},
		{"valid_limit", "QS1", []ruleProof{{Kind: "limit", Section: "Not expressible as a case", Route: "- QS1: valid"}}, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mapping := map[string][]ruleProof{}
			if tc.proofs != nil {
				mapping[tc.id] = tc.proofs
			}
			got := ledgerProblems(root, cases, map[string]bool{tc.id: true}, mapping, tc.pending)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRuleGrowthControls(t *testing.T) {
	cmd := exec.Command("python3", "-I", "-B", "../../scripts/test_rule_growth.py")
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s: %v", output, err)
	}
}

func TestEvidenceConstraint(t *testing.T) {
	for _, tc := range []struct {
		source string
		want   bool
	}{
		{"//go:build macos_evidence", true},
		{"//go:build darwin && macos_evidence", true},
		{"//go:build darwin || macos_evidence", false},
		{"// macos_evidence", false},
		{"//go:build !macos_evidence", false},
	} {
		if got := evidenceOnly(tc.source); got != tc.want {
			t.Errorf("%q: got %v, want %v", tc.source, got, tc.want)
		}
	}
}

func TestListGrammar(t *testing.T) {
	got := listEntries("  # comment\n\n  QH1  \nQH2\n")
	want := map[string]bool{"QH1": true, "QH2": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("entries %v, want %v", got, want)
	}
}

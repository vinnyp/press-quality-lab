package docrefs

import (
	"encoding/json"
	"fmt"
	"go/ast"
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
	for _, line := range strings.Split(source, "\n") {
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

func proofExists(root, cases string, p ruleProof) bool {
	if p.Kind == "limit" {
		if p.Section != "Stated limits" && p.Section != "Not expressible as a case" {
			return false
		}
		section := strings.SplitN(cases, "## "+p.Section+"\n", 2)
		if len(section) != 2 || p.Route == "" {
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
	if p.Kind == "macos_evidence" && !evidenceOnly(string(data)) {
		return false
	}
	file, err := parser.ParseFile(token.NewFileSet(), p.File, data, 0)
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
			return true
		}
	}
	return false
}

func ledgerProblems(root, cases string, rules map[string]bool, mapping map[string][]ruleProof, pending map[string]bool) []string {
	var problems []string
	for id := range rules {
		proofs := mapping[id]
		if len(proofs) == 0 && !pending[id] {
			problems = append(problems, id+": neither mapped nor listed")
		}
		if len(proofs) > 0 && pending[id] {
			problems = append(problems, id+": mapped and listed")
		}
		for _, p := range proofs {
			namesRule := false
			for _, token := range clauseToken.FindAllString(p.Route, -1) {
				if token == id {
					namesRule = true
				}
			}
			if !proofExists(root, cases, p) || (p.Kind == "limit" && !namesRule) {
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
	pending := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if line != "" && !strings.HasPrefix(line, "#") {
			pending[line] = true
		}
	}
	cases, err := os.ReadFile(filepath.Join(root, "docs/specs/conformance-collection-1.0/CASES.md"))
	if err != nil {
		t.Fatal(err)
	}
	if ps := ledgerProblems(root, string(cases), rules, ledger.Rules, pending); len(ps) != 0 {
		t.Fatalf("%v", ps)
	}
}

func TestLedgerControls(t *testing.T) {
	proof := ruleProof{Kind: "test", File: "internal/docrefs/ledger_test.go", Function: "TestRuleLedger"}
	for _, tc := range []struct {
		name    string
		mapping map[string][]ruleProof
		pending map[string]bool
		want    []string
	}{
		{"existing", map[string][]ruleProof{"QS1": {proof}}, nil, nil},
		{"missing test", map[string][]ruleProof{"QS1": {{Kind: "test", File: proof.File, Function: "TestAbsent"}}}, nil, []string{"QS1: invalid proof"}},
		{"removed mapping", nil, nil, []string{"QS1: neither mapped nor listed"}},
		{"both", map[string][]ruleProof{"QS1": {proof}}, map[string]bool{"QS1": true}, []string{"QS1: mapped and listed"}},
		{"unknown", nil, map[string]bool{"QS1": true, "QZ99": true}, []string{"QZ99: unknown listed rule"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ledgerProblems("../..", "", map[string]bool{"QS1": true}, tc.mapping, tc.pending)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRuleGrowthControls(t *testing.T) {
	cmd := exec.Command("python3", "-I", "../../scripts/test_rule_growth.py")
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

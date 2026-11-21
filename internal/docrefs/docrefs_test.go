package docrefs

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var definitionPattern = regexp.MustCompile(`(?m)^- (R|AE)([0-9]+)\. `)
var citationPattern = regexp.MustCompile(`\b(R|AE)[0-9]+\b`)

type problem struct {
	line    int
	message string
}
type citation struct {
	id   string
	line int
}

func lineAt(text string, offset int) int { return 1 + strings.Count(text[:offset], "\n") }

func parseDefinitions(text string) (map[string]int, []problem) {
	definitions := make(map[string]int)
	var problems []problem
	for _, match := range definitionPattern.FindAllStringSubmatchIndex(text, -1) {
		id := text[match[2]:match[3]] + text[match[4]:match[5]]
		line := lineAt(text, match[0])
		if first, exists := definitions[id]; exists {
			problems = append(problems, problem{line, fmt.Sprintf("%s is defined twice (lines %d and %d)", id, first, line)})
		} else {
			definitions[id] = line
		}
	}
	if len(definitions) == 0 {
		problems = append(problems, problem{1, "no definitions found"})
	}
	return definitions, problems
}

func findCitations(text string) []citation {
	var citations []citation
	for _, match := range citationPattern.FindAllStringIndex(text, -1) {
		citations = append(citations, citation{text[match[0]:match[1]], lineAt(text, match[0])})
	}
	return citations
}

func unresolved(definitions map[string]int, citations []citation) []problem {
	var problems []problem
	for _, c := range citations {
		if _, exists := definitions[c.id]; !exists {
			problems = append(problems, problem{c.line, c.id + " is not defined in docs/requirements.md"})
		}
	}
	return problems
}

func TestDocsReferencesResolve(t *testing.T) {
	mod, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		t.Fatalf("find module: %v", err)
	}
	path := strings.TrimSpace(string(mod))
	if path == "" || path == "/dev/null" {
		t.Fatal("no module found")
	}
	root := filepath.Dir(path)
	const requirements = "docs/requirements.md"
	data, err := os.ReadFile(filepath.Join(root, requirements))
	if err != nil {
		t.Fatalf("read %s: %v", requirements, err)
	}
	definitions, problems := parseDefinitions(string(data))
	for _, p := range problems {
		t.Errorf("%s:%d: %s", requirements, p.line, p.message)
	}
	total, outside := 0, 0
	err = filepath.WalkDir(filepath.Join(root, "docs"), func(path string, entry fs.DirEntry, walkErr error) error {
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		relative = filepath.ToSlash(relative)
		if walkErr != nil {
			return fmt.Errorf("%s: %w", relative, walkErr)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s: symlink under docs", relative)
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return fmt.Errorf("%s: %w", relative, infoErr)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("%s: %w", relative, readErr)
		}
		citations := findCitations(string(content))
		total += len(citations)
		if relative != requirements {
			outside += len(citations)
		}
		for _, p := range unresolved(definitions, citations) {
			t.Errorf("%s:%d: %s", relative, p.line, p.message)
		}
		return nil
	})
	if err != nil {
		t.Errorf("walk docs: %v", err)
	}
	if outside == 0 {
		t.Error("no citations found outside docs/requirements.md")
	}
	t.Logf("checked %d definitions and %d citations (%d outside docs/requirements.md)", len(definitions), total, outside)
}

func TestRulesOfTheCheck(t *testing.T) {
	tests := []struct {
		name, definitions, text string
		wantDefinitions         int
		wantTokens              string
		wantProblem             string
		wantLine                int
	}{
		{"duplicate", "- R3. first\n- R3. second", "", 1, "", "R3 is defined twice (lines 1 and 2)", 2},
		{"undefined", "- R3. text", "text\nR4", 1, "R4", "R4 is not defined in docs/requirements.md", 2},
		{"non tokens", "- R3. text", "ADR-0006 AR1 R1x xAE2 XR2", 1, "", "", 0},
		{"punctuation", "- R7. text\n- AE12. text\n- R30. text", "R7, (AE12) **R30**", 3, "R7 AE12 R30", "", 0},
		{"definition", "- R3. text", "- R3. text", 1, "R3", "", 0},
		{"missing period", "- R3 text", "", 0, "", "no definitions found", 1},
		{"indented", "  - R3. text", "", 0, "", "no definitions found", 1},
		{"empty", "", "", 0, "", "no definitions found", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			definitions, problems := parseDefinitions(tt.definitions)
			if len(definitions) != tt.wantDefinitions {
				t.Errorf("definitions = %d, want %d", len(definitions), tt.wantDefinitions)
			}
			citations := findCitations(tt.text)
			var ids []string
			for _, c := range citations {
				ids = append(ids, c.id)
			}
			if got := strings.Join(ids, " "); got != tt.wantTokens {
				t.Errorf("tokens = %q, want %q", got, tt.wantTokens)
			}
			problems = append(problems, unresolved(definitions, citations)...)
			if tt.wantProblem == "" {
				if len(problems) != 0 {
					t.Errorf("unexpected problems: %v", problems)
				}
				return
			}
			if len(problems) != 1 {
				t.Fatalf("problems = %v, want one", problems)
			}
			if problems[0].message != tt.wantProblem || problems[0].line != tt.wantLine {
				t.Errorf("problem = %+v, want line %d: %s", problems[0], tt.wantLine, tt.wantProblem)
			}
		})
	}
}

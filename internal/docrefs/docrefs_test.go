package docrefs

import (
	"fmt"
	"io/fs"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"
)

var definitionPattern = regexp.MustCompile(`(?m)^- (R|AE)([0-9]+)\. `)
var citationPattern = regexp.MustCompile(`(?:^|[^A-Za-z0-9])((?:R|AE)[0-9]+)(?:[^A-Za-z0-9]|$)`)

type problem struct {
	line    int
	message string
}
type citation struct {
	invalidRange bool
	id           string
	line         int
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

// definitionGaps is separate so rule fixtures can use sparse definitions when
// testing citation resolution independently of definition completeness.
func definitionGaps(definitions map[string]int) []problem {
	var problems []problem
	for _, prefix := range []string{"R", "AE"} {
		highest := new(big.Int)
		for id := range definitions {
			if !strings.HasPrefix(id, prefix) {
				continue
			}
			n, ok := new(big.Int).SetString(strings.TrimPrefix(id, prefix), 10)
			if ok && n.Cmp(highest) > 0 {
				highest.Set(n)
			}
		}
		for n := big.NewInt(1); n.Cmp(highest) <= 0; n.Add(n, big.NewInt(1)) {
			id := prefix + n.String()
			if _, exists := definitions[id]; !exists {
				problems = append(problems, problem{1, id + " is missing from definitions"})
			}
		}
	}
	return problems
}

func splitID(id string) (string, *big.Int) {
	prefix := "R"
	if strings.HasPrefix(id, "AE") {
		prefix = "AE"
	}
	n, _ := new(big.Int).SetString(strings.TrimPrefix(id, prefix), 10)
	return prefix, n
}

func findCitations(text string) []citation {
	var citations []citation
	previousEnd := 0
	for offset := 0; offset < len(text); {
		match := citationPattern.FindStringSubmatchIndex(text[offset:])
		if match == nil {
			break
		}
		start, end := offset+match[2], offset+match[3]
		id := text[start:end]
		current := citation{id: id, line: lineAt(text, start)}
		if len(citations) > 0 && strings.TrimSpace(text[previousEnd:start]) == "to" &&
			len(text[previousEnd:start]) > len("to") &&
			strings.TrimLeftFunc(text[previousEnd:start], unicode.IsSpace) != text[previousEnd:start] &&
			strings.TrimRightFunc(text[previousEnd:start], unicode.IsSpace) != text[previousEnd:start] {
			previous := citations[len(citations)-1]
			prefix, first := splitID(previous.id)
			endPrefix, last := splitID(id)
			if prefix == endPrefix {
				if last.Cmp(first) <= 0 {
					current.invalidRange = true
				} else {
					for n := new(big.Int).Add(first, big.NewInt(1)); n.Cmp(last) < 0; n.Add(n, big.NewInt(1)) {
						citations = append(citations, citation{id: prefix + n.String(), line: previous.line})
					}
				}
			}
		}
		citations = append(citations, current)
		previousEnd = end
		// Retain the trailing separator for the next match (R1,R2).
		offset = end
	}
	return citations
}

func unresolved(definitions map[string]int, citations []citation) []problem {
	var problems []problem
	for _, c := range citations {
		if c.invalidRange {
			problems = append(problems, problem{c.line, "range end " + c.id + " must be above its start"})
		}
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
	problems = append(problems, definitionGaps(definitions)...)
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
		checkGaps               bool
	}{
		{"duplicate", "- R3. first\n- R3. second", "", 1, "", "R3 is defined twice (lines 1 and 2)", 2, false},
		{"undefined", "- R3. text", "text\nR4", 1, "R4", "R4 is not defined in docs/requirements.md", 2, false},
		{"non tokens", "- R3. text", "ADR-0006 AR1 R1x xAE2 XR2", 1, "", "", 0, false},
		{"punctuation", "- R7. text\n- AE12. text\n- R30. text", "R7, (AE12) **R30**", 3, "R7 AE12 R30", "", 0, false},
		{"definition", "- R3. text", "- R3. text", 1, "R3", "", 0, false},
		{"missing period", "- R3 text", "", 0, "", "no definitions found", 1, false},
		{"indented", "  - R3. text", "", 0, "", "no definitions found", 1, false},
		{"empty", "", "", 0, "", "no definitions found", 1, false},

		{"underscore", "- R1. text", "text\n_R99_", 1, "R99", "R99 is not defined in docs/requirements.md", 2, false},
		{"adjacent comma", "- R1. text\n- R2. text", "R1,R2", 2, "R1 R2", "", 0, false},
		{"adjacent space", "- R1. text\n- R2. text", "R1 R2", 2, "R1 R2", "", 0, false},
		{"underscore AE", "- AE3. text", "_AE3_", 1, "AE3", "", 0, false},
		{"range", "- R28. text\n- R32. text", "R28 to R32", 2, "R28 R29 R30 R31 R32", "R29 is not defined in docs/requirements.md|R30 is not defined in docs/requirements.md|R31 is not defined in docs/requirements.md", 1, false},
		{"descending range", "- R5. text\n- R3. text", "R5 to R3", 2, "R5 R3", "range end R3 must be above its start", 1, false},
		{"equal range", "- R3. text", "R3 to R3", 1, "R3 R3", "range end R3 must be above its start", 1, false},
		{"different prefixes", "- R3. text\n- AE5. text", "R3 to AE5", 2, "R3 AE5", "", 0, false},
		{"AE whitespace range", "- AE16. text\n- AE19. text", "text\nAE16 \n\tto \nAE19", 2, "AE16 AE17 AE18 AE19", "AE17 is not defined in docs/requirements.md|AE18 is not defined in docs/requirements.md", 2, false},
		{"gap", "- R1. text\n- R3. text", "", 2, "", "R2 is missing from definitions", 1, true},
		{"AE gap", "- AE1. text\n- AE3. text", "", 2, "", "AE2 is missing from definitions", 1, true},
		{"contiguous", "- R1. text\n- R2. text\n- R3. text", "", 3, "", "", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			definitions, problems := parseDefinitions(tt.definitions)
			if len(definitions) != tt.wantDefinitions {
				t.Errorf("definitions = %d, want %d", len(definitions), tt.wantDefinitions)
			}
			if tt.checkGaps {
				problems = append(problems, definitionGaps(definitions)...)
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
			wantProblems := strings.Split(tt.wantProblem, "|")
			if len(problems) != len(wantProblems) {
				t.Fatalf("problems = %v, want %v", problems, wantProblems)
			}
			for i, want := range wantProblems {
				if problems[i].message != want || problems[i].line != tt.wantLine {
					t.Errorf("problem = %+v, want line %d: %s", problems[i], tt.wantLine, want)
				}
			}
		})
	}
}

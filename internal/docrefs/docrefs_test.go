package docrefs

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"unicode"
)

// Each floor guards a distinct source of citations: normative prose, evidence
// fields, the engine plan, and the machine-readable published cases.
var coverageFloors = []struct {
	path              string
	excludeCollisions bool
}{
	{"docs/specs/judge-1.0/rulebook.md", true},
	{"docs/specs/judge-1.0/evidence-record.md", true},
	{"docs/plans/2026-11-21-engine-plan.md", true},
	{"docs/specs/judge-1.0/cases/*/case.json", false},
}

var definitionPattern = regexp.MustCompile(`(?m)^- (R|AE)([0-9]+)\. `)

// In the judge specification, R1, R2 and R3 also name comparison results
// (rulebook §11, the judge contract and case files). They resolve to requirements
// by coincidence and prove nothing, so the live coverage floors exclude them.
var citationPattern = regexp.MustCompile(`(?:^|[^A-Za-z0-9])((?:R|AE)[0-9]+)(?:[^A-Za-z0-9]|$)`)

type problem struct {
	line    int
	message string
}
type citation struct {
	invalidRange bool
	wideRange    bool
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
		if _, n := splitID(id); n > 1000 {
			problems = append(problems, problem{line, id + ": number above 1000"})
			continue
		}
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
		highest := 0
		for id := range definitions {
			p, n := splitID(id)
			if p == prefix && n <= 1000 && n > highest {
				highest = n
			}
		}
		for n := 1; n <= highest; n++ {
			id := prefix + strconv.Itoa(n)
			if _, exists := definitions[id]; !exists {
				problems = append(problems, problem{1, id + " is missing from definitions"})
			}
		}
	}
	return problems
}

// Saturate before conversion so arbitrarily long digit strings cannot overflow.
func splitID(id string) (string, int) {
	prefix := "R"
	if strings.HasPrefix(id, "AE") {
		prefix = "AE"
	}
	digits := strings.TrimLeft(strings.TrimPrefix(id, prefix), "0")
	if len(digits) > 4 {
		return prefix, int(^uint(0) >> 1)
	}
	if digits == "" {
		return prefix, 0
	}
	n, err := strconv.Atoi(digits)
	if err != nil {
		return prefix, int(^uint(0) >> 1)
	}
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
			strings.TrimLeftFunc(text[previousEnd:start], unicode.IsSpace) != text[previousEnd:start] &&
			strings.TrimRightFunc(text[previousEnd:start], unicode.IsSpace) != text[previousEnd:start] {
			previous := citations[len(citations)-1]
			prefix, first := splitID(previous.id)
			endPrefix, last := splitID(id)
			if prefix == endPrefix {
				if last-first > 1000 {
					current.wideRange = true
				} else if first > 1000 || last > 1000 {
					// Invalid endpoints are reported without expanding the range.
				} else if last <= first {
					current.invalidRange = true
				} else {
					for n := first + 1; n < last; n++ {
						citations = append(citations, citation{id: prefix + strconv.Itoa(n), line: previous.line})
					}
				}
			}
		}
		citations = append(citations, current)
		previousEnd = end
		// Restart at the token end so adjacent tokens can share a separator (R1,R2).
		offset = end
	}
	return citations
}

func unresolved(definitions map[string]int, citations []citation) []problem {
	var problems []problem
	for _, c := range citations {
		if c.wideRange {
			problems = append(problems, problem{c.line, "range end " + c.id + ": range wider than 1000"})
		}
		if _, n := splitID(c.id); n > 1000 {
			problems = append(problems, problem{c.line, c.id + ": number above 1000"})
			continue
		}
		if c.invalidRange {
			problems = append(problems, problem{c.line, "range end " + c.id + " must be above its start"})
		}
		if _, exists := definitions[c.id]; !exists {
			problems = append(problems, problem{c.line, c.id + " is not defined in docs/requirements.md"})
		}
	}
	return problems
}

const (
	requirements           = "docs/requirements.md"
	requirementsDigest     = "c4178fab76aaf76012d68341f4a6169f3ab495c67c19cb60cea6b5592588652c"
	requirementCount       = 42
	acceptanceExampleCount = 36
)

var entryPattern = regexp.MustCompile(`(?m)^- (?:R|AE)[0-9]+\. .*$`)
var actorFlowPattern = regexp.MustCompile(`(?m)^- ((?:A|F)[0-9]+)\. `)

type stats struct {
	definitions, total, outside, collisions int
	citations                               map[string][]citation
}

func pinnedEntries(text string, definitions map[string]int) []problem {
	var problems []problem
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(entryPattern.FindAllString(text, -1), "\n"))))
	if digest != requirementsDigest {
		problems = append(problems, problem{1, "R and AE entries changed: their numbers and wording do not change (AGENTS.md); a deliberate, owner-approved change must update the digest constant in the same pull request"})
	}
	for _, series := range []struct {
		prefix string
		count  int
	}{{"R", requirementCount}, {"AE", acceptanceExampleCount}} {
		for n := 1; n <= series.count; n++ {
			id := series.prefix + strconv.Itoa(n)
			if _, ok := definitions[id]; !ok {
				problems = append(problems, problem{1, id + " is required in definitions"})
			}
		}
		// Report unexpected entries in file order, rather than map iteration order.
		for _, match := range definitionPattern.FindAllStringSubmatch(text, -1) {
			id := match[1] + match[2]
			prefix, n := splitID(id)
			if prefix == series.prefix && (n < 1 || n > series.count) {
				problems = append(problems, problem{1, id + " is outside the required definitions"})
			}
		}
	}
	counts := map[string]int{}
	for _, match := range actorFlowPattern.FindAllStringSubmatch(text, -1) {
		counts[match[1]]++
	}
	for _, series := range []struct {
		prefix string
		count  int
	}{{"A", 4}, {"F", 5}} {
		for n := 1; n <= series.count; n++ {
			id := series.prefix + strconv.Itoa(n)
			if counts[id] != 1 {
				problems = append(problems, problem{1, fmt.Sprintf("%s must appear exactly once (found %d)", id, counts[id])})
			}
		}
	}
	return problems
}

func checkDocs(fsys fs.FS) (problems []string, result stats) {
	result.citations = make(map[string][]citation)
	report := func(name string, ps []problem) {
		for _, p := range ps {
			problems = append(problems, fmt.Sprintf("%s:%d: %s", name, p.line, p.message))
		}
	}
	data, err := fs.ReadFile(fsys, requirements)
	if err != nil {
		report(requirements, []problem{{1, "read requirements: " + err.Error()}})
		return problems, result
	}
	definitions, ps := parseDefinitions(string(data))
	result.definitions = len(definitions)
	report(requirements, ps)
	report(requirements, definitionGaps(definitions))
	report(requirements, pinnedEntries(string(data), definitions))
	err = fs.WalkDir(fsys, "docs", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			report(name, []problem{{1, walkErr.Error()}})
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			report(name, []problem{{1, "symlink under docs"}})
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			report(name, []problem{{1, infoErr.Error()}})
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		content, readErr := fs.ReadFile(fsys, name)
		if readErr != nil {
			report(name, []problem{{1, readErr.Error()}})
			return nil
		}
		citations := findCitations(string(content))
		result.citations[name] = citations
		result.total += len(citations)
		if name != requirements {
			result.outside += len(citations)
		}
		for _, c := range citations {
			if collision(c.id) {
				result.collisions++
			}
		}
		report(name, unresolved(definitions, citations))
		return nil
	})
	if err != nil {
		report("docs", []problem{{1, err.Error()}})
	}
	return problems, result
}

func collision(id string) bool { return id == "R1" || id == "R2" || id == "R3" }

func repositoryRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if info, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil && info.Mode().IsRegular() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found above working directory")
		}
		dir = parent
	}
}

func TestDocsReferencesResolve(t *testing.T) {
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	problems, result := checkDocs(os.DirFS(root))
	for _, p := range problems {
		t.Error(p)
	}
	for _, floor := range coverageFloors {
		count := 0
		for name, citations := range result.citations {
			matches, matchErr := path.Match(floor.path, name)
			if matchErr != nil {
				t.Fatal(matchErr)
			}
			if matches {
				for _, c := range citations {
					if !floor.excludeCollisions || !collision(c.id) {
						count++
					}
				}
			}
		}
		if count == 0 {
			t.Errorf("citation coverage floor not met: %s", floor.path)
		}
	}
	t.Logf("checked %d definitions and %d citations (%d outside docs/requirements.md; %d R1-R3 collision tokens)", result.definitions, result.total, result.outside, result.collisions)
}

func TestCheckDocsOnFixtures(t *testing.T) {
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join(root, requirements))
	if err != nil {
		t.Fatal(err)
	}
	clean := string(source)
	digestProblem := requirements + ":1: R and AE entries changed: their numbers and wording do not change (AGENTS.md); a deliberate, owner-approved change must update the digest constant in the same pull request"
	tests := []struct {
		name, requirementsText string
		extra                  fstest.MapFS
		missing                bool
		want                   []string
	}{
		{name: "clean", requirementsText: clean},
		{name: "missing actor", requirementsText: strings.Replace(clean, "- A1. ", "- removedA1. ", 1), want: []string{requirements + ":1: A1 must appear exactly once (found 0)"}},
		{name: "duplicate flow", requirementsText: clean + "\n- F5. duplicate", want: []string{requirements + ":1: F5 must appear exactly once (found 2)"}},
		{name: "changed wording", requirementsText: strings.Replace(clean, "- R5. ", "- R5. changed ", 1), want: []string{digestProblem}},
		{name: "nested JSON", requirementsText: clean, extra: fstest.MapFS{"docs/specs/x/cases/c/case.json": {Data: []byte("{\n\"citation\": \"R99\"\n}")}}, want: []string{"docs/specs/x/cases/c/case.json:2: R99 is not defined in docs/requirements.md"}},
		{name: "draft kit JSON", requirementsText: clean, extra: fstest.MapFS{"docs/specs/conformance-collection-1.0/examples/fixture.json": {Data: []byte("{\n\"citation\": \"R99\"\n}")}}, want: []string{"docs/specs/conformance-collection-1.0/examples/fixture.json:2: R99 is not defined in docs/requirements.md"}},
		{name: "duplicate", requirementsText: clean + "\n- R5. duplicate", want: []string{fmt.Sprintf("%s:%d: R5 is defined twice (lines %d and %d)", requirements, lineAt(clean, len(clean))+1, lineAt(clean, strings.Index(clean, "- R5. ")), lineAt(clean, len(clean))+1), digestProblem}},
		{name: "gap", requirementsText: strings.Replace(clean, "- R5. ", "- removed5. ", 1), want: func() []string {
			want := []string{requirements + ":1: R5 is missing from definitions", digestProblem, requirements + ":1: R5 is required in definitions"}
			for _, c := range findCitations(strings.Replace(clean, "- R5. ", "- removed5. ", 1)) {
				if c.id == "R5" {
					want = append(want, fmt.Sprintf("%s:%d: R5 is not defined in docs/requirements.md", requirements, c.line))
				}
			}
			return want
		}()},
		{name: "symlink", requirementsText: clean, extra: fstest.MapFS{"docs/link": {Mode: fs.ModeSymlink}}, want: []string{"docs/link:1: symlink under docs"}},
		{name: "missing requirements", missing: true, want: []string{requirements + ":1: read requirements: open docs/requirements.md: file does not exist"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := fstest.MapFS{}
			if !tt.missing {
				fixture[requirements] = &fstest.MapFile{Data: []byte(tt.requirementsText)}
			}
			for name, file := range tt.extra {
				fixture[name] = file
			}
			got, _ := checkDocs(fixture)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("problems = %#v, want %#v", got, tt.want)
			}
		})
	}
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
		{"large definition", "- R1001. text\n- R1. text", "", 1, "", "R1001: number above 1000", 1, false},
		{"large citation", "- R1. text", "R999999999999999999999999", 1, "R999999999999999999999999", "R999999999999999999999999: number above 1000", 1, false},
		{"wide range", "- R1. text", "R1 to R1002", 1, "R1 R1002", "range end R1002: range wider than 1000|R1002: number above 1000", 1, false},
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

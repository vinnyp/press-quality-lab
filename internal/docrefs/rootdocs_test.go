package docrefs

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
	"unicode"
)

var rootDocuments = []string{"AGENTS.md", "CODING_STANDARDS.md", "REVIEW.md", "CONCEPTS.md"}
var collectionDefinition = regexp.MustCompile(`(?m)^- \*\*(Q[A-Z]+[0-9]+[a-z]?)\. `)
var judgeDefinition = regexp.MustCompile(`(?m)^- \*\*([A-Z][0-9]+[a-z]?)\.`)
var clauseToken = regexp.MustCompile(`Q[A-Z]+[0-9]+[a-z]?|[A-Z][0-9]+[a-z]?`)
var clauseFamily = regexp.MustCompile(`^[A-Z]+`)

type clauseLocation struct {
	file string
	line int
}
type rootCounts struct{ collection, judge, requirements, paths int }

// Each total guards its current source across the four docs, not each file.
func rootCoverage(c rootCounts) []string {
	var ps []string
	for _, floor := range []struct {
		name  string
		count int
	}{
		{"collection id", c.collection},    // CODING_STANDARDS.md's collection rules.
		{"judge id", c.judge},              // CODING_STANDARDS.md's judge rules.
		{"requirement id", c.requirements}, // AGENTS.md's R and AE ranges.
		{"path", c.paths},                  // Repository paths in AGENTS.md and CODING_STANDARDS.md.
	} {
		if floor.count == 0 {
			ps = append(ps, "root docs:1: no "+floor.name+" references found across the four root docs; restore citation coverage")
		}
	}
	return ps
}

func specificationDefinitions(fsys fs.FS, prefix string, pattern *regexp.Regexp) (map[string]clauseLocation, []string) {
	definitions := make(map[string]clauseLocation)
	var ps []string
	entries, err := fs.ReadDir(fsys, "docs/specs")
	if err != nil {
		return definitions, []string{"docs/specs:1: read specification directories: " + err.Error() + "; restore the specification directories"}
	}
	var folders []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
			folders = append(folders, entry.Name())
		}
	}
	if len(folders) != 1 {
		return definitions, []string{fmt.Sprintf("docs/specs:1: %s root docs' ids name no version; need exactly one version directory (found %d)", strings.TrimSuffix(prefix, "-"), len(folders))}
	}
	folder := path.Join("docs/specs", folders[0])
	files, err := fs.Glob(fsys, folder+"/*.md")
	if err != nil {
		return definitions, []string{folder + ":1: list definitions: " + err.Error() + "; correct specification directory names"}
	}
	for _, name := range files {
		data, readErr := fs.ReadFile(fsys, name)
		if readErr != nil {
			ps = append(ps, name+":1: read definitions: "+readErr.Error()+"; restore the definition file")
			continue
		}
		text := string(data)
		for _, match := range pattern.FindAllStringSubmatchIndex(text, -1) {
			id := text[match[2]:match[3]]
			at := clauseLocation{name, lineAt(text, match[0])}
			if first, exists := definitions[id]; exists {
				ps = append(ps, fmt.Sprintf("%s:%d: %s is defined twice (first at %s:%d); make definitions unique", name, at.line, id, first.file, first.line))
			} else {
				definitions[id] = at
			}
		}
	}
	if len(definitions) == 0 {
		ps = append(ps, folder+":1: no clause definitions found; check definition syntax")
	}
	return definitions, ps
}

func findClauseCitations(text string) []citation {
	var found []citation
	for _, m := range clauseToken.FindAllStringIndex(text, -1) {
		start, end := m[0], m[1]
		// ASCII alphanumeric boundaries include the optional suffix in the token.
		alnum := func(b byte) bool { return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b >= '0' && b <= '9' }
		if start > 0 && alnum(text[start-1]) || end < len(text) && alnum(text[end]) {
			continue
		}
		found = append(found, citation{id: text[start:end], line: lineAt(text, start)})
	}
	return found
}

var repositoryExtension = regexp.MustCompile(`\.(?:md|sh|go|mod|sum|json|yaml|yml|py)$`)
var hostLabel = regexp.MustCompile(`^[A-Za-z]{2,}$`)

func repositoryPath(span string) bool {
	if span == "" || strings.ContainsAny(span, "<>*{}@:") || strings.ContainsAny(span[:1], "$~/") || strings.IndexFunc(span, unicode.IsSpace) >= 0 {
		return false
	}
	hasSlash := strings.Contains(span, "/")
	if !hasSlash {
		return repositoryExtension.MatchString(span)
	}
	first := strings.SplitN(span, "/", 2)[0]
	labels := strings.Split(first, ".")
	if !strings.HasPrefix(first, ".") && len(labels) > 1 && hostLabel.MatchString(labels[len(labels)-1]) {
		return false
	}
	return true
}

func findRootPaths(text string) ([]citation, []problem) {
	var problems []problem
	fenceLine := 0
	var found []citation
	fence := byte(0)
	fenceLength := 0
	offset := 0
	for _, line := range strings.SplitAfter(text, "\n") {
		trimmed := strings.TrimLeft(line, " ")
		if len(line)-len(trimmed) > 3 {
			trimmed = ""
		}
		run := 0
		if len(trimmed) > 0 && (trimmed[0] == '`' || trimmed[0] == '~') {
			for run < len(trimmed) && trimmed[run] == trimmed[0] {
				run++
			}
		}
		if fence != 0 {
			if run >= fenceLength && trimmed[0] == fence && strings.TrimSpace(trimmed[run:]) == "" {
				fence = 0
			}
		} else if run >= 3 && !strings.Contains(trimmed[run:], "`") {
			fenceLine = lineAt(text, offset)
			fence = trimmed[0]
			fenceLength = run
		} else {
			for pos := 0; pos < len(line); {
				start := strings.IndexByte(line[pos:], '`')
				if start < 0 {
					break
				}
				start += pos
				width := 1
				for start+width < len(line) && line[start+width] == '`' {
					width++
				}
				delimiter := strings.Repeat("`", width)
				end := strings.Index(line[start+width:], delimiter)
				if end < 0 {
					pos = start + width
					continue
				}
				end += start + width
				span := line[start+width : end]
				if repositoryPath(span) {
					found = append(found, citation{id: span, line: lineAt(text, offset+start)})
				}
				pos = end + width
			}
		}
		offset += len(line)
	}
	if fence != 0 {
		problems = append(problems, problem{fenceLine, "unclosed fence; close the fence so following paths are checked"})
	}
	return found, problems
}

// Translating a family lets the shared range parser apply exactly the R rules.
// Mask R and AE tokens without changing byte offsets, then restore the family.
func findActorFlowCitations(text string) []citation {
	var found []citation
	for _, family := range []string{"A", "F"} {
		masked := []byte(strings.Map(func(r rune) rune {
			if r == 'R' || r == 'A' {
				return '#'
			}
			return r
		}, text))
		pattern := regexp.MustCompile(`(?:^|[^A-Za-z0-9])(` + family + `[0-9]+)(?:[^A-Za-z0-9]|$)`)
		for offset := 0; offset < len(text); {
			m := pattern.FindStringSubmatchIndex(text[offset:])
			if m == nil {
				break
			}
			start, end := offset+m[2], offset+m[3]
			copy(masked[start:end], text[start:end])
			masked[start] = 'R'
			offset = end
		}
		for _, c := range findCitations(string(masked)) {
			c.id = family + strings.TrimPrefix(c.id, "R")
			found = append(found, c)
		}
	}
	return found
}

func unresolvedActorFlows(definitions map[string]int, refs []citation) []problem {
	var ps []problem
	for _, c := range refs {
		family := c.id[:1]
		translated := make(map[string]int)
		for id, line := range definitions {
			if strings.HasPrefix(id, family) {
				translated["R"+id[1:]] = line
			}
		}
		c.id = "R" + c.id[1:]
		for _, p := range unresolved(translated, []citation{c}) {
			p.message = regexp.MustCompile(`R([0-9]+)`).ReplaceAllString(p.message, family+"${1}")
			ps = append(ps, p)
		}
	}
	return ps
}

// checkRootDocs proves that a cited reference exists, not that it is the right
// clause for the rule citing it. Section references and other docs are outside
// this check's scope.
func checkRootDocs(fsys fs.FS) ([]string, rootCounts) {
	var counts rootCounts
	collection, ps := specificationDefinitions(fsys, "conformance-collection-", collectionDefinition)
	judge, more := specificationDefinitions(fsys, "judge-", judgeDefinition)
	ps = append(ps, more...)
	families := make(map[string]bool)
	for id := range judge {
		family := clauseFamily.FindString(id)
		if family != "R" && family != "A" && family != "F" {
			families[family] = true
		}
	}
	req := make(map[string]int)
	data, err := fs.ReadFile(fsys, requirements)
	if err != nil {
		ps = append(ps, requirements+":1: read requirements: "+err.Error()+"; restore docs/requirements.md")
	} else {
		var problems []problem
		req, problems = parseDefinitions(string(data))
		for _, p := range problems {
			ps = append(ps, fmt.Sprintf("%s:%d: %s; correct the requirement definitions", requirements, p.line, p.message))
		}
	}
	actors := make(map[string]int)
	for _, match := range actorFlowPattern.FindAllStringSubmatchIndex(string(data), -1) {
		id := string(data[match[2]:match[3]])
		if first, ok := actors[id]; ok {
			ps = append(ps, fmt.Sprintf("%s:%d: %s is defined twice (first at line %d); make definitions unique", requirements, lineAt(string(data), match[0]), id, first))
		} else {
			actors[id] = lineAt(string(data), match[0])
		}
	}
	for _, name := range rootDocuments {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			ps = append(ps, name+":1: read root doc: "+err.Error()+"; restore the root doc")
			continue
		}
		text := string(data)
		for _, c := range findClauseCitations(text) {
			defs := judge
			family := clauseFamily.FindString(c.id)
			if strings.HasPrefix(family, "Q") && len(family) > 1 {
				counts.collection++
				defs = collection
			} else if families[family] {
				counts.judge++
			} else {
				continue
			}
			if _, ok := defs[c.id]; !ok {
				ps = append(ps, fmt.Sprintf("%s:%d: %s is not defined in its specification; correct the cited id", name, c.line, c.id))
			}
		}
		refs := findCitations(text)
		counts.requirements += len(refs)
		for _, p := range unresolved(req, refs) {
			ps = append(ps, fmt.Sprintf("%s:%d: %s; correct the cited id or range", name, p.line, p.message))
		}
		for _, p := range unresolvedActorFlows(actors, findActorFlowCitations(text)) {
			ps = append(ps, fmt.Sprintf("%s:%d: %s; correct the cited id or range", name, p.line, p.message))
		}
		paths, fenceProblems := findRootPaths(text)
		for _, p := range fenceProblems {
			ps = append(ps, fmt.Sprintf("%s:%d: %s", name, p.line, p.message))
		}
		for _, c := range paths {
			counts.paths++
			if _, err := fs.Stat(fsys, strings.TrimSuffix(c.id, "/")); err != nil {
				ps = append(ps, fmt.Sprintf("%s:%d: %s repository path does not resolve: %v; correct the cited path", name, c.line, c.id, err))
			}
		}
	}
	ps = append(ps, rootCoverage(counts)...)
	sort.Strings(ps)
	return ps, counts
}

func TestRootDocsReferencesResolve(t *testing.T) {
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	ps, counts := checkRootDocs(os.DirFS(root))
	for _, p := range ps {
		t.Error(p)
	}
	t.Logf("checked %d collection ids, %d judge ids, %d requirement ids and %d paths", counts.collection, counts.judge, counts.requirements, counts.paths)
}

func rootFixture() fstest.MapFS {
	return fstest.MapFS{
		"docs/specs/conformance-collection-1.0/rules.md": {Data: []byte("- **QH8. text\n")},
		"docs/specs/judge-1.0/rules.md":                  {Data: []byte("- **E9a. title** text\n- **P3.** text\n- **R1. result**\n- **A1. judge**\n- **F1. judge**\n")},
		"docs/requirements.md":                           {Data: []byte("- R1. one\n- R2. two\n- R4. four\n- R42. last\n- AE36. last\n- A1. actor\n- A3. actor\n- F1. flow\n- F2. flow\n- F3. flow\n")},
		"docs/exists.md":                                 {Data: []byte("exists")},
		"gate.sh":                                        {},
		"AGENTS.md":                                      {Data: []byte("QH8 E9a R42 AE36 R2 `docs/exists.md`\n")},
		"CODING_STANDARDS.md":                            {}, "REVIEW.md": {}, "CONCEPTS.md": {},
	}
}

func TestRootDocFixtures(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		change func(fstest.MapFS)
		want   []string
		counts rootCounts
	}{
		{name: "clean", counts: rootCounts{1, 1, 3, 1}},
		{name: "undefined_collection", text: "text\nQH99", want: []string{"REVIEW.md:2: QH99 is not defined in its specification; correct the cited id"}, counts: rootCounts{2, 1, 3, 1}},
		{name: "undefined_judge", text: "text\nE99", want: []string{"REVIEW.md:2: E99 is not defined in its specification; correct the cited id"}, counts: rootCounts{1, 2, 3, 1}},
		{name: "boundaries", text: "AQH8 E9a QH8x Q1", want: []string{"REVIEW.md:1: QH8x is not defined in its specification; correct the cited id"}, counts: rootCounts{2, 2, 3, 1}},
		{name: "unbounded_ids", text: "QH1234 E1234", want: []string{"REVIEW.md:1: E1234 is not defined in its specification; correct the cited id", "REVIEW.md:1: QH1234 is not defined in its specification; correct the cited id"}, counts: rootCounts{2, 2, 3, 1}},
		{name: "collection_suffix", text: "QH8a", want: []string{"REVIEW.md:1: QH8a is not defined in its specification; correct the cited id"}, counts: rootCounts{2, 1, 3, 1}},
		{name: "subfolder", text: "P4", change: func(f fstest.MapFS) {
			f["docs/specs/judge-1.0/cases/case-001/README.md"] = &fstest.MapFile{Data: []byte("- **P4.** text")}
		}, want: []string{"REVIEW.md:1: P4 is not defined in its specification; correct the cited id"}, counts: rootCounts{1, 2, 3, 1}},
		{name: "judge_anchor", text: "P4", change: func(f fstest.MapFS) {
			f["docs/specs/judge-1.0/rules.md"].Data = append(f["docs/specs/judge-1.0/rules.md"].Data, []byte("inline **P3.**\n  - **P4. title\nsee - **P4.** here\n")...)
		}, want: []string{"REVIEW.md:1: P4 is not defined in its specification; correct the cited id"}, counts: rootCounts{1, 2, 3, 1}},
		{name: "collection_anchor", text: "QH9", change: func(f fstest.MapFS) {
			f["docs/specs/conformance-collection-1.0/rules.md"].Data = append(f["docs/specs/conformance-collection-1.0/rules.md"].Data, []byte("  - **QH9. text\nsee - **QH9. text\n")...)
		}, want: []string{"REVIEW.md:1: QH9 is not defined in its specification; correct the cited id"}, counts: rootCounts{2, 1, 3, 1}},
		{name: "duplicate", change: func(f fstest.MapFS) {
			f["docs/specs/judge-1.0/second.md"] = &fstest.MapFile{Data: []byte("- **P3.** duplicate")}
		}, want: []string{"docs/specs/judge-1.0/second.md:1: P3 is defined twice (first at docs/specs/judge-1.0/rules.md:2); make definitions unique"}, counts: rootCounts{1, 1, 3, 1}},
		{name: "R_resolution", text: "R43", want: []string{"REVIEW.md:1: R43 is not defined in docs/requirements.md; correct the cited id or range"}, counts: rootCounts{1, 1, 4, 1}},
		{name: "AE_resolution", text: "AE37", want: []string{"REVIEW.md:1: AE37 is not defined in docs/requirements.md; correct the cited id or range"}, counts: rootCounts{1, 1, 4, 1}},
		{name: "R_exclusion", text: "R2", counts: rootCounts{1, 1, 4, 1}},
		{name: "range", text: "R1 to R4", want: []string{"REVIEW.md:1: R3 is not defined in docs/requirements.md; correct the cited id or range"}, counts: rootCounts{1, 1, 7, 1}},
		{name: "actors_flows", text: "A3 F2", counts: rootCounts{1, 1, 3, 1}},
		{name: "A_resolution", text: "A9", want: []string{"REVIEW.md:1: A9 is not defined in docs/requirements.md; correct the cited id or range"}, counts: rootCounts{1, 1, 3, 1}},
		{name: "F_resolution", text: "F9", want: []string{"REVIEW.md:1: F9 is not defined in docs/requirements.md; correct the cited id or range"}, counts: rootCounts{1, 1, 3, 1}},
		{name: "actor_range", text: "A1 to A3", want: []string{"REVIEW.md:1: A2 is not defined in docs/requirements.md; correct the cited id or range"}, counts: rootCounts{1, 1, 3, 1}},
		{name: "flow_range", text: "F1 to F3", counts: rootCounts{1, 1, 3, 1}},
		{name: "directory_lookup", change: func(f fstest.MapFS) {
			f["docs/specs/judge-1.0-errata.md"] = &fstest.MapFile{}
			f["docs/specs/conformance-collection-1.0-seal.md"] = &fstest.MapFile{}
		}, counts: rootCounts{1, 1, 3, 1}},
		{name: "directories", text: "`docs/specs` `docs/specs/`", counts: rootCounts{1, 1, 3, 3}},
		{name: "missing_path", text: "text\n`docs/missing.md`", want: []string{"REVIEW.md:2: docs/missing.md repository path does not resolve: open docs/missing.md: file does not exist; correct the cited path"}, counts: rootCounts{1, 1, 3, 2}},
		{name: "host_rule", text: "`judge-1.0/x`", want: []string{"REVIEW.md:1: judge-1.0/x repository path does not resolve: open judge-1.0/x: file does not exist; correct the cited path"}, counts: rootCounts{1, 1, 3, 2}},
		{name: "root_files", text: "`gate.sh` `scann.sh` `pql`", want: []string{"REVIEW.md:1: scann.sh repository path does not resolve: open scann.sh: file does not exist; correct the cited path"}, counts: rootCounts{1, 1, 3, 3}},
		{name: "path_exclusions", text: "`$HOME/x/y` `~/x/y` `/abs/x` `github.com/a/b` `users.noreply.github.com` `docs/<name>/x` `docs/*/x` `docs/{x}/y` `docs/a b` `a@b/x` `a:b/x`", counts: rootCounts{1, 1, 3, 1}},
		{name: "dot_directory", text: "`.github/workflows`", want: []string{"REVIEW.md:1: .github/workflows repository path does not resolve: open .github/workflows: file does not exist; correct the cited path"}, counts: rootCounts{1, 1, 3, 2}},
		{name: "parent_segment", text: "`docs/../AGENTS.md`", want: []string{"REVIEW.md:1: docs/../AGENTS.md repository path does not resolve: open docs/../AGENTS.md: file does not exist; correct the cited path"}, counts: rootCounts{1, 1, 3, 2}},
		{name: "every_span", text: "`docs/exists.md` `docs/later.md`", want: []string{"REVIEW.md:1: docs/later.md repository path does not resolve: open docs/later.md: file does not exist; correct the cited path"}, counts: rootCounts{1, 1, 3, 3}},
		{name: "fenced_skip", text: "```sh\n`docs/missing`\n```\n~~~\n`docs/also-missing`\n~~~\n", counts: rootCounts{1, 1, 3, 1}},
		{name: "fence_closer", text: "````sh\n```\n`docs/hidden`\n`````\n`docs/exists.md`\n", counts: rootCounts{1, 1, 3, 2}},
		{name: "fence_closer_text", text: "```\n```go\n`docs/hidden`\n```\n`docs/exists.md`", counts: rootCounts{1, 1, 3, 2}},
		{name: "indented_fence", text: "    ```\n`docs/missing.md`", want: []string{"REVIEW.md:2: docs/missing.md repository path does not resolve: open docs/missing.md: file does not exist; correct the cited path"}, counts: rootCounts{1, 1, 3, 2}},
		{name: "unpaired_backtick", text: "the ``` mark and `docs/missing.md`", want: []string{"REVIEW.md:1: docs/missing.md repository path does not resolve: open docs/missing.md: file does not exist; correct the cited path"}, counts: rootCounts{1, 1, 3, 2}},
		{name: "fence_character", text: "```\n~~~\n`docs/hidden`\n```\n~~~\n```\n`docs/also-hidden`\n~~~\n`docs/exists.md`", counts: rootCounts{1, 1, 3, 2}},
		{name: "unclosed_fence", text: "text\n```sh\n`docs/hidden`", want: []string{"REVIEW.md:2: unclosed fence; close the fence so following paths are checked"}, counts: rootCounts{1, 1, 3, 1}},
		{name: "invalid_opener", text: "```x```\n`docs/exists.md`", counts: rootCounts{1, 1, 3, 2}},
		{name: "concepts_path", change: func(f fstest.MapFS) { f["CONCEPTS.md"].Data = []byte("`docs/exists.md`") }, counts: rootCounts{1, 1, 3, 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := rootFixture()
			f["REVIEW.md"].Data = []byte(tt.text)
			if tt.change != nil {
				tt.change(f)
			}
			ps, counts := checkRootDocs(f)
			if !reflect.DeepEqual(ps, tt.want) {
				t.Errorf("%s: problems = %#v, want %#v", tt.name, ps, tt.want)
			}
			if counts != tt.counts {
				t.Errorf("%s: counts = %+v, want %+v", tt.name, counts, tt.counts)
			}
		})
	}
}

func TestRootDocFloors(t *testing.T) {
	for _, tt := range []struct {
		name, text string
		counts     rootCounts
	}{
		{"collection_id", "E9a R42 AE36 R2 `docs/exists.md`", rootCounts{0, 1, 3, 1}},
		{"judge_id", "QH8 R42 AE36 R2 `docs/exists.md`", rootCounts{1, 0, 3, 1}},
		{"requirement_id", "QH8 E9a `docs/exists.md`", rootCounts{1, 1, 0, 1}},
		{"path", "QH8 E9a R42 AE36 R2", rootCounts{1, 1, 3, 0}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := rootFixture()
			f["AGENTS.md"].Data = []byte(tt.text)
			ps, counts := checkRootDocs(f)
			want := []string{"root docs:1: no " + strings.ReplaceAll(tt.name, "_", " ") + " references found across the four root docs; restore citation coverage"}
			if !reflect.DeepEqual(ps, want) {
				t.Errorf("%s: problems = %#v, want %#v", tt.name, ps, want)
			}
			if counts != tt.counts {
				t.Errorf("%s: counts = %+v, want %+v", tt.name, counts, tt.counts)
			}
		})
	}
}

func TestRootDocSet(t *testing.T) {
	want := []string{"AGENTS.md", "CODING_STANDARDS.md", "REVIEW.md", "CONCEPTS.md"}
	if !reflect.DeepEqual(rootDocuments, want) {
		t.Errorf("checked docs = %#v, want %#v", rootDocuments, want)
	}
	for _, name := range want {
		t.Run(strings.TrimSuffix(name, ".md"), func(t *testing.T) {
			f := rootFixture()
			// Keep coverage independent of the missing document.
			f["AGENTS.md"].Data = nil
			other := "AGENTS.md"
			if name == other {
				other = "REVIEW.md"
			}
			f[other].Data = []byte("QH8 E9a R42 AE36 R2 `docs/exists.md`")
			delete(f, name)
			ps, counts := checkRootDocs(f)
			want := []string{name + ":1: read root doc: open " + name + ": file does not exist; restore the root doc"}
			if !reflect.DeepEqual(ps, want) {
				t.Errorf("%s: problems = %#v, want %#v", name, ps, want)
			}
			if counts != (rootCounts{1, 1, 3, 1}) {
				t.Errorf("%s: counts = %+v, want {1 1 3 1}", name, counts)
			}
		})
	}
}

func TestRootDocVersions(t *testing.T) {
	for _, tt := range []struct {
		name, prefix, text string
		two                bool
		counts             rootCounts
		extra              []string
	}{
		{"no_collection", "conformance-collection", "E9a R42 `docs/exists.md`", false, rootCounts{0, 1, 1, 1}, []string{"root docs:1: no collection id references found across the four root docs; restore citation coverage"}},
		{"two_collections", "conformance-collection", "QH8 E9a R42 `docs/exists.md`", true, rootCounts{1, 1, 1, 1}, []string{"AGENTS.md:1: QH8 is not defined in its specification; correct the cited id"}},
		{"no_judge", "judge", "QH8 R42 `docs/exists.md`", false, rootCounts{1, 0, 1, 1}, []string{"root docs:1: no judge id references found across the four root docs; restore citation coverage"}},
		{"two_judges", "judge", "QH8 R42 `docs/exists.md`", true, rootCounts{1, 0, 1, 1}, []string{"root docs:1: no judge id references found across the four root docs; restore citation coverage"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := rootFixture()
			f["AGENTS.md"].Data = []byte(tt.text)
			n := 0
			if tt.two {
				f["docs/specs/"+tt.prefix+"-2.0/rules.md"] = &fstest.MapFile{}
				n = 2
			} else {
				delete(f, "docs/specs/"+tt.prefix+"-1.0/rules.md")
			}
			want := append([]string{fmt.Sprintf("docs/specs:1: %s root docs' ids name no version; need exactly one version directory (found %d)", tt.prefix, n)}, tt.extra...)
			sort.Strings(want)
			ps, counts := checkRootDocs(f)
			if !reflect.DeepEqual(ps, want) {
				t.Errorf("%s: problems = %#v, want %#v", tt.name, ps, want)
			}
			if counts != tt.counts {
				t.Errorf("%s: counts = %+v, want %+v", tt.name, counts, tt.counts)
			}
		})
	}
}

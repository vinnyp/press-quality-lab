package docrefs

import (
	"fmt"
	"io/fs"
	"os"
	"path"
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
var clauseToken = regexp.MustCompile(`Q[A-Z]+[0-9]+|[A-Z][0-9]+[a-z]?`)
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
			ps = append(ps, "AGENTS.md:1: no "+floor.name+" references found across the four root docs; restore citation coverage")
		}
	}
	return ps
}

func specificationDefinitions(fsys fs.FS, prefix string, pattern *regexp.Regexp) (map[string]clauseLocation, []string) {
	definitions := make(map[string]clauseLocation)
	var ps []string
	entries, err := fs.ReadDir(fsys, "docs/specs")
	if err != nil {
		return definitions, []string{"docs/specs:1: read specification directories: " + err.Error()}
	}
	var folders []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
			folders = append(folders, entry.Name())
		}
	}
	if len(folders) != 1 {
		return definitions, []string{fmt.Sprintf("docs/specs:1: %s root docs' ids name no version; need exactly one version directory (found %d)", prefix, len(folders))}
	}
	folder := path.Join("docs/specs", folders[0])
	files, err := fs.Glob(fsys, folder+"/*.md")
	if err != nil {
		return definitions, []string{folder + ":1: list definitions: " + err.Error()}
	}
	for _, name := range files {
		data, readErr := fs.ReadFile(fsys, name)
		if readErr != nil {
			ps = append(ps, name+":1: read definitions: "+readErr.Error())
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

func repositoryPath(span string) bool {
	if !strings.Contains(span, "/") || strings.ContainsAny(span, "<>*{}") || strings.ContainsAny(span[:1], "$~/") || strings.IndexFunc(span, unicode.IsSpace) >= 0 {
		return false
	}
	first := strings.SplitN(span, "/", 2)[0]
	if strings.Contains(first, ".") && !strings.HasPrefix(first, ".") {
		return false
	}
	return true
}

func findRootPaths(text string) []citation {
	var found []citation
	fence := byte(0)
	fenceLength := 0
	offset := 0
	for _, line := range strings.SplitAfter(text, "\n") {
		trimmed := strings.TrimLeft(line, " ")
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
		} else if run >= 3 {
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
					break
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
	return found
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
		if family != "R" {
			families[family] = true
		}
	}
	req := make(map[string]int)
	data, err := fs.ReadFile(fsys, requirements)
	if err != nil {
		ps = append(ps, requirements+":1: read requirements: "+err.Error())
	} else {
		var problems []problem
		req, problems = parseDefinitions(string(data))
		for _, p := range problems {
			ps = append(ps, fmt.Sprintf("%s:%d: %s", requirements, p.line, p.message))
		}
	}
	for _, name := range rootDocuments {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			ps = append(ps, name+":1: read root doc: "+err.Error())
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
			ps = append(ps, fmt.Sprintf("%s:%d: %s", name, p.line, p.message))
		}
		for _, c := range findRootPaths(text) {
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
	fixture := fstest.MapFS{
		"docs/specs/conformance-collection-1.0/rules.md": {Data: []byte("- **QH8. text\n")},
		"docs/specs/judge-1.0/rules.md":                  {Data: []byte("- **E9a. title** text\n- **P3.** text\n- **R1. result**\n")},
		"docs/requirements.md":                           {Data: []byte("- R1. one\n- R2. two\n- R4. four\n- R42. last\n- AE36. last\n")},
		"docs/exists.md":                                 {Data: []byte("exists")},
	}
	for _, name := range rootDocuments {
		fixture[name] = &fstest.MapFile{Data: []byte("")}
	}
	fixture["AGENTS.md"].Data = []byte("QH8 E9a R42 AE36 R2 `docs/exists.md`\n")
	return fixture
}

func TestRootDocFixtures(t *testing.T) {
	tests := []struct {
		name   string
		change func(fstest.MapFS)
		want   string
	}{
		{"clean", func(fstest.MapFS) {}, ""},
		{"undefined_collection", func(f fstest.MapFS) { f["REVIEW.md"].Data = []byte("text\nQH99") }, "REVIEW.md:2: QH99 is not defined"},
		{"undefined_judge", func(f fstest.MapFS) { f["REVIEW.md"].Data = []byte("text\nE99") }, "REVIEW.md:2: E99 is not defined"},
		{"boundaries", func(f fstest.MapFS) { f["REVIEW.md"].Data = []byte("AQH8 QH8x A3 F2") }, ""},
		{"subfolder", func(f fstest.MapFS) {
			f["docs/specs/judge-1.0/cases/case-001/README.md"] = &fstest.MapFile{Data: []byte("- **P4.** text")}
			f["REVIEW.md"].Data = []byte("P4")
		}, "REVIEW.md:1: P4 is not defined"},
		{"line_anchor", func(f fstest.MapFS) {
			f["docs/specs/judge-1.0/rules.md"].Data = append(f["docs/specs/judge-1.0/rules.md"].Data, []byte("mid-line **P4.** example\n")...)
			f["REVIEW.md"].Data = []byte("P4")
		}, "REVIEW.md:1: P4 is not defined"},
		{"duplicate", func(f fstest.MapFS) {
			f["docs/specs/judge-1.0/second.md"] = &fstest.MapFile{Data: []byte("- **P3.** duplicate")}
		}, "docs/specs/judge-1.0/second.md:1: P3 is defined twice"},
		{"requirements", func(f fstest.MapFS) { f["REVIEW.md"].Data = []byte("R43 AE37") }, "REVIEW.md:1: R43 is not defined in docs/requirements.md"},
		{"range", func(f fstest.MapFS) { f["REVIEW.md"].Data = []byte("R1 to R4") }, "REVIEW.md:1: R3 is not defined in docs/requirements.md"},
		{"no_collection", func(f fstest.MapFS) { delete(f, "docs/specs/conformance-collection-1.0/rules.md") }, "conformance-collection- root docs' ids name no version; need exactly one version directory (found 0)"},
		{"two_collections", func(f fstest.MapFS) { f["docs/specs/conformance-collection-2.0/rules.md"] = &fstest.MapFile{} }, "conformance-collection- root docs' ids name no version; need exactly one version directory (found 2)"},
		{"no_judge", func(f fstest.MapFS) { delete(f, "docs/specs/judge-1.0/rules.md") }, "judge- root docs' ids name no version; need exactly one version directory (found 0)"},
		{"two_judges", func(f fstest.MapFS) { f["docs/specs/judge-2.0/rules.md"] = &fstest.MapFile{} }, "judge- root docs' ids name no version; need exactly one version directory (found 2)"},
		{"directory_lookup", func(f fstest.MapFS) {
			f["docs/specs/judge-1.0-errata.md"] = &fstest.MapFile{}
			f["docs/specs/conformance-collection-1.0-seal.md"] = &fstest.MapFile{}
		}, ""},
		{"directories", func(f fstest.MapFS) { f["REVIEW.md"].Data = []byte("`docs/specs` `docs/specs/`") }, ""},
		{"missing_path", func(f fstest.MapFS) { f["REVIEW.md"].Data = []byte("text\n`docs/missing.md`") }, "REVIEW.md:2: docs/missing.md repository path does not resolve"},
		{"host_skip", func(f fstest.MapFS) { f["REVIEW.md"].Data = []byte("`github.com/a/b`") }, ""},
		{"path_exclusions", func(f fstest.MapFS) {
			f["REVIEW.md"].Data = []byte("`$HOME/x/y` `~/x/y` `/abs/x` `docs/<name>/x` `docs/*/x` `docs/{x}/y` `docs/a b` `file.md`")
		}, ""},
		{"dot_directory", func(f fstest.MapFS) { f["REVIEW.md"].Data = []byte("`.github/workflows`") }, "REVIEW.md:1: .github/workflows repository path does not resolve"},
		{"fenced_skip", func(f fstest.MapFS) {
			f["REVIEW.md"].Data = []byte("```sh\n`docs/missing`\n```\n~~~\n`docs/also-missing`\n~~~\n")
		}, ""},
		{"missing_doc", func(f fstest.MapFS) { delete(f, "REVIEW.md") }, "REVIEW.md:1: read root doc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := rootFixture()
			tt.change(f)
			ps, counts := checkRootDocs(f)
			if tt.name == "boundaries" && counts.collection != 1 {
				t.Errorf("boundaries: collection citations = %d, want 1", counts.collection)
			}
			got := strings.Join(ps, "\n")
			if tt.want == "" {
				if len(ps) != 0 {
					t.Errorf("%s: unexpected problems: %s", tt.name, got)
				}
			} else if !strings.Contains(got, tt.want) {
				t.Errorf("%s: missing expected diagnostic %q; got %s", tt.name, tt.want, got)
			}
		})
	}
}

func TestRootDocFloors(t *testing.T) {
	for _, name := range []string{"collection id", "judge id", "requirement id", "path"} {
		t.Run(strings.ReplaceAll(name, " ", "_"), func(t *testing.T) {
			f := rootFixture()
			f["AGENTS.md"].Data = nil
			ps, _ := checkRootDocs(f)
			want := "no " + name + " references found"
			if !strings.Contains(strings.Join(ps, "\n"), want) {
				t.Errorf("%s: missing floor diagnostic %q; got %v", name, want, ps)
			}
		})
	}
}

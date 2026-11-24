package collectionkit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func writeCorpus(t *testing.T, root, name string, value any) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func scratchCorpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"collection-rules.md", "collection-contract.md", "credit-and-routes.md", "confinement.md"} {
		if err := os.WriteFile(filepath.Join(root, name), read(t, filepath.Join(kit, name)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeCorpus(t, root, "subjects/form/source.json", map[string]any{})
	invocation := map[string]any{"words": []string{"list"}, "offline_args": []string{}, "env": []string{}, "runbook": nil, "step": nil, "computation": nil, "home": "seed"}
	writeCorpus(t, root, "behaviour/form.json", map[string]any{"invocations": []any{map[string]any{"key": invocation, "confined": []any{map[string]any{"stdout": map[string]any{"head": "Available Commands:\n  list  List\n"}, "stderr": map[string]any{"head": ""}, "leak_scan": map[string]any{"leaks": []any{}}}}}}})
	writeCorpus(t, root, "cases/case-001/subject.json", map[string]any{"runbooks": []any{map[string]any{"steps": []any{map[string]any{"checks": []any{map[string]any{"predicate": "stdout_equals"}}}}}}})
	writeCorpus(t, root, "cases/case-001/input.json", map[string]any{"required": []any{map[string]any{"id": "a", "dimension": "bounding", "crediting": true}}})
	writeCorpus(t, root, "cases/case-001/case.json", map[string]any{"id": "case-001", "kind": "judge-checked", "rules": []string{"QP3"}, "covers": []string{"route:bounding"}, "named_check": "a", "twin": nil, "invocations": []any{invocation}, "input": map[string]any{"form": "form", "subject_file": "subject.json", "scope": []string{"conformance"}}, "expect": map[string]any{"judge_output": map[string]any{"checks": []any{map[string]any{"id": "a", "result": "failed"}}}}})
	return root
}

func mutateCorpus(t *testing.T, root, name string, mutate func(map[string]any)) {
	t.Helper()
	var value map[string]any
	decode(t, read(t, filepath.Join(root, name)), &value)
	mutate(value)
	writeCorpus(t, root, name, value)
}

func TestCorpusControls(t *testing.T) {
	controls := []struct {
		name, want string
		mutate     func(*testing.T, string)
	}{
		{"uncovered row", "uncovered tag route:availability", func(t *testing.T, root string) {
			if err := os.WriteFile(filepath.Join(root, "CASES.md"), []byte("Corpus: complete\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{"one credit side", "uncovered tag credit:token-economy:fails", func(t *testing.T, root string) {
			mutateCorpus(t, root, "cases/case-001/input.json", func(v map[string]any) { corpusMap(corpusList(v["required"])[0])["dimension"] = "token-economy" })
			mutateCorpus(t, root, "cases/case-001/case.json", func(v map[string]any) { v["covers"] = []string{"credit:token-economy:holds"} })
			if err := os.WriteFile(filepath.Join(root, "CASES.md"), []byte("Corpus: complete\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{"unrecorded invocation", "unrecorded invocation", func(t *testing.T, root string) {
			mutateCorpus(t, root, "cases/case-001/case.json", func(v map[string]any) { v["invocations"] = []any{map[string]any{"words": []string{"absent"}}} })
		}},
		{"two red checks", "red on named check only", func(t *testing.T, root string) {
			mutateCorpus(t, root, "cases/case-001/case.json", func(v map[string]any) {
				corpusMap(corpusMap(v["expect"])["judge_output"])["checks"] = []any{map[string]any{"id": "a", "result": "failed"}, map[string]any{"id": "b", "result": "failed"}}
			})
		}},
		{"two twin inputs", "twin differs in 2 inputs", func(t *testing.T, root string) {
			var twin map[string]any
			decode(t, read(t, filepath.Join(root, "cases/case-001/case.json")), &twin)
			twin["id"] = "case-002"
			twin["named_check"] = nil
			corpusMap(twin["input"])["request"] = "anchor"
			corpusMap(twin["input"])["scope"] = []string{"destructive"}
			writeCorpus(t, root, "cases/case-002/case.json", twin)
			writeCorpus(t, root, "cases/case-002/subject.json", map[string]any{"runbooks": []any{map[string]any{"steps": []any{map[string]any{"checks": []any{map[string]any{"predicate": "stdout_equals"}}}}}}})
			writeCorpus(t, root, "cases/case-002/input.json", map[string]any{"required": []any{map[string]any{"id": "a", "dimension": "bounding", "crediting": true}}})
			mutateCorpus(t, root, "cases/case-001/case.json", func(v map[string]any) { v["twin"] = "case-002" })
		}},
		{"false predicate tag", "predicate tag absent from subject", func(t *testing.T, root string) {
			mutateCorpus(t, root, "cases/case-001/case.json", func(v map[string]any) { v["covers"] = []string{"predicate:row_count"} })
		}},
	}
	for _, control := range controls {
		t.Run(control.name, func(t *testing.T) {
			root := scratchCorpus(t)
			if failures, _ := checkCorpus(root); len(failures) != 0 {
				t.Fatal(failures)
			}
			control.mutate(t, root)
			failures, _ := checkCorpus(root)
			joined := strings.Join(failures, "\n")
			if !strings.Contains(joined, control.want) {
				t.Fatalf("want %q, got %s", control.want, joined)
			}
			t.Log(control.want)
		})
	}
}

func TestCollectionCorpus(t *testing.T) {
	failures, logs := checkCorpus(kit)
	for _, message := range logs {
		t.Log(message)
	}
	for _, message := range failures {
		t.Error(message)
	}
}

// Corpus checks return diagnostics so the same checks can inspect scratch corpora.
type corpusCase struct {
	ID          string         `json:"id"`
	Kind        string         `json:"kind"`
	Rules       []string       `json:"rules"`
	Covers      []string       `json:"covers"`
	Named       *string        `json:"named_check"`
	Twin        *string        `json:"twin"`
	Input       map[string]any `json:"input"`
	Invocations []any          `json:"invocations"`
	Expect      struct {
		Output struct {
			Checks []struct {
				ID     string `json:"id"`
				Result string `json:"result"`
			} `json:"checks"`
		} `json:"judge_output"`
	} `json:"expect"`
	subject   []byte
	required  []corpusRequired
	preimages map[string]corpusSelector
}

// Only these preimage fields identify a counterpart; repaired inputs may change its id.
type corpusSelector struct {
	Probe     *string  `json:"probe"`
	Runbook   *string  `json:"runbook"`
	Step      *string  `json:"step"`
	Index     *int     `json:"index"`
	Dimension string   `json:"dimension"`
	Words     []string `json:"words"`
}

type corpusRequired struct {
	ID        string `json:"id"`
	Dimension string `json:"dimension"`
	Crediting bool   `json:"crediting"`
}

func corpusJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
func corpusEqual(a, b any) bool { return reflect.DeepEqual(a, b) }
func corpusContains(value any, key, wanted string) bool {
	switch node := value.(type) {
	case map[string]any:
		if node[key] == wanted {
			return true
		}
		for _, child := range node {
			if corpusContains(child, key, wanted) {
				return true
			}
		}
	case []any:
		for _, child := range node {
			if corpusContains(child, key, wanted) {
				return true
			}
		}
	}
	return false
}
func corpusSection(text, heading string) string {
	_, rest, ok := strings.Cut(text, heading)
	if !ok {
		return ""
	}
	section, _, _ := strings.Cut(rest, "\n## ")
	return section
}
func corpusRows(text string) [][]string {
	var rows [][]string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		parts := strings.Split(strings.Trim(line, "|"), "|")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		rows = append(rows, parts)
	}
	return rows
}
func corpusTags(routes, rules, contract string) map[string]bool {
	tags := map[string]bool{}
	for _, row := range corpusRows(corpusSection(routes, "## 2.")) {
		dimension := strings.Trim(row[0], "`")
		tags["route:"+dimension] = true
		if len(row) > 3 && row[3] != "none" && !strings.HasPrefix(row[3], "never") {
			tags["credit:"+dimension+":holds"] = true
			tags["credit:"+dimension+":fails"] = true
		}
	}
	for _, row := range corpusRows(corpusSection(routes, "## 4.")) {
		tags["encoding:"+strings.Trim(row[0], "`")] = true
	}
	for _, line := range strings.Split(corpusSection(routes, "## 5."), "\n") {
		if strings.HasPrefix(line, "| Cobra |") {
			tags["help:cobra"] = true
		}
		if strings.HasPrefix(line, "| urfave/cli |") {
			tags["help:urfave-cli"] = true
		}
		if strings.HasPrefix(line, "| clap |") {
			tags["help:clap"] = true
		}
	}
	for _, row := range corpusRows(corpusSection(rules, "- **QA9.")) {
		tags["predicate:"+strings.Trim(row[0], "`")] = true
	}
	for _, row := range strings.Split(corpusSection(contract, "- **QT6."), "\n") {
		if !strings.Contains(row, "| `order:<earlier>><later>` |") {
			continue
		}
		for _, pair := range regexp.MustCompile("`(Q[A-Z][0-9]+>Q[A-Z][0-9]+)`").FindAllStringSubmatch(row, -1) {
			tags["order:"+pair[1]] = true
		}
	}
	return tags
}
func corpusRuleConcerns(rule string, probe *string) (bool, bool) {
	if strings.HasPrefix(rule, "QP") {
		number, err := strconv.Atoi(strings.TrimPrefix(rule, "QP"))
		if err != nil {
			return false, false
		}
		if probe == nil {
			return false, true
		}
		switch {
		case number == 1 || number == 5:
			return true, true
		case number == 2:
			return *probe == "bare", true
		case number == 3:
			return *probe == "byte_bound", true
		case number == 4:
			return *probe == "exit_codes", true
		case number >= 6 && number <= 9:
			return *probe == "store_unchanged", true
		case number >= 10 && number <= 14:
			return *probe == "planted_secret", true
		}
		return false, false
	}
	if strings.HasPrefix(rule, "QD") {
		return probe != nil && (*probe == "discovery" || rule == "QD4"), true
	}
	if strings.HasPrefix(rule, "QA") {
		return probe == nil, true
	}
	for _, family := range []string{"QS", "QO", "QH", "QE", "QT"} {
		if strings.HasPrefix(rule, family) {
			return true, true
		}
	}
	return false, false
}
func corpusHelp(recording map[string]any, generator string) bool {
	entries := corpusList(recording["invocations"])
	for _, entry := range entries {
		inv := corpusMap(entry)
		key := corpusMap(inv["key"])
		words := corpusList(key["words"])
		if len(words) == 0 || words[len(words)-1] != "--help" {
			continue
		}
		runs := corpusList(inv["confined"])
		for _, item := range runs {
			run := corpusMap(item)
			for _, name := range []string{"stdout", "stderr"} {
				stream := corpusMap(run[name])
				head := corpusString(stream["head"])
				for _, line := range strings.Split(head, "\n") {
					if strings.HasPrefix(line, " ") {
						continue
					}
					if (generator == "cobra" && strings.HasSuffix(line, " Commands:")) || (generator == "urfave-cli" && line == "COMMANDS:") || (generator == "clap" && (line == "Commands:" || line == "SUBCOMMANDS:")) {
						return true
					}
				}
			}
		}
	}
	return false
}

func checkCorpus(root string) ([]string, []string) {
	var failures, logs []string
	fail := func(format string, args ...any) { failures = append(failures, fmt.Sprintf(format, args...)) }
	texts := map[string]string{}
	defined := map[string]bool{}
	pattern := regexp.MustCompile(`\*\*(Q[A-Z][0-9]+)\.`)
	for _, name := range []string{"collection-contract.md", "collection-rules.md", "credit-and-routes.md", "confinement.md"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			fail("read %s: %v", name, err)
			continue
		}
		texts[name] = string(data)
		for _, match := range pattern.FindAllStringSubmatch(string(data), -1) {
			defined[match[1]] = true
		}
	}
	expected := corpusTags(texts["credit-and-routes.md"], texts["collection-rules.md"], texts["collection-contract.md"])
	complete := false
	notExpressible := map[string]bool{}
	entryPattern := regexp.MustCompile(`^- (Q[A-Z][0-9]+): (.*\S.*)$`)
	if data, err := os.ReadFile(filepath.Join(root, "CASES.md")); err == nil {
		inSection := false
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "## ") {
				inSection = line == "## Not expressible as a case"
				continue
			}
			if inSection && strings.HasPrefix(line, "- ") {
				match := entryPattern.FindStringSubmatch(line)
				if match == nil {
					fail("CASES.md: malformed entry %s", line)
				} else {
					rule := match[1]
					if notExpressible[rule] {
						fail("CASES.md: %s listed twice", rule)
					} else {
						notExpressible[rule] = true
						logs = append(logs, fmt.Sprintf("%s: not expressible as a case", rule))
						if !defined[rule] {
							fail("CASES.md: %s is not a defined rule", rule)
						}
					}
				}
			}
			if line == "Corpus: complete" {
				complete = true
			}
		}
	} else if !os.IsNotExist(err) {
		fail("read CASES.md: %v", err)
	}
	cases := map[string]*corpusCase{}
	directories := map[string]string{}
	entries, err := os.ReadDir(filepath.Join(root, "cases"))
	if err != nil && !os.IsNotExist(err) {
		fail("read cases: %v", err)
	}
	for _, entry := range entries {
		path := filepath.Join(root, "cases", entry.Name())
		var c corpusCase
		if !entry.IsDir() {
			fail("unexpected case file %s", entry.Name())
			continue
		}
		if err := corpusJSON(filepath.Join(path, "case.json"), &c); err != nil {
			fail("parse %s: %v", entry.Name(), err)
			continue
		}
		if c.ID == "" || cases[c.ID] != nil {
			fail("duplicate or missing case id %s", c.ID)
			continue
		}
		cases[c.ID] = &c
		directories[c.ID] = path
	}
	covered := map[string]int{}
	judged := map[string]int{}
	tags := map[string]bool{}
	for _, id := range sortedCorpusKeys(cases) {
		c := cases[id]
		path := directories[id]
		for _, rule := range c.Rules {
			if notExpressible[rule] && covered[rule] == 0 {
				fail("CASES.md: %s is listed and covered by %s", rule, id)
			}
			covered[rule]++
			if c.Kind == "judge-checked" {
				judged[rule]++
			}
			if !defined[rule] {
				fail("%s: undefined rule %s", id, rule)
			}
		}
		form := corpusString(c.Input["form"])
		subjectName := corpusString(c.Input["subject_file"])
		if info, err := os.Stat(filepath.Join(root, "subjects", form)); err != nil || !info.IsDir() {
			fail("%s: missing subject form %s", id, form)
		}
		var recording map[string]any
		if err := corpusJSON(filepath.Join(root, "behaviour", form+".json"), &recording); err != nil {
			fail("%s: recording: %v", id, err)
		}
		invocations := corpusList(recording["invocations"])
		for _, key := range c.Invocations {
			found := false
			for _, entry := range invocations {
				inv := corpusMap(entry)
				if corpusEqual(key, inv["key"]) {
					found = true
				}
			}
			if !found {
				fail("%s: unrecorded invocation %v", id, key)
			}
		}
		subject, err := os.ReadFile(filepath.Join(path, subjectName))
		if err != nil {
			fail("%s: subject: %v", id, err)
		}
		c.subject = subject
		var subjectValue any
		if err := json.Unmarshal(subject, &subjectValue); err != nil {
			fail("%s: subject JSON: %v", id, err)
		}
		if c.Kind == "judge-checked" {
			var input struct {
				Required []corpusRequired `json:"required"`
			}
			if err := corpusJSON(filepath.Join(path, "input.json"), &input); err != nil {
				fail("%s: input: %v", id, err)
			}
			c.required = input.Required
		}
		for _, tag := range c.Covers {
			tags[tag] = true
			if !expected[tag] {
				fail("%s: unknown covers tag %s", id, tag)
				continue
			}
			parts := strings.Split(tag, ":")
			switch parts[0] {
			case "predicate":
				if !corpusContains(subjectValue, "predicate", parts[1]) {
					fail("%s: predicate tag absent from subject: %s", id, tag)
				}
			case "encoding":
				if !corpusContains(recording, "encoding", parts[1]) {
					fail("%s: encoding tag absent from recording: %s", id, tag)
				}
			case "help":
				if !corpusHelp(recording, parts[1]) {
					fail("%s: help tag absent from recording: %s", id, tag)
				}
			case "route", "credit":
				found := false
				for _, check := range c.required {
					if check.Dimension == parts[1] && (parts[0] == "route" || check.Crediting == (parts[2] == "holds")) {
						found = true
					}
				}
				if !found {
					fail("%s: tag absent from required: %s", id, tag)
				}
			case "order":
			}
		}
		if c.Named != nil {
			red := 0
			namedRed := false
			for _, check := range c.Expect.Output.Checks {
				if check.Result == "failed" || check.Result == "failed_expected" {
					red++
					if check.ID == *c.Named {
						namedRed = true
					}
				}
			}
			if red != 1 || !namedRed {
				fail("%s: must be red on named check only", id)
			}
		}
	}
	for _, id := range sortedCorpusKeys(cases) {
		c := cases[id]
		if c.Kind != "judge-checked" {
			continue
		}
		var twin *corpusCase
		if c.Twin != nil {
			twin = cases[*c.Twin]
			if twin == nil {
				fail("%s: missing twin %s", id, *c.Twin)
			} else {
				differences := 0
				fields := map[string]bool{}
				for name := range c.Input {
					fields[name] = true
				}
				for name := range twin.Input {
					fields[name] = true
				}
				for name := range fields {
					if !corpusEqual(c.Input[name], twin.Input[name]) {
						differences++
					}
				}
				if !bytes.Equal(c.subject, twin.subject) {
					differences++
				}
				if differences != 1 {
					fail("%s: twin differs in %d inputs", id, differences)
				}
			}
		}
		var unobserved []string
		for _, check := range c.Expect.Output.Checks {
			if check.Result != "unobserved" {
				continue
			}
			concerns := c.Named != nil && *c.Named == check.ID
			if !concerns && c.Named == nil {
				if c.preimages == nil {
					if err := corpusJSON(filepath.Join(directories[id], "preimages.json"), &c.preimages); err != nil {
						fail("%s: unclear not-measured check %s: %v", id, check.ID, err)
						continue
					}
				}
				preimage, ok := c.preimages[check.ID]
				if !ok {
					fail("%s: unclear not-measured check %s: missing preimage", id, check.ID)
					continue
				}
				for _, rule := range c.Rules {
					relevant, known := corpusRuleConcerns(rule, preimage.Probe)
					if !known {
						fail("%s: unclear checks concerned by %s", id, rule)
					}
					concerns = concerns || relevant
				}
			}
			if concerns {
				unobserved = append(unobserved, check.ID)
			}
		}
		for _, checkID := range unobserved {
			held, cycled := false, false
			visited := map[string]bool{id: true}
			candidate := c
			if candidate.preimages == nil {
				if err := corpusJSON(filepath.Join(directories[candidate.ID], "preimages.json"), &candidate.preimages); err != nil {
					fail("%s: counterpart preimages: %v", candidate.ID, err)
				}
			}
			selector, present := c.preimages[checkID]
			next := c.Twin
			for next != nil {
				if visited[*next] {
					fail("%s: twin chain cycles at %s", id, *next)
					cycled = true
					break
				}
				visited[*next] = true
				candidate = cases[*next]
				if candidate == nil {
					break
				}
				if candidate.preimages == nil {
					if err := corpusJSON(filepath.Join(directories[candidate.ID], "preimages.json"), &candidate.preimages); err != nil {
						fail("%s: counterpart preimages: %v", candidate.ID, err)
					}
				}
				result := ""
				neverCredits := false
				for _, check := range candidate.Expect.Output.Checks {
					counterpart, exists := candidate.preimages[check.ID]
					if present && exists && corpusEqual(selector, counterpart) {
						result = check.Result
						for _, required := range candidate.required {
							if required.ID == check.ID && !required.Crediting {
								neverCredits = true
							}
						}
						break
					}
				}
				if result == "held_crediting" || (result == "held_non_crediting" && neverCredits) {
					held = true
					break
				}
				if result != "unobserved" {
					break
				}
				next = candidate.Twin
			}
			if !held && !cycled {
				fail("%s: twin counterpart %s must be held and crediting", id, checkID)
			}
		}
	}
	for _, rule := range sortedCorpusKeys(defined) {
		logs = append(logs, fmt.Sprintf("%s: %d cases; %d judge-checked", rule, covered[rule], judged[rule]))
		if covered[rule] == 0 && !notExpressible[rule] {
			message := "uncovered rule " + rule
			if complete {
				fail("%s", message)
			} else {
				logs = append(logs, message)
			}
		}
	}
	for _, tag := range sortedCorpusKeys(expected) {
		if !tags[tag] {
			message := "uncovered tag " + tag
			if complete {
				fail("%s", message)
			} else {
				logs = append(logs, message)
			}
		}
	}
	return failures, logs
}
func sortedCorpusKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func corpusMap(value any) map[string]any {
	switch node := value.(type) {
	case map[string]any:
		return node
	default:
		return nil
	}
}
func corpusList(value any) []any {
	switch node := value.(type) {
	case []any:
		return node
	default:
		return nil
	}
}
func corpusString(value any) string {
	switch node := value.(type) {
	case string:
		return node
	default:
		return ""
	}
}

func TestCorpusTwinSelectors(t *testing.T) {
	for _, result := range []string{"held_crediting", "held_non_crediting", "unobserved", "failed"} {
		t.Run(result, func(t *testing.T) {
			root := scratchCorpus(t)
			var twin map[string]any
			decode(t, read(t, filepath.Join(root, "cases/case-001/case.json")), &twin)
			twin["id"] = "case-002"
			twin["named_check"] = nil
			corpusMap(twin["input"])["request"] = "anchor"
			corpusMap(corpusMap(twin["expect"])["judge_output"])["checks"] = []any{map[string]any{"id": "b", "result": result}}
			writeCorpus(t, root, "cases/case-002/case.json", twin)
			for _, name := range []string{"subject.json", "input.json"} {
				var value any
				decode(t, read(t, filepath.Join(root, "cases/case-001", name)), &value)
				writeCorpus(t, root, "cases/case-002/"+name, value)
			}
			selector := map[string]any{"probe": "byte_bound", "runbook": nil, "step": nil, "index": nil, "dimension": "bounding", "words": []string{"list"}}
			writeCorpus(t, root, "cases/case-001/preimages.json", map[string]any{"a": selector})
			writeCorpus(t, root, "cases/case-002/preimages.json", map[string]any{"b": selector})
			mutateCorpus(t, root, "cases/case-001/case.json", func(v map[string]any) {
				v["named_check"] = nil
				v["twin"] = "case-002"
				corpusMap(corpusMap(v["expect"])["judge_output"])["checks"] = []any{map[string]any{"id": "a", "result": "unobserved"}}
			})
			failures, _ := checkCorpus(root)
			if result == "held_crediting" && len(failures) != 0 {
				t.Fatal(failures)
			}
			if result != "held_crediting" && !strings.Contains(strings.Join(failures, "\n"), "twin counterpart a must be held and crediting") {
				t.Fatal(failures)
			}
		})
	}
}

func TestCorpusOrderTags(t *testing.T) {
	tags := corpusTags("", "", string(read(t, filepath.Join(kit, "collection-contract.md"))))
	want := map[string]bool{}
	for _, pair := range []string{"QO1>QS5", "QO1>QA7", "QS5>QD4", "QS5>QA7", "QA7>QH8", "QH8>QA6"} {
		want["order:"+pair] = true
	}
	if !corpusEqual(tags, want) {
		t.Fatalf("order tags = %v; want %v", tags, want)
	}
}

func TestCorpusTwinChains(t *testing.T) {
	for _, test := range []struct {
		name, last, want string
		cycle            bool
		neverCredits     bool
	}{
		{"repaired", "held_crediting", "", false, false},
		{"non crediting", "held_non_crediting", "case-001: twin counterpart a must be held and crediting", false, false},
		{"never credits", "held_non_crediting", "", false, true},
		{"ended", "unobserved", "case-003: twin counterpart c must be held and crediting", false, false},
		{"cycle", "unobserved", "case-001: twin chain cycles at case-001", true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := scratchCorpus(t)
			count := 3
			if test.cycle {
				count = 2
			}
			for i := 1; i <= count; i++ {
				var c map[string]any
				decode(t, read(t, filepath.Join(root, "cases/case-001/case.json")), &c)
				id := fmt.Sprintf("case-%03d", i)
				check := string(rune('a' + i - 1))
				c["id"], c["named_check"], c["twin"] = id, nil, nil
				c["input"] = map[string]any{"form": "form", "subject_file": "subject.json", "request": strconv.Itoa(i)}
				if i < count {
					c["twin"] = fmt.Sprintf("case-%03d", i+1)
				}
				if test.cycle && i == count {
					c["twin"] = "case-001"
				}
				result := "unobserved"
				if i == count {
					result = test.last
				}
				corpusMap(corpusMap(c["expect"])["judge_output"])["checks"] = []any{map[string]any{"id": check, "result": result}}
				writeCorpus(t, root, "cases/"+id+"/case.json", c)
				if i > 1 {
					for _, name := range []string{"subject.json", "input.json"} {
						var v any
						decode(t, read(t, filepath.Join(root, "cases/case-001", name)), &v)
						writeCorpus(t, root, "cases/"+id+"/"+name, v)
					}
				}
				writeCorpus(t, root, "cases/"+id+"/input.json", map[string]any{"required": []any{map[string]any{"id": check, "dimension": "bounding", "crediting": !(test.neverCredits && i == count)}}})
				writeCorpus(t, root, "cases/"+id+"/preimages.json", map[string]any{check: map[string]any{"probe": "byte_bound", "dimension": "bounding", "words": []string{"list"}}})
			}
			failures, _ := checkCorpus(root)
			if test.want == "" {
				if len(failures) != 0 {
					t.Fatal(failures)
				}
				return
			}
			if !strings.Contains(strings.Join(failures, "\n"), test.want) {
				t.Fatalf("want %q, got %v", test.want, failures)
			}
		})
	}
}

func TestCorpusNotExpressible(t *testing.T) {
	for _, test := range []struct{ name, entry, want string }{
		{"closes gap", "- QH3: Requires host state", ""},
		{"covered", "- QP3: Requires host state", "CASES.md: QP3 is listed and covered by case-001"},
		{"undefined", "- QH999: Requires host state", "CASES.md: QH999 is not a defined rule"},
		{"empty reason", "- QH3: ", "CASES.md: malformed entry - QH3: "},
		{"duplicate", "- QH3: Host state\n- QH3: Host state", "CASES.md: QH3 listed twice"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := scratchCorpus(t)
			put := func(body string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(root, "CASES.md"), []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			put("Corpus: complete\n")
			failures, _ := checkCorpus(root)
			if !strings.Contains(strings.Join(failures, "\n"), "uncovered rule QH3") {
				t.Fatal(failures)
			}
			put("Corpus: complete\n## Not expressible as a case\n" + test.entry + "\n## Other\n- invalid outside section\n")
			failures, logs := checkCorpus(root)
			joined := strings.Join(failures, "\n")
			if test.want != "" {
				if !strings.Contains(joined, test.want) {
					t.Fatalf("want %q, got %s", test.want, joined)
				}
				return
			}
			if strings.Contains(joined, "uncovered rule QH3") {
				t.Fatal("listed rule remains uncovered")
			}
			if !strings.Contains(strings.Join(logs, "\n"), "QH3: not expressible as a case") {
				t.Fatal(logs)
			}
		})
	}
}

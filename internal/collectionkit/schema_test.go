package collectionkit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/judge"
)

const kit = "../../docs/specs/conformance-collection-1.0"
const sealed = "../../docs/specs/judge-1.0"

func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func decode(t *testing.T, b []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}

// enumSites checks references at their use sites as well as definitions.
func enumSites(t *testing.T, root any, v any, path string, found map[string][]any, refs map[string]bool) {
	t.Helper()
	switch node := v.(type) {
	case map[string]any:
		if ref, ok := node["$ref"].(string); ok && strings.HasPrefix(ref, "#/") {
			if refs[ref] {
				t.Fatalf("cyclic reference %s", ref)
			}
			target := root
			for _, part := range strings.Split(ref[2:], "/") {
				object, ok := target.(map[string]any)
				if !ok {
					t.Fatalf("invalid reference %s", ref)
				}
				target, ok = object[strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")]
				if !ok {
					t.Fatalf("missing reference %s", ref)
				}
			}
			next := map[string]bool{}
			for k, v := range refs {
				next[k] = v
			}
			next[ref] = true
			enumSites(t, root, target, path, found, next)
		}
		if values, ok := node["enum"].([]any); ok {
			found[path] = values
		}
		if value, ok := node["const"]; ok {
			found[path] = append(found[path], value)
		}
		for key, child := range node {
			enumSites(t, root, child, path+"/"+key, found, refs)
		}
	case []any:
		for i, child := range node {
			enumSites(t, root, child, fmt.Sprintf("%s/%d", path, i), found, refs)
		}
	}
}

func TestJudgeSchemaCopies(t *testing.T) {
	for _, name := range []string{"judge-input.schema.json", "judge-output.schema.json"} {
		t.Run(strings.ReplaceAll(name, ".", "_"), func(t *testing.T) {
			if !bytes.Equal(read(t, filepath.Join(kit, "schema", name)), read(t, filepath.Join(sealed, "schema", name))) {
				t.Fatal("schema differs from sealed judge")
			}
		})
	}
}
func TestSchemaEnums(t *testing.T) {
	type policy struct{ oracle, site, owned string }
	policies := map[string]policy{
		"baseline.schema.json/properties/baseline/properties/entries/items/properties/dimension":                                 {oracle: "judge-input.schema.json", site: "/properties/required/items/properties/dimension"},
		"baseline.schema.json/properties/baseline/properties/entries/items/properties/kind":                                      {oracle: "judge-input.schema.json", site: "/properties/required/items/properties/kind"},
		"baseline.schema.json/properties/baseline/properties/entries/items/properties/result":                                    {oracle: "judge-input.schema.json", site: "/properties/baseline/oneOf/1/properties/entries/items/properties/result"},
		"baseline.schema.json/properties/baseline/properties/entries/items/properties/tier":                                      {oracle: "judge-input.schema.json", site: "/properties/evidence/items/properties/tier"},
		"baseline.schema.json/properties/baseline_version":                                                                       {owned: "[\"1.0\"]"},
		"baseline.schema.json/properties/preimages/additionalProperties/properties/parameters/anyOf/4/properties/kind":           {owned: "[\"credential\",\"seeded_state\"]"},
		"baseline.schema.json/properties/preimages/additionalProperties/properties/probe":                                        {owned: "[\"discovery\",\"bare\",\"byte_bound\",\"exit_codes\",\"planted_secret\",\"store_unchanged\",null]"},
		"baseline.schema.json/properties/preimages/additionalProperties/properties/scheme":                                       {owned: "[\"pql-check-id-1\"]"},
		"baseline.schema.json/properties/spec_version":                                                                           {owned: "[\"1.0\"]"},
		"case.schema.json/properties/entry/properties/fixtures/items/properties/kind":                                            {owned: "[\"link\",\"special_file\",\"oversized_output\",\"invalid_bytes\",\"planted_secret\"]"},
		"case.schema.json/properties/entry/properties/request":                                                                   {oracle: "judge-input.schema.json", site: "/properties/request"},
		"case.schema.json/properties/entry/properties/scope/items":                                                               {oracle: "judge-output.schema.json", site: "/properties/scope/items"},
		"case.schema.json/properties/expect/properties/exit_code":                                                                {owned: "[0,1,2,3,4,5,64,70]"},
		"case.schema.json/properties/kind":                                                                                       {owned: "[\"judge-checked\",\"derivation-only\"]"},
		"record.schema.json/properties/exit_code":                                                                                {owned: "[0,1,2,3,4,5,64,70]"},
		"record.schema.json/properties/next_actions/items/oneOf/0/properties/reason":                                             {oracle: "judge-output.schema.json", site: "/properties/checks/items/properties/reason"},
		"record.schema.json/properties/next_actions/items/oneOf/1/properties/dimension":                                          {oracle: "judge-input.schema.json", site: "/properties/required/items/properties/dimension"},
		"record.schema.json/properties/next_actions/items/oneOf/1/properties/reason":                                             {oracle: "judge-output.schema.json", site: "/properties/dimensions/items/properties/reason"},
		"record.schema.json/properties/record_version":                                                                           {owned: "[\"1.0\"]"},
		"record.schema.json/properties/request":                                                                                  {oracle: "judge-input.schema.json", site: "/properties/request"},
		"record.schema.json/properties/scope/items":                                                                              {oracle: "judge-output.schema.json", site: "/properties/scope/items"},
		"record.schema.json/properties/spec_version":                                                                             {owned: "[\"1.0\"]"},
		"subject-file.schema.json/$defs/check/properties/dimension":                                                              {oracle: "judge-input.schema.json", site: "/properties/required/items/properties/dimension"},
		"subject-file.schema.json/$defs/check/properties/predicate":                                                              {owned: "[\"exit_code\",\"stdout_equals\",\"stdout_json_equals\",\"stdout_contains\",\"stdout_not_contains\",\"stderr_contains\",\"stdout_bytes_at_most\",\"json_path_equals\",\"row_count\",\"stdout_empty\"]"},
		"subject-file.schema.json/$defs/command/properties/class/oneOf/0":                                                        {owned: "[\"safe\",\"destructive\"]"},
		"subject-file.schema.json/$defs/dimension":                                                                               {owned: "[\"availability\",\"execution\",\"graceful-failure\",\"robustness\",\"response-correctness\",\"determinism\",\"provenance-honesty\",\"completeness-signaling\",\"output-conformance\",\"flag-conformance\",\"bounding\",\"discoverability\",\"error-message-quality\",\"token-economy\",\"side-effect-integrity\",\"idempotency\",\"destructive-guard\",\"pii-secret-hygiene\"]"},
		"subject-file.schema.json/$defs/runbook/properties/steps/items/properties/checks/items/properties/dimension":             {oracle: "judge-input.schema.json", site: "/properties/required/items/properties/dimension"},
		"subject-file.schema.json/$defs/runbook/properties/steps/items/properties/checks/items/properties/predicate":             {owned: "[\"exit_code\",\"stdout_equals\",\"stdout_json_equals\",\"stdout_contains\",\"stdout_not_contains\",\"stderr_contains\",\"stdout_bytes_at_most\",\"json_path_equals\",\"row_count\",\"stdout_empty\"]"},
		"subject-file.schema.json/$defs/step/properties/checks/items/properties/dimension":                                       {oracle: "judge-input.schema.json", site: "/properties/required/items/properties/dimension"},
		"subject-file.schema.json/$defs/step/properties/checks/items/properties/predicate":                                       {owned: "[\"exit_code\",\"stdout_equals\",\"stdout_json_equals\",\"stdout_contains\",\"stdout_not_contains\",\"stderr_contains\",\"stdout_bytes_at_most\",\"json_path_equals\",\"row_count\",\"stdout_empty\"]"},
		"subject-file.schema.json/properties/commands/items/properties/class/oneOf/0":                                            {owned: "[\"safe\",\"destructive\"]"},
		"subject-file.schema.json/properties/facts/properties/bounding/oneOf/0":                                                  {oracle: "judge-input.schema.json", site: "/properties/subject_facts/properties/bounding"},
		"subject-file.schema.json/properties/markers/items/properties/dimension":                                                 {oracle: "judge-input.schema.json", site: "/properties/required/items/properties/dimension"},
		"subject-file.schema.json/properties/runbooks/items/properties/steps/items/properties/checks/items/properties/dimension": {oracle: "judge-input.schema.json", site: "/properties/required/items/properties/dimension"},
		"subject-file.schema.json/properties/runbooks/items/properties/steps/items/properties/checks/items/properties/predicate": {owned: "[\"exit_code\",\"stdout_equals\",\"stdout_json_equals\",\"stdout_contains\",\"stdout_not_contains\",\"stderr_contains\",\"stdout_bytes_at_most\",\"json_path_equals\",\"row_count\",\"stdout_empty\"]"},
		"subject-file.schema.json/properties/secret/oneOf/2/if/properties/kind":                                                  {owned: "[\"seeded_state\"]"},
		"subject-file.schema.json/properties/secret/oneOf/2/properties/kind":                                                     {owned: "[\"credential\",\"seeded_state\"]"},
		"subject-file.schema.json/properties/subject_file_version":                                                               {owned: "[\"1.0\"]"},
	}
	oracles := map[string]map[string][]any{}
	for _, name := range []string{"judge-input.schema.json", "judge-output.schema.json"} {
		var v any
		decode(t, read(t, filepath.Join(sealed, "schema", name)), &v)
		sites := map[string][]any{}
		enumSites(t, v, v, "", sites, nil)
		oracles[name] = sites
	}
	files, err := filepath.Glob(filepath.Join(kit, "schema", "*.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no schemas")
	}
	seen := map[string]bool{}
	for _, file := range files {
		var v any
		decode(t, read(t, file), &v)
		sites := map[string][]any{}
		enumSites(t, v, v, "", sites, nil)
		name := filepath.Base(file)
		for site, values := range sites {
			key := name + site
			seen[key] = true
			rule, ok := policies[key]
			if oracles[name] != nil {
				rule = policy{oracle: name, site: site}
				ok = true
			}
			if !ok {
				t.Errorf("unmapped enum site %s", key)
				continue
			}
			if rule.owned != "" {
				var expected []any
				decode(t, []byte(rule.owned), &expected)
				if !reflect.DeepEqual(values, expected) {
					t.Errorf("%s kit-owned values differ: %v", key, values)
				}
				continue
			}
			expected := oracles[rule.oracle][rule.site]
			if len(expected) == 0 {
				t.Errorf("missing judge oracle %s %s", rule.oracle, rule.site)
			}
			for _, value := range values {
				if !slices.ContainsFunc(expected, func(want any) bool { return reflect.DeepEqual(want, value) }) {
					t.Errorf("%s enum value %v absent from sealed judge site", key, value)
				}
			}
		}
	}
	for key := range policies {
		if !seen[key] {
			t.Errorf("enum policy path no longer resolves: %s", key)
		}
	}
}
func TestExampleRecords(t *testing.T) {
	var subject struct {
		Commands []struct {
			Words []string `json:"words"`
			Bound int      `json:"byte_bound"`
			Codes []int    `json:"exit_codes"`
		} `json:"commands"`
	}
	decode(t, read(t, filepath.Join(kit, "examples", "subject-file.json")), &subject)
	if len(subject.Commands) != 1 || !reflect.DeepEqual(subject.Commands[0].Words, []string{"list"}) || subject.Commands[0].Bound == 0 || len(subject.Commands[0].Codes) == 0 {
		t.Fatal("example must declare both probes on list")
	}

	files, err := filepath.Glob(filepath.Join(kit, "examples", "record-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no records")
	}
	for _, file := range files {
		t.Run(strings.ReplaceAll(filepath.Base(file), ".", "_"), func(t *testing.T) {
			var record struct {
				Input    json.RawMessage `json:"judge_input"`
				Output   any             `json:"judge_output"`
				ExitCode int             `json:"exit_code"`
				Rejected bool            `json:"judge_rejected"`
			}
			decode(t, read(t, file), &record)
			output, code := judge.Judge(record.Input)
			var got map[string]any
			decode(t, output, &got)
			if record.Rejected != (got["outcome"] == "rejected") {
				t.Error("judge_rejected differs from judge outcome")
			}
			if code != record.ExitCode {
				t.Errorf("exit code %d, record expects %d", code, record.ExitCode)
			}
			if !reflect.DeepEqual(got, record.Output) {
				t.Errorf("judge output differs from record")
			}
			var input struct {
				Required []struct {
					ID        string `json:"id"`
					Dimension string `json:"dimension"`
				} `json:"required"`
			}
			decode(t, record.Input, &input)
			ids := map[string]bool{}
			probes := map[string]string{}
			for _, check := range input.Required {
				if ids[check.ID] {
					t.Errorf("duplicate required id %q", check.ID)
				}
				ids[check.ID] = true
				probes[check.Dimension] = check.ID
			}
			var evidence struct {
				Evidence []struct {
					ID       string         `json:"check_id"`
					Expected map[string]any `json:"expected"`
				} `json:"evidence"`
			}
			decode(t, record.Input, &evidence)
			matched := map[string]bool{}
			for _, item := range evidence.Evidence {
				if item.ID == probes["bounding"] && item.Expected["bound"] == float64(subject.Commands[0].Bound) {
					matched["bounding"] = true
				}
				codes, err := json.Marshal(subject.Commands[0].Codes)
				if err != nil {
					t.Fatal(err)
				}
				var expectedCodes any
				decode(t, codes, &expectedCodes)
				if item.ID == probes["execution"] && reflect.DeepEqual(item.Expected["exit_codes"], expectedCodes) {
					matched["execution"] = true
				}
			}
			if !matched["bounding"] || !matched["execution"] {
				t.Error("required probes must have evidence matching list bounds and exit codes")
			}
			if probes["bounding"] == "" || probes["execution"] == "" || probes["bounding"] == probes["execution"] {
				t.Error("byte bound and exit codes require distinct ids")
			}
		})
	}
}

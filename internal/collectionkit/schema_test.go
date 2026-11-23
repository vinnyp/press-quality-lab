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

// enumSites retains the complete schema site through compositions.
func enumSites(v any, path string, found map[string][]any) {
	switch node := v.(type) {
	case map[string]any:
		if values, ok := node["enum"].([]any); ok {
			found[path] = values
		}
		if value, ok := node["const"]; ok {
			found[path] = append(found[path], value)
		}
		for key, child := range node {
			enumSites(child, path+"/"+key, found)
		}
	case []any:
		for i, child := range node {
			enumSites(child, fmt.Sprintf("%s/%d", path, i), found)
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
	oracles := map[string]map[string][]any{}
	for _, name := range []string{"judge-input.schema.json", "judge-output.schema.json"} {
		var v any
		decode(t, read(t, filepath.Join(sealed, "schema", name)), &v)
		sites := map[string][]any{}
		enumSites(v, "", sites)
		oracles[name] = sites
	}
	input, output := oracles["judge-input.schema.json"], oracles["judge-output.schema.json"]
	files, err := filepath.Glob(filepath.Join(kit, "schema", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no schemas")
	}
	for _, file := range files {
		t.Run(strings.ReplaceAll(filepath.Base(file), ".", "_"), func(t *testing.T) {
			var v any
			decode(t, read(t, file), &v)
			sites := map[string][]any{}
			enumSites(v, "", sites)
			for site, values := range sites {
				var expected []any
				switch {
				case oracles[filepath.Base(file)] != nil:
					expected = oracles[filepath.Base(file)][site]
				case filepath.Base(file) == "subject-file.schema.json" && (site == "/properties/secret/oneOf/2/properties/kind" || site == "/properties/secret/oneOf/2/if/properties/kind"):
					expected = []any{"credential", "seeded_state"}
				case filepath.Base(file) == "case.schema.json" && site == "/properties/kind":
					expected = []any{"judge-checked", "derivation-only"}
				case filepath.Base(file) == "case.schema.json" && site == "/properties/entry/properties/fixtures/items/properties/kind":
					expected = []any{"link", "special_file", "oversized_output", "invalid_bytes", "planted_secret"}
				case filepath.Base(file) == "baseline.schema.json" && site == "/properties/baseline/properties/entries/items/properties/kind":
					expected = input["/$defs/baseline/properties/entries/items/properties/kind"]
				case strings.HasSuffix(site, "/properties/dimension") || site == "/$defs/dimension":
					expected = input["/properties/required/items/properties/dimension"]
				case filepath.Base(file) == "record.schema.json" && site == "/properties/next_actions/items/oneOf/0/properties/reason":
					expected = output["/properties/checks/items/properties/reason"]
				case filepath.Base(file) == "record.schema.json" && site == "/properties/next_actions/items/oneOf/1/properties/reason":
					expected = output["/properties/dimensions/items/properties/reason"]
				case strings.HasSuffix(site, "/properties/failure_kind"), strings.HasSuffix(site, "/properties/tier"), site == "/properties/claimed_tier":
					expected = input[site]
					if filepath.Base(file) == "baseline.schema.json" && site == "/properties/baseline/properties/entries/items/properties/tier" {
						expected = input["/$defs/baseline/properties/entries/items/properties/tier"]
					}
					if filepath.Base(file) == "judge-output.schema.json" {
						switch site {
						case "/$defs/baseline_entry/properties/tier":
							expected = input["/$defs/baseline/properties/entries/items/properties/tier"]
						default:
							t.Errorf("no input tier oracle for output site %s", site)
							continue
						}
					}
				default:
					parts := strings.Split(site, "/")
					property := ""
					for i := 0; i+1 < len(parts); i++ {
						if parts[i] == "properties" || parts[i] == "$defs" {
							property = parts[i+1]
						}
					}
					if slices.Contains([]string{"reason", "failure_kind", "tier", "claimed_tier", "dimension", "kind", "outcome"}, property) {
						t.Errorf("unmapped enum site %s", site)
					}
					continue
				}
				if len(expected) == 0 {
					t.Errorf("no oracle for %s", site)
					continue
				}
				for _, value := range values {
					if !slices.ContainsFunc(expected, func(want any) bool { return reflect.DeepEqual(want, value) }) {
						t.Errorf("%s enum value %v absent from sealed judge site", site, value)
					}
				}
			}
		})
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

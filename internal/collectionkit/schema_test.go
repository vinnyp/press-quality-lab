package collectionkit

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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

// Carry the property or definition name through compositions to find nested enums.
func enums(v any, name string, found map[string]map[string]bool) {
	switch node := v.(type) {
	case map[string]any:
		if values, ok := node["enum"].([]any); ok {
			category := name
			if strings.Contains(name, "reason") {
				category = "reason"
			}
			if strings.Contains(name, "tier") {
				category = "tier"
			}
			if category == "reason" || category == "tier" || category == "failure_kind" {
				if found[category] == nil {
					found[category] = map[string]bool{}
				}
				for _, value := range values {
					if s, ok := value.(string); ok {
						found[category][s] = true
					}
				}
			}
		}
		for key, child := range node {
			next := name
			if key == "properties" || key == "$defs" {
				if fields, ok := child.(map[string]any); ok {
					for field, schema := range fields {
						enums(schema, field, found)
					}
					continue
				}
			}
			enums(child, next, found)
		}
	case []any:
		for _, child := range node {
			enums(child, name, found)
		}
	}
}

func TestOutputSchemaCopy(t *testing.T) {
	for _, name := range []string{"judge-output.schema.json"} {
		t.Run(name, func(t *testing.T) {
			if !bytes.Equal(read(t, filepath.Join(kit, "schema", name)), read(t, filepath.Join(sealed, "schema", name))) {
				t.Fatal("output schema differs from sealed judge")
			}
		})
	}
}
func TestSchemaEnums(t *testing.T) {
	expected := map[string]map[string]bool{}
	for _, name := range []string{"judge-input.schema.json", "judge-output.schema.json"} {
		var v any
		decode(t, read(t, filepath.Join(sealed, "schema", name)), &v)
		sets := map[string]map[string]bool{}
		enums(v, "", sets)
		for category, values := range sets {
			if name == "judge-output.schema.json" && category != "reason" {
				continue
			}
			if expected[category] == nil {
				expected[category] = map[string]bool{}
			}
			for value := range values {
				expected[category][value] = true
			}
		}
	}
	files, err := filepath.Glob(filepath.Join(kit, "schema", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no schemas")
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			var v any
			decode(t, read(t, file), &v)
			found := map[string]map[string]bool{}
			enums(v, "", found)
			for category, values := range found {
				for value := range values {
					if !expected[category][value] {
						t.Errorf("%s enum value %q absent from sealed judge", category, value)
					}
				}
			}
		})
	}
	for _, category := range []string{"reason", "failure_kind", "tier"} {
		if len(expected[category]) == 0 {
			t.Fatalf("no sealed %s enum", category)
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
		t.Run(filepath.Base(file), func(t *testing.T) {
			var record struct {
				Input    json.RawMessage `json:"judge_input"`
				Output   any             `json:"judge_output"`
				ExitCode int             `json:"exit_code"`
			}
			decode(t, read(t, file), &record)
			output, code := judge.Judge(record.Input)
			var got map[string]any
			decode(t, output, &got)
			if got["outcome"] == "rejected" {
				t.Fatal("judge rejected example")
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

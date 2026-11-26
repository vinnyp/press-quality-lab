package subject

import (
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	// QS1: absent, numeric and other versions are rejected before shape checks.
	for _, value := range []any{nil, strictjson.Number("1.0"), "2.0"} {
		o := parsedFixture(t)
		checkFixture(t, o, "")
		if value == nil {
			delete(o, "subject_file_version")
		} else {
			o["subject_file_version"] = value
		}
		checkFixture(t, o, "QS1")
	}

}

func TestCorpus(t *testing.T) {
	// QS1, QS2, QS3, QS4, QS5, QS6, QS7, QS10, QS11, QP10, QA4, QA9:
	// declaration refusals; file-content refusals use the grade reader.
	dirs, err := filepath.Glob("../../docs/specs/conformance-collection-1.0/cases/case-*")
	if err != nil || len(dirs) == 0 {
		t.Fatal("missing corpus", err)
	}
	refusals := map[string]string{
		"case_100": "QS2",
		"case_104": "QS6",
		"case_105": "QS6",
		"case_106": "QS6",
		"case_107": "QS7",
		"case_108": "QS7",
		"case_109": "QS7",
		"case_113": "QS10",
		"case_114": "QS11",
		"case_115": "QS11",
		"case_122": "QP10",
		"case_123": "QA4",
		"case_124": "QA4",
		"case_125": "QA9",
		"case_126": "QA9",
		"case_127": "QA9",
		"case_128": "QA9",
		"case_101": "QS4",
		"case_102": "QS3",
		"case_103": "QS5",
		"case_116": "QS11",
		"case_129": "QS1",
	}
	for _, dir := range dirs {
		t.Run(strings.ReplaceAll(filepath.Base(dir), "-", "_"), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, "subject.json"))
			if err != nil {
				t.Fatal(err)
			}
			_, refusal := Parse(data)
			want := refusals[strings.ReplaceAll(filepath.Base(dir), "-", "_")]
			assertRule(t, refusal, want)
		})
	}
}
func assertRule(t *testing.T, got *Error, want string) {
	t.Helper()
	if got == nil {
		if want != "" {
			t.Fatalf("wanted %s", want)
		}
	} else if got.Rule != want {
		t.Fatalf("got %s want %s", got.Rule, want)
	}
}
func baseline(t *testing.T) []byte {
	t.Helper()
	b, e := os.ReadFile("../../docs/specs/conformance-collection-1.0/cases/case-001/subject.json")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestInvalidExamples(t *testing.T) {
	// QS5 and QP10: the sealed invalid examples name their broken rule.
	paths, e := filepath.Glob("../../docs/specs/conformance-collection-1.0/examples/invalid/subject-file-*.json")
	if e != nil || len(paths) != 8 {
		t.Fatal(paths, e)
	}
	for _, p := range paths {
		parts := strings.Split(filepath.Base(p), "-")
		t.Run(strings.ReplaceAll(strings.TrimSuffix(filepath.Base(p), ".json"), "-", "_"), func(t *testing.T) {
			b, e := os.ReadFile(p)
			if e != nil {
				t.Fatal(e)
			}
			_, err := Parse(b)
			assertRule(t, err, parts[2])
		})
	}
}
func TestFacts(t *testing.T) {
	// QS4: every fact rejects each wrong JSON type; placeholders remain distinct.
	for _, key := range []string{"read_only", "has_destructive_ops", "store_backed", "has_cli", "bounding"} {
		for i, value := range []any{nil, "wrong", strictjson.Number("1"), strictjson.Array{}, strictjson.Object{}, true, false} {
			if key != "bounding" && (value == true || value == false) {
				continue
			}
			t.Run(key+"_"+strconv.Itoa(i), func(t *testing.T) {
				v, ok := strictjson.Read(baseline(t), strictjson.Bounds{Bytes: 16 << 20, Depth: 64, NumberChars: 32, ExponentDigits: 4}, nil)
				if !ok {
					t.Fatal("fixture")
				}
				o := testObject(v)
				checkFixture(t, o, "")
				testObject(o["facts"])[key] = value
				data := strictjson.Write(o)
				_, err := Parse(data)
				assertRule(t, err, "QS4")
			})
		}
	}
}
func TestStrictReading(t *testing.T) {
	// QS2: each strict-reading clause and an otherwise unnamed schema refusal.
	b := baseline(t)
	literal := func(raw string) []byte {
		o := parsedFixture(t)
		firstCheck(o)["expected"] = strictjson.Object{"literal": "REPLACE_LITERAL"}
		return []byte(strings.Replace(string(strictjson.Write(o)), `"REPLACE_LITERAL"`, raw, 1))
	}
	// The literal sits under eight containers; 57 nested arrays make depth 65.
	for name, raw := range map[string]string{
		"depth_at_bound":    strings.Repeat("[", 48) + "0" + strings.Repeat("]", 48),
		"number_at_bound":   "10000000000000000000000000000000",
		"exponent_at_bound": "1e0001",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(literal(raw)); err != nil {
				t.Fatalf("at bound: %v", err)
			}
		})
	}

	inputs := map[string][]byte{
		"duplicate": append([]byte(`{"subject_file_version":"1.0",`), b[1:]...),
		"utf8":      {0xff}, "surrogate": []byte(strings.Replace(string(b), `"none"`, `"\ud800"`, 1)), "trailing": append(append([]byte{}, b...), '0'),
		"size":          []byte(strings.Repeat(" ", 16<<20) + string(b)),
		"depth":         literal(strings.Repeat("[", 57) + "0" + strings.Repeat("]", 57)),
		"number_length": literal(strings.Repeat("1", 33)), "exponent": literal(`1e00001`), "syntax": []byte(`{"x":01}`),
		"schema": []byte(strings.Replace(string(b), `"version": "none"`, `"version": 0`, 1)),
	}
	for name, b := range inputs {
		t.Run(name, func(t *testing.T) { _, err := Parse(b); assertRule(t, err, "QS2") })
	}
}

func TestSchemaRules(t *testing.T) {
	// QS6, QS7, QS9, QA4, QA9: only schema-detectable declaration constraints.
	rows := []struct {
		name, rule string
		change     func(strictjson.Object)
	}{
		{"command_budget", "QS6", func(o strictjson.Object) {
			testObject(testArray(o["commands"])[0])["budget_seconds"] = strictjson.Number("1.0")
		}},
		{"offline_argument", "QS7", func(o strictjson.Object) {
			o["offline"] = strictjson.Array{strictjson.Object{"applies_to": strictjson.Array{strictjson.Array{}}, "args": strictjson.Array{false}, "env": strictjson.Object{}}}
		}},
		{"absolute_store", "QS9", func(o strictjson.Object) { o["store"] = "/store" }},
		{"computation_budget", "QA4", func(o strictjson.Object) {
			c := firstCheck(o)
			c["expected"] = strictjson.Object{"computation": strictjson.Object{"argv": strictjson.Array{"/bin/echo"}, "budget_seconds": strictjson.Number("601"), "declared_files": strictjson.Array{}, "captures": strictjson.Array{}}}
		}},
		{"predicate", "QA9", func(o strictjson.Object) { firstCheck(o)["predicate"] = "unknown" }},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			o := parsedFixture(t)
			row.change(o)
			_, err := Parse(strictjson.Write(o))
			assertRule(t, err, row.rule)
		})
	}
}
func parsedFixture(t *testing.T) strictjson.Object {
	t.Helper()
	v, ok := strictjson.Read(baseline(t), strictjson.Bounds{Bytes: 16 << 20, Depth: 64, NumberChars: 32, ExponentDigits: 4}, nil)
	if !ok {
		t.Fatal("fixture")
	}
	return testObject(v)
}
func testObject(v any) strictjson.Object {
	o, ok := v.(strictjson.Object)
	if !ok {
		panic("fixture requires object")
	}
	return o
}
func testArray(v any) strictjson.Array {
	a, ok := v.(strictjson.Array)
	if !ok {
		panic("fixture requires array")
	}
	return a
}
func firstCheck(o strictjson.Object) strictjson.Object {
	rb := testObject(testArray(o["runbooks"])[0])
	step := testObject(testArray(rb["steps"])[0])
	return testObject(testArray(step["checks"])[0])
}

func TestPreservesModel(t *testing.T) {
	// QS4 and QS5: all facts and placeholders remain available without coercion.
	o := parsedFixture(t)
	facts := strictjson.Object{"read_only": false, "has_destructive_ops": true, "store_backed": false, "has_cli": false, "bounding": "none"}
	o["facts"] = facts
	o["binary"] = strictjson.Object{"placeholder": "Supply the executable."}
	got, err := Parse(strictjson.Write(o))
	if err != nil {
		t.Fatal(err)
	}
	if !strictjson.EqualJSON(got, o) {
		t.Fatal("model changed")
	}
}

func TestSchemaBounds(t *testing.T) {
	// QS2, QS6, QS11: schema boundaries are literals from subject-file.schema.json.
	for _, n := range []int{0, 1, 3600, 3601} {
		t.Run("budget_"+strconv.Itoa(n), func(t *testing.T) {
			o := parsedFixture(t)
			testObject(testArray(o["commands"])[0])["budget_seconds"] = strictjson.Number(strconv.Itoa(n))
			_, err := Parse(strictjson.Write(o))
			want := ""
			if n == 0 || n == 3601 {
				want = "QS6"
			}
			assertRule(t, err, want)
		})
	}
	for _, n := range []int{511, 512, 513} {
		t.Run("commands_"+strconv.Itoa(n), func(t *testing.T) {
			o := parsedFixture(t)
			command := testObject(testArray(o["commands"])[0])
			a := make(strictjson.Array, n)
			for i := range a {
				copy := strictjson.Object{}
				for k, v := range command {
					copy[k] = v
				}
				copy["words"] = strictjson.Array{"unique" + strconv.Itoa(i)}
				a[i] = copy
			}
			original := testArray(o["commands"])
			copy(a, original)
			o["commands"] = a
			_, err := Parse(strictjson.Write(o))
			want := ""
			if n == 513 {
				want = "QS11"
			}
			assertRule(t, err, want)
		})
	}
}

func TestRuleOrder(t *testing.T) {
	// QS1, QS3, QS4, QS5, QS6, QS7, QS9, QS11, QP10, QA4, QA9:
	// E-9 orders named failures ahead of the QS2 fallback.
	o := parsedFixture(t)
	o["subject_file_version"] = "2.0"
	delete(o, "binary")
	testObject(o["facts"])["read_only"] = nil
	testObject(testArray(o["commands"])[0])["budget_seconds"] = strictjson.Object{"placeholder": "Budget required."}
	_, err := Parse(strictjson.Write(o))
	assertRule(t, err, "QS1")
	o["subject_file_version"] = "1.0"
	_, err = Parse(strictjson.Write(o))
	assertRule(t, err, "QS3")
	o["binary"] = "bin/widgets"
	_, err = Parse(strictjson.Write(o))
	assertRule(t, err, "QS4")
	testObject(o["facts"])["read_only"] = true
	_, err = Parse(strictjson.Write(o))
	assertRule(t, err, "QS5")
}

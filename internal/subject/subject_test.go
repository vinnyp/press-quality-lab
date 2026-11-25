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
	for _, input := range []string{`{}`, `{"subject_file_version":1.0}`, `{"subject_file_version":"2.0"}`} {
		if _, err := Parse([]byte(input)); err == nil || err.Rule != "QS1" {
			t.Fatalf("got %v", err)
		}
	}
}

func TestCorpus(t *testing.T) {
	// QS1, QS2, QS3, QS4, QS5, QS11: schema refusals; semantic refusals wait for U9.
	dirs, err := filepath.Glob("../../docs/specs/conformance-collection-1.0/cases/case-*")
	if err != nil || len(dirs) == 0 {
		t.Fatal("missing corpus", err)
	}
	refusals := map[string]string{
		"case_100": "QS2",
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
	inputs := map[string][]byte{
		"duplicate": append([]byte(`{"subject_file_version":"1.0",`), b[1:]...),
		"utf8":      {0xff}, "surrogate": []byte(`"\ud800"`), "trailing": append(append([]byte{}, b...), '0'),
		"size":          []byte(strings.Repeat(" ", 16<<20) + string(b)),
		"depth":         []byte(strings.Repeat("[", 65) + strings.Repeat("]", 65)),
		"number_length": []byte(strings.Repeat("1", 33)), "exponent": []byte(`1e00001`), "syntax": []byte(`{"x":01}`),
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
			command := testArray(o["commands"])[0]
			a := make(strictjson.Array, n)
			for i := range a {
				a[i] = command
			}
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

func TestRead(t *testing.T) {
	// QS2: file reads use exactly the same strict grammar as byte reads.
	path := filepath.Join(t.TempDir(), "subject.json")
	if err := os.WriteFile(path, baseline(t), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Read(path)
	if err != nil || got == nil {
		t.Fatalf("read: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"x":0,"x":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = Read(path)
	refusal, ok := err.(*Error)
	if !ok {
		t.Fatalf("got %v", err)
	}
	assertRule(t, refusal, "QS2")
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

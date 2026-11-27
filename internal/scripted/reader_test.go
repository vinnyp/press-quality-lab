//go:build scripted

package scripted

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func TestProvenance(t *testing.T) {
	// QT1, QT5: provenance shape is required; its value is never compared.
	b, err := os.ReadFile("../../docs/specs/conformance-collection-1.0/examples/behaviour-01.json")
	if err != nil {
		t.Fatal(err)
	}
	h := []byte(`{"confinement":{"applied":true,"sandbox_exec_exit":null},"self_test":"passed","known_fast":"within","read_calibration":"moved","store_control":"detected","read_control":"killed","private_copy":"unchanged","kill_attribution":[]}`)
	if _, err := Read(h, b, nil); err != nil {
		t.Fatal(err)
	}

}

const kit = "../../docs/specs/conformance-collection-1.0"
const passing = `{"confinement":{"applied":true,"sandbox_exec_exit":null},"self_test":"passed","known_fast":"within","read_calibration":"moved","store_control":"detected","read_control":"killed","private_copy":"unchanged","kill_attribution":[]}`

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func decode(t *testing.T, b []byte) object {
	t.Helper()
	v, ok := strictjson.Read(b, strictjson.Bounds{Bytes: 16 << 20, Depth: 64, NumberChars: 32, ExponentDigits: 4}, nil)
	if !ok {
		t.Fatal("fixture JSON invalid")
	}
	return obj(v)
}
func same(t *testing.T, got any, want any) {
	t.Helper()
	// Independent decoder checks the production serialiser's whole JSON shape.
	var g, w any
	gotBytes, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	d := json.NewDecoder(bytes.NewReader(gotBytes))
	d.UseNumber()
	if err := d.Decode(&g); err != nil {
		t.Fatal(err)
	}
	d = json.NewDecoder(bytes.NewReader(strictjson.Write(want)))
	d.UseNumber()
	if err := d.Decode(&w); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(g, w) {
		t.Fatalf("got %v want %v", g, w)
	}
}
func TestCorpusProjection(t *testing.T) {
	// QT3, QT4, QT5: every first confined run and every host project field for field.
	paths, err := filepath.Glob(filepath.Join(kit, "behaviour", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 54 {
		t.Fatalf("recordings: %d", len(paths))
	}
	for _, path := range paths {
		t.Run(strings.ReplaceAll(strings.TrimSuffix(filepath.Base(path), ".json"), "-", "_"), func(t *testing.T) {
			b := readFile(t, path)
			s, err := Read([]byte(passing), b, nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, v := range arr(decode(t, b)["invocations"]) {
				i := obj(v)
				r, err := s.Observe(map[string]any(obj(i["key"])), observe.Spec{})
				if err != nil {
					t.Fatal(err)
				}
				same(t, observe.ProjectRun(r), arr(i["confined"])[0])
			}
		})
	}
	cases, err := filepath.Glob(filepath.Join(kit, "cases", "*", "case.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 152 {
		t.Fatalf("cases: %d", len(cases))
	}
	hosts := 0
	for _, path := range cases {
		c := decode(t, readFile(t, path))
		in := obj(c["input"])
		if in["host"] == nil {
			continue
		}
		hosts++
		s, err := Read(strictjson.Write(in["host"]), readFile(t, filepath.Join(kit, "behaviour", textValue(in["form"])+".json")), nil)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		controls, err := s.Controls()
		if err != nil {
			t.Fatal(err)
		}
		same(t, observe.ProjectHost(controls), in["host"])
		recording := decode(t, readFile(t, filepath.Join(kit, "behaviour", textValue(in["form"])+".json")))
		for _, value := range arr(recording["invocations"]) {
			invocation := obj(value)
			run, err := s.Observe(map[string]any(obj(invocation["key"])), observe.Spec{})
			if err != nil {
				t.Fatal(err)
			}
			same(t, observe.ProjectRun(run), arr(invocation["confined"])[0])
		}

	}
	if hosts != 149 {
		t.Fatalf("host count %d", hosts)
	}
}
func errorCode(t *testing.T, err error, code int, rule string) {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) || e.Code != code || e.Rule != rule {
		t.Fatalf("error %v want %d %s", err, code, rule)
	}
}
func TestInvalidExamples(t *testing.T) {
	// QT1, QT4, QT5: every invalid host and behaviour example is rejected.
	paths, err := filepath.Glob(filepath.Join(kit, "examples", "invalid", "behaviour-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 8 {
		t.Fatalf("invalid behaviour count %d", len(paths))
	}
	for _, path := range paths {
		_, err := Read([]byte(passing), readFile(t, path), nil)
		errorCode(t, err, 2, "QT5")
	}
	paths, err = filepath.Glob(filepath.Join(kit, "examples", "invalid", "case-QT4-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 {
		t.Fatalf("invalid host count %d", len(paths))
	}
	for _, path := range paths {
		in := obj(decode(t, readFile(t, path))["input"])
		_, err := Read(strictjson.Write(in["host"]), readFile(t, filepath.Join(kit, "examples", "behaviour-01.json")), nil)
		errorCode(t, err, 2, "QT4")
	}
}
func TestMissingInvocation(t *testing.T) {
	// QT3, QT5: a needed invocation absent from the recording is an engine fault.
	s, err := Read([]byte(passing), readFile(t, filepath.Join(kit, "examples", "behaviour-02.json")), nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Observe(observe.Key{"words": array{"missing"}}, observe.Spec{})
	errorCode(t, err, 70, "QT5")
}

func TestInputRefusals(t *testing.T) {
	// QT1, QT4, QT5: malformed JSON and invalid relations never reach collection.
	base := readFile(t, filepath.Join(kit, "examples", "behaviour-01.json"))
	for _, row := range []struct {
		name string
		edit func(object)
	}{
		{"ProvenanceShape", func(b object) { b["form_sha256"] = "x" }},
		{"DuplicateKey", func(b object) { b["invocations"] = append(arr(b["invocations"]), arr(b["invocations"])[0]) }},
		{"Agreement", func(b object) {
			first := obj(arr(obj(arr(b["invocations"])[0])["confined"])[1])
			first["end"] = object{"kind": "exit", "code": number("1")}
		}},
		{"EnvironmentOrder", func(b object) { obj(obj(arr(b["invocations"])[0])["key"])["env"] = array{"Z", "A"} }},
		{"MissingField", func(b object) { delete(b, "system") }},
		{"ExtraField", func(b object) { b["extra"] = nil }},
		{"WrongType", func(b object) { b["violating"] = "true" }},
		{"FiveRuns", func(b object) { i := obj(arr(b["invocations"])[0]); i["confined"] = arr(i["confined"])[:4] }},
		{"PositiveControl", func(b object) { obj(arr(b["invocations"])[0])["unconfined"] = nil }},
	} {
		t.Run(row.name, func(t *testing.T) {
			b := decode(t, base)
			row.edit(b)
			_, err := Read([]byte(passing), strictjson.Write(b), nil)
			errorCode(t, err, 2, "QT5")
		})
	}
	for _, raw := range []string{`{"x":1,"x":2}`, `{} {}`, "\xff", `"\ud800"`, strings.Repeat("[", 65) + "0" + strings.Repeat("]", 65), strings.Repeat("1", 33), "1e10000", strings.Repeat(" ", 16<<20) + "0"} {
		_, err := Read([]byte(passing), []byte(raw), nil)
		errorCode(t, err, 2, "QT1")
		_, err = Read([]byte(raw), base, nil)
		errorCode(t, err, 2, "QT1")
	}
	for _, row := range []struct {
		name string
		edit func(object)
	}{
		{"MissingControl", func(h object) { delete(h, "self_test") }},
		{"ConfinementPair", func(h object) { obj(h["confinement"])["sandbox_exec_exit"] = number("1") }},
		{"UnknownOverride", func(h object) {
			h["kill_attribution"] = array{object{"invocation": obj(arr(decode(t, base)["invocations"])[0])["key"], "by": "unknown"}}
		}},
		{"MissingOverrideKey", func(h object) {
			k := obj(obj(arr(decode(t, base)["invocations"])[0])["key"])
			k["words"] = array{"absent"}
			h["kill_attribution"] = array{object{"invocation": k, "by": "memory"}}
		}},
	} {
		t.Run(row.name, func(t *testing.T) {
			h := decode(t, []byte(passing))
			row.edit(h)
			_, err := Read(strictjson.Write(h), base, nil)
			errorCode(t, err, 2, "QT4")
		})
	}

}
func TestAgreementScope(t *testing.T) {
	// QT5: agreement names byte count/digest/cut, not stream head/kept or the home control.
	b := decode(t, readFile(t, filepath.Join(kit, "examples", "behaviour-02.json")))
	other := obj(arr(obj(arr(b["invocations"])[0])["confined"])[1])
	obj(other["stdout"])["head"] = "a different head"
	obj(other["stdout"])["kept"] = "eA=="
	obj(other["home_changes"])["control_detected"] = true
	if _, err := Read([]byte(passing), strictjson.Write(b), nil); err != nil {
		t.Fatal(err)
	}
}
func TestHostOverrides(t *testing.T) {
	// QT4: overrides target only the first confined confinement kill, once per key.
	base := readFile(t, filepath.Join(kit, "examples", "behaviour-01.json"))
	b := decode(t, base)
	key := obj(obj(arr(b["invocations"])[0])["key"])
	for _, by := range []string{"memory", "code_signing", "engine"} {
		t.Run(by, func(t *testing.T) {
			h := decode(t, []byte(passing))
			entry := object{"invocation": key, "by": by}
			h["kill_attribution"] = array{entry}
			s, err := Read(strictjson.Write(h), base, nil)
			if err != nil {
				t.Fatal(err)
			}
			c, err := s.Controls()
			if err != nil {
				t.Fatal(err)
			}
			same(t, observe.ProjectHost(c), h)
			r, err := s.Observe(map[string]any(key), observe.Spec{})
			if err != nil {
				t.Fatal(err)
			}
			if r.End.MemoryKilled != (by == "memory") || r.End.CodeSigningKilled != (by == "code_signing") || r.End.EngineKilled != (by == "engine") || r.End.BudgetExpired {
				t.Fatalf("raw end %+v", r.End)
			}
			h["kill_attribution"] = array{entry, entry}
			_, err = Read(strictjson.Write(h), base, nil)
			errorCode(t, err, 2, "QT4")
		})
	}
	h := decode(t, []byte(passing))
	obj(h["confinement"])["applied"] = false
	obj(h["confinement"])["sandbox_exec_exit"] = number("65")
	s, err := Read(strictjson.Write(h), base, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.Observe(map[string]any(key), observe.Spec{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Markers.Confinement || r.End.Code != 65 || r.End.Signal != 0 {
		t.Fatalf("confinement override %+v", r)
	}
	h = decode(t, []byte(passing))
	h["private_copy"] = "changed"
	s, err = Read(strictjson.Write(h), base, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err = s.Observe(map[string]any(key), observe.Spec{})
	if err != nil || !r.CopyChanged {
		t.Fatalf("copy override %v %v", r.CopyChanged, err)
	}
}
func TestTrace(t *testing.T) {
	// KTD2: literal event order and shapes match corpusrunner's readTrace format.
	var trace bytes.Buffer
	s, err := Read([]byte(passing), readFile(t, filepath.Join(kit, "examples", "behaviour-02.json")), &trace)
	if err != nil {
		t.Fatal(err)
	}
	b := decode(t, readFile(t, filepath.Join(kit, "examples", "behaviour-02.json")))
	key := obj(obj(arr(b["invocations"])[0])["key"])
	if err := s.Home("early"); err == nil {
		t.Fatal("early home accepted")
	}
	if _, err := s.Observe(map[string]any(key), observe.Spec{}); err == nil {
		t.Fatal("early serve accepted")
	}
	if err := s.Required(array{number("9007199254740993")}); err != nil {
		t.Fatal(err)
	}
	if err := s.Required(array{}); err == nil {
		t.Fatal("repeated required accepted")
	}
	if _, err := s.Observe(map[string]any(key), observe.Spec{}); err != nil {
		t.Fatal(err)
	}
	if err := s.Home("made"); err != nil {
		t.Fatal(err)
	}
	// UseNumber and sequential objects are exactly the corpus reader's wire format.
	d := json.NewDecoder(bytes.NewReader(trace.Bytes()))
	d.UseNumber()
	var events []map[string]any
	for {
		var e map[string]any
		err := d.Decode(&e)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, e)
	}
	if len(events) != 3 || len(events[0]) != 2 || len(events[1]) != 2 || len(events[2]) != 2 {
		t.Fatalf("events %+v", events)
	}
	if events[0]["kind"] != "required" || !reflect.DeepEqual(events[0]["value"], []any{json.Number("9007199254740993")}) || events[1]["kind"] != "served" || events[2]["kind"] != "home" || events[2]["value"] != "made" {
		t.Fatalf("events %+v", events)
	}
	var wantKey any
	if err := json.Unmarshal(strictjson.Write(key), &wantKey); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(events[1]["value"], wantKey) {
		t.Fatalf("served key %+v", events[1])
	}
}

func TestSchemaBoundaries(t *testing.T) {
	// QT1, QT5: limits come from the sealed schemas, including integral decimal spellings.
	base := readFile(t, filepath.Join(kit, "examples", "behaviour-02.json"))
	for _, row := range []struct {
		name  string
		codes []string
		valid bool
	}{
		{"BelowExit", []string{"-1"}, false}, {"LowerExit", []string{"0", "0.0", "0e1"}, true},
		{"UpperExit", []string{"255", "255.0", "255e0"}, true}, {"AboveExit", []string{"256"}, false}, {"FractionalExit", []string{"0.5"}, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			for _, code := range row.codes {
				b := decode(t, base)
				for _, i := range arr(b["invocations"]) {
					for _, run := range arr(obj(i)["confined"]) {
						obj(run)["end"] = object{"kind": "exit", "code": number(code)}
					}
				}
				_, err := Read([]byte(passing), strictjson.Write(b), nil)
				if (err == nil) != row.valid {
					t.Fatalf("%s: %v", code, err)
				}
			}
		})
	}
	for _, count := range []int{4, 5, 6} {
		b := decode(t, base)
		i := obj(arr(b["invocations"])[0])
		a := arr(i["confined"])
		if count < 5 {
			a = a[:count]
		} else if count > 5 {
			a = append(a, a[0])
		}
		i["confined"] = a
		_, err := Read([]byte(passing), strictjson.Write(b), nil)
		if (err == nil) != (count == 5) {
			t.Fatalf("run count %d: %v", count, err)
		}
	}
	for _, size := range []int{63, 64, 65} {
		b := decode(t, base)
		b["form_sha256"] = strings.Repeat("a", size)
		_, err := Read([]byte(passing), strictjson.Write(b), nil)
		if (err == nil) != (size == 64) {
			t.Fatalf("digest size %d: %v", size, err)
		}
	}
	for _, size := range []int{39, 40, 41} {
		b := decode(t, base)
		b["commit"] = strings.Repeat("a", size)
		_, err := Read([]byte(passing), strictjson.Write(b), nil)
		if (err == nil) != (size == 40) {
			t.Fatalf("commit size %d: %v", size, err)
		}
	}
	for _, count := range []string{"-1", "0", "1e32", "0.5"} {
		b := decode(t, base)
		for _, i := range arr(b["invocations"]) {
			for _, run := range arr(obj(i)["confined"]) {
				obj(obj(run)["stdout"])["bytes"] = number(count)
			}
		}
		s, err := Read([]byte(passing), strictjson.Write(b), nil)
		// QE10: schema-valid oversized integers are engine faults.
		if count == "1e32" {
			errorCode(t, err, 70, "QE10")
			continue
		}
		valid := count == "0"
		if (err == nil) != valid {
			t.Fatalf("bytes %s: %v", count, err)
		}
		if valid {
			i := obj(arr(b["invocations"])[0])
			r, err := s.Observe(map[string]any(obj(i["key"])), observe.Spec{})
			if err != nil {
				t.Fatal(err)
			}
			same(t, observe.ProjectRun(r), arr(i["confined"])[0])
		}
	}
	h := decode(t, []byte(passing))
	obj(h["confinement"])["applied"] = false
	obj(h["confinement"])["sandbox_exec_exit"] = number("1e32")
	_, err := Read(strictjson.Write(h), base, nil)
	errorCode(t, err, 70, "QE10")
}

func TestRawValuesProjection(t *testing.T) {
	// QT5: raw shapes preserve arbitrary allowed signals, kept encodings and printed JSON.
	base := readFile(t, filepath.Join(kit, "examples", "behaviour-02.json"))
	for _, row := range []struct {
		name string
		end  object
	}{
		{"OtherSignal", object{"kind": "signal", "signal": "SIG999", "by": nil}},
		{"BeforeMarker", object{"kind": "signal", "signal": "SIGKILL", "by": "before_marker"}},
		{"Budget", object{"kind": "signal", "signal": "SIGKILL", "by": "budget"}},
		{"Memory", object{"kind": "signal", "signal": "SIGKILL", "by": "memory"}},
		{"CodeSigning", object{"kind": "signal", "signal": "SIGKILL", "by": "code_signing"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			b := decode(t, base)
			i := obj(arr(b["invocations"])[0])
			for _, value := range arr(i["confined"]) {
				run := obj(value)
				run["end"] = row.end
				run["planted"] = object{"atime_moved": false, "same_file": false, "control_end": row.end}
				if row.end["by"] == "before_marker" {
					obj(run["markers"])["confinement"] = false
				}
				run["printed_value"] = object{"value": object{"n": number("9007199254740993"), "null": nil, "list": array{true}}}
				obj(run["stdout"])["kept"] = "AB=="
			}
			s, err := Read([]byte(passing), strictjson.Write(b), nil)
			if err != nil {
				t.Fatal(err)
			}
			r, err := s.Observe(map[string]any(obj(i["key"])), observe.Spec{})
			if err != nil {
				t.Fatal(err)
			}
			same(t, observe.ProjectRun(r), arr(i["confined"])[0])
		})
	}
}

type traceFailure struct {
	err   error
	short bool
}

func (w traceFailure) Write(p []byte) (int, error) {
	if w.short {
		return len(p) - 1, nil
	}
	return 0, w.err
}

func TestTraceWriterFailures(t *testing.T) {
	// KTD2: trace failures propagate and a failed required event does not fix the set.
	failure := errors.New("trace failure")
	for _, row := range []struct {
		name   string
		writer io.Writer
		want   error
	}{
		{"Write", traceFailure{err: failure}, failure},
		{"Short", traceFailure{short: true}, io.ErrShortWrite},
	} {
		t.Run(row.name, func(t *testing.T) {
			trace := NewTrace(row.writer)
			if err := trace.Required(array{}); !errors.Is(err, row.want) {
				t.Fatalf("error %v", err)
			}
			var output bytes.Buffer
			trace.writer = &output
			if err := trace.Required(array{}); err != nil {
				t.Fatal(err)
			}
			if output.String() != "{\"kind\":\"required\",\"value\":[]}\n" {
				t.Fatalf("trace %q", output.String())
			}
		})
	}
}

func TestAttributionSeam(t *testing.T) {
	// QT4, QT5, QH21: every recorded invocation is attributed, including all host overrides.
	cases, err := filepath.Glob(filepath.Join(kit, "cases", "*", "case.json"))
	if err != nil {
		t.Fatal(err)
	}
	overrides := map[string]bool{}
	forms := map[string]bool{}
	for _, path := range cases {
		c := decode(t, readFile(t, path))
		input := obj(c["input"])
		if input["host"] == nil {
			continue
		}
		forms[textValue(input["form"])] = true
		b := readFile(t, filepath.Join(kit, "behaviour", textValue(input["form"])+".json"))
		s, err := Read(strictjson.Write(input["host"]), b, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range arr(decode(t, b)["invocations"]) {
			i := obj(v)
			raw := obj(arr(i["confined"])[0])
			end := obj(raw["end"])
			markers := obj(raw["markers"])
			by := "exit"
			if end["kind"] == "signal" {
				by = textValue(end["by"])
				if by == "" {
					by = "signal"
				}
			}
			if markers["could_not_start"] == true {
				by = "could_not_start"
			}
			for _, v := range arr(obj(input["host"])["kill_attribution"]) {
				override := obj(v)
				if strictjson.EqualJSON(override["invocation"], i["key"]) {
					by = textValue(override["by"])
					overrides[textValue(c["id"])] = true
				}
			}
			if markers["confinement"] != true || obj(obj(input["host"])["confinement"])["applied"] != true {
				by = "unconfined"
			}
			r, err := s.Observe(map[string]any(obj(i["key"])), observe.Spec{})
			if err != nil {
				t.Fatal(err)
			}
			CheckAttribution(t, r, by)
		}
	}
	recordings, err := filepath.Glob(filepath.Join(kit, "behaviour", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(recordings) != 54 {
		t.Fatalf("recordings %d", len(recordings))
	}
	for _, path := range recordings {
		if !forms[strings.TrimSuffix(filepath.Base(path), ".json")] {
			t.Fatal("recording not attributed", filepath.Base(path))
		}
	}
	for _, id := range []string{"case-078", "case-079", "case-089"} {
		if !overrides[id] {
			t.Fatalf("missing override %s", id)
		}
	}
}

func TestContradictoryRecording(t *testing.T) {
	// QT5, E-31, QE16: a contradictory run anywhere invalidates the recording with exit 2.
	base := readFile(t, filepath.Join(kit, "examples", "behaviour-02.json"))
	if _, err := Read([]byte(passing), base, nil); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 6; index++ {
		for _, by := range []string{"confinement", "before_marker"} {
			t.Run(fmt.Sprintf("Run%d_%s", index, by), func(t *testing.T) {
				b := decode(t, base)
				i := obj(arr(b["invocations"])[0])
				runs := arr(i["confined"])
				var run object
				if index == 5 {
					b["violating"] = true
					for _, value := range arr(b["invocations"]) {
						inv := obj(value)
						inv["unconfined"] = decode(t, strictjson.Write(arr(inv["confined"])[0]))
					}
					run = obj(i["unconfined"])
				} else {
					run = obj(runs[index])
				}
				run["end"] = object{"kind": "signal", "signal": "SIGKILL", "by": by}
				obj(run["markers"])["confinement"] = by == "before_marker"
				_, err := Read([]byte(passing), strictjson.Write(b), nil)
				var rejection *Error
				if !errors.As(err, &rejection) || rejection.Code != 2 || rejection.Rule != "QT5" || rejection.Detail != "marker contradicts attribution" {
					t.Fatalf("%v", err)
				}
			})
		}
	}
}

func TestOutcomeLedgerCases(t *testing.T) {
	// QO2: case-095 proves the marker through the scripted source, check by check.
	// QO4: case-072 and case-073 prove the known-fast control through the scripted source.
	for _, id := range []string{"case-095", "case-072", "case-073"} {
		t.Run(strings.ReplaceAll(id, "-", "_"), func(t *testing.T) {
			c := decode(t, readFile(t, filepath.Join(kit, "cases", id, "case.json")))
			input := obj(c["input"])
			b := readFile(t, filepath.Join(kit, "behaviour", textValue(input["form"])+".json"))
			s, err := Read(strictjson.Write(input["host"]), b, nil)
			if err != nil {
				t.Fatal(err)
			}
			controls, err := s.Controls()
			if err != nil {
				t.Fatal(err)
			}
			preimages := decode(t, readFile(t, filepath.Join(kit, "cases", id, "preimages.json")))
			subject := decode(t, readFile(t, filepath.Join(kit, "cases", id, "subject.json")))
			evidence := decode(t, readFile(t, filepath.Join(kit, "cases", id, "input.json")))
			n := 0
			for _, value := range arr(evidence["evidence"]) {
				e := obj(value)
				pre := obj(preimages[textValue(e["check_id"])])
				var keys []plan.Object
				switch pre["probe"] {
				case "discovery":
					keys = append(keys, plan.DiscoveryKey(plan.Array{}))
				case "planted_secret":
					for _, v := range arr(obj(pre["parameters"])["commands"]) {
						keys = append(keys, plan.SubjectKey(subject, arr(obj(v)["words"]), "planted", nil, nil))
					}
				default:
					if pre["words"] != nil {
						home := "seed"
						if pre["runbook"] != nil {
							home = "runbook"
						}
						keys = append(keys, plan.SubjectKey(subject, arr(pre["words"]), home, pre["runbook"], pre["step"]))
					}
				}
				var runs []observe.Run
				for _, key := range keys {
					r, err := s.Observe(map[string]any(key), observe.Spec{})
					if err != nil {
						t.Fatal(err)
					}
					runs = append(runs, r)
				}
				t.Run(textValue(e["check_id"]), func(t *testing.T) {
					CheckInvocationDecision(t, pre, runs, controls, textValue(e["outcome"]), textValue(e["reason"]), textValue(e["failure_kind"]))
				})
				n++
			}
			if n == 0 {
				t.Fatal("no checks evaluated")
			}
		})
	}
}

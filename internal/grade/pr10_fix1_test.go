package grade

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/collect"
	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/judge"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/record"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

type fix1Source struct{ passingSource }

func (fix1Source) Observe(observe.Key, observe.Spec) (observe.Run, error) {
	raw := []byte("~~~???ab fn5-Pz8_YWI")
	return observe.Run{Markers: observe.Markers{Confinement: true}, Stdout: raw, StdoutSeen: int64(len(raw))}, nil
}
func TestFix1DeclaredRun(t *testing.T) {
	// QE18, QE10, KTD16: values mask once, while counts, digests and judge fields remain raw.
	o, d, saved := fixture(t)
	o.JSON = true
	data := strings.Replace(validSubject, `"offline":[]`, `"offline":[{"applies_to":[[]],"args":[],"env":{"SYNTHETIC":"~~~???ab","FIELD":"execution"}}]`, 1)
	data = strings.Replace(data, `"store_backed":false`, `"store_backed":true`, 1)
	data = strings.Replace(data, `"seed":null`, `"seed":"seed"`, 1)
	data = strings.Replace(data, `"store":null`, `"store":"~~~???ab"`, 1)
	seed := filepath.Join(filepath.Dir(o.SubjectFile), "seed")
	if err := os.Mkdir(seed, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seed, "~~~???ab"), []byte("seed data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(o.SubjectFile, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	d.Source = func() (observe.Source, func() error, error) { return fix1Source{}, func() error { return nil }, nil }
	d.Collect = func(p plan.Plan, s observe.Source) ([]collect.Check, error) {
		checks := []collect.Check{}
		for _, ch := range p.Checks {
			r, err := s.Observe(nil, observe.Spec{})
			if err != nil {
				return nil, err
			}
			var key map[string]any
			if len(ch.Keys) > 0 {
				key = map[string]any(ch.Keys[0])
			}
			checks = append(checks, collect.Check{Plan: ch, Result: decide.Result{Level: 8, Outcome: "held"}, Invocations: []observe.Invocation{{Key: key, Run: r}}})
		}
		return checks, nil
	}
	var judgeInput []byte
	calls := 0
	d.Judge = func(b []byte) ([]byte, int) {
		calls++
		if calls == 1 {
			return judge.Judge(b)
		}
		judgeInput = append([]byte(nil), b...)
		v, ok := strictjson.Read(b, strictjson.Bounds{Bytes: 16 << 20, Depth: 64, NumberChars: 32, ExponentDigits: 4}, nil)
		if !ok {
			t.Fatal("judge input invalid")
		}
		doc := subjectObject(v)
		ev := subjectObject(subjectArray(doc["evidence"])[0])
		pair := subjectObject(subjectArray(subjectObject(ev["observed"])["streams"])[0])
		head := subjectObject(pair["stdout"])["head"]
		return strictjson.Write(strictjson.Object{"exit_code": 0, "outcome": "graded", "dimension": "execution", "display": head}), 0
	}
	var out bytes.Buffer
	if code, err := Run(o, d, &out); err != nil || code != 0 {
		t.Fatalf("code %d err %v", code, err)
	}
	for name, b := range map[string][]byte{"Judge": judgeInput, "Record": strictjson.Write(saved.Document()), "Terminal": out.Bytes()} {
		if !bytes.Contains(b, []byte("******** ***********")) {
			t.Fatalf("%s missing exact masked output: %s", name, b)
		}
		for _, form := range record.EncodeForms("~~~???ab") {
			if bytes.Contains(b, []byte(form.Text)) {
				t.Fatalf("%s contains %s", name, form.Name)
			}
		}
	}
	if !bytes.Contains(out.Bytes(), []byte(`"dimension":"execution"`)) {
		t.Fatalf("judge field rewritten: %s", out.Bytes())
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte("~~~???ab fn5-Pz8_YWI")))
	if !bytes.Contains(judgeInput, []byte(`"sha256":"`+digest+`"`)) || !bytes.Contains(judgeInput, []byte(`"bytes":20`)) {
		t.Fatalf("raw measurement lost: %s", judgeInput)
	}
}
func TestFix1BuildMasksValue(t *testing.T) {
	// QE18: non-stream evidence is masked at the value boundary in Build.
	p := plan.Plan{Checks: []plan.Check{{ID: "check", Required: plan.Object{}, Preimage: plan.Object{}}}}
	doc, _, err := record.Build(record.Input{Plan: p, Results: map[string]record.Resolution{"check": {Result: decide.Result{Outcome: "held"}, Expected: "abcdefgh"}}, Masker: record.NewMasker(nil, []string{"abcdefgh"})})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(strictjson.Write(doc), []byte(`"expected":"********"`)) {
		t.Fatalf("unmasked value %s", strictjson.Write(doc))
	}
}
func TestFix1CauseBeforeEscape(t *testing.T) {
	// KTD16, QE18: terminal escaping follows value masking in the exit funnel.
	m := record.NewMasker(nil, []string{"abc\tdefgh"})
	e := maskedError{failure(2, "QS7", "abc\tdefgh"), m}
	if got := Escape(e.Error()); got != "rejected: QS7: *********" {
		t.Fatalf("error %q", got)
	}
}
func TestFix1RecordedControl7(t *testing.T) {
	// QE10, QE8: one exact stream pair per invocation; byte-bound expectation equals its bound.
	bound := strictjson.Number("13")
	ch := collect.Check{Plan: plan.Check{Preimage: plan.Object{"probe": "byte_bound", "parameters": plan.Object{"bound": bound}}}, Result: decide.Result{Outcome: "held"}, Invocations: []observe.Invocation{{Run: observe.Run{Stdout: []byte("abc"), StdoutSeen: 3}}}}
	r, err := checkResolution(ch, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Expected != bound {
		t.Fatal("expected bound lost")
	}
	ch.Plan.Preimage = plan.Object{}
	ch.Invocations = append(ch.Invocations, observe.Invocation{Run: observe.Run{Stderr: []byte("def"), StderrSeen: 3}})
	r, err = checkResolution(ch, nil)
	if err != nil {
		t.Fatal(err)
	}
	doc, _, err := record.Build(record.Input{Plan: plan.Plan{Checks: []plan.Check{{ID: "check", Required: plan.Object{}, Preimage: plan.Object{}}}}, Results: map[string]record.Resolution{"check": r}})
	if err != nil {
		t.Fatal(err)
	}
	ev := subjectObject(subjectArray(doc["evidence"])[0])
	pairs := subjectArray(subjectObject(ev["observed"])["streams"])
	if len(pairs) != 2 {
		t.Fatalf("pairs %v", pairs)
	}
	for _, v := range pairs {
		pair := subjectObject(v)
		if len(pair) != 2 {
			t.Fatal("pair keys")
		}
		for _, key := range []string{"stdout", "stderr"} {
			stream := subjectObject(pair[key])
			if len(stream) != 4 {
				t.Fatalf("stream %#v", stream)
			}
			for _, key := range []string{"bytes", "sha256", "cut", "head"} {
				if _, ok := stream[key]; !ok {
					t.Fatalf("missing %s", key)
				}
			}
		}
	}
}

package judge

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestPublishedInputsNeverDefect(t *testing.T) {
	inputs, err := filepath.Glob("../../docs/specs/judge-1.0/cases/*/input.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 383 {
		t.Fatalf("case count %d", len(inputs))
	}
	for _, path := range inputs {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		out, code, defect := Safe(Judge, data)
		if defect != nil {
			t.Fatalf("%s: %v", path, defect)
		}
		if len(out) == 0 || code < 0 || code > 5 {
			t.Fatalf("%s: no verdict document", path)
		}
	}
}
func TestInputOrderAndUnusedFields(t *testing.T) {
	f := conformanceFixture()
	for _, key := range []string{"required", "evidence"} {
		a := arr(f.in[key])
		for i, j := 0, len(a)-1; i < j; i, j = i+1, j-1 {
			a[i], a[j] = a[j], a[i]
		}
	}
	for _, v := range arr(f.in["evidence"]) {
		e := obj(v)
		e["expected"] = object{"large": number("1e9999"), "nested": array{true, nil}}
		e["observed"] = array{"unused"}
		e["failure_kind"] = "crashed"
		e["targets"] = array{object{"kind": "artifact", "id": "unused", "tier": "replay"}}
		e["evaluator"] = object{"kind": "model", "model": "unused"}
	}
	for _, v := range arr(f.in["required"]) {
		obj(v)["precondition"] = "unused"
	}
	assertFixture(t, f)
	p := steeringFixture()
	p.c("prompt")["condition_a"] = "unused"
	p.c("prompt")["record"] = nil
	for _, v := range arr(p.in["trials"]) {
		obj(v)["grade"] = "partial"
	}
	a := arr(p.in["trials"])
	for i, j := 0, len(a)-1; i < j; i, j = i+1, j-1 {
		a[i], a[j] = a[j], a[i]
	}
	assertFixture(t, p)
}
func TestSizeLimitInJudge(t *testing.T) {
	f := conformanceFixture()
	data := write(f.in)
	data = append(data, bytes.Repeat([]byte{' '}, MaxInput-len(data))...)
	assertBytes(t, data, f.want)
	data = append(data, ' ')
	assertBytes(t, data, rejectedExpected(X12))
}
func TestOutOfScopePromptRulesStillApply(t *testing.T) {
	f := conformanceFixture()
	c := required("prompt", "prompt", "steering")
	f.in["required"] = append(arr(f.in["required"]), c)
	f.want["checks"] = append(arr(f.want["checks"]), expectedCheck("prompt", "out_of_scope", "", nil))
	f.out("prompt")["rule"] = nil
	sortObjects(arr(f.want["checks"]), "id")
	v := trial("prompt", "", "old", 0)
	f.in["trials"] = array{v}
	f.want["set_aside"] = array{object{"check_id": "prompt", "arm": nil, "condition": "old", "trial_index": int64(0), "reason": "earlier_pool"}}
	f.want["earlier_pools"] = array{object{"check_id": "prompt", "arm": nil, "condition": "old", "k": int64(1), "n": int64(1), "rule": P1}}
	assertFixture(t, f)
	f.c("prompt")["declared_count"] = number("15")
	assertBytes(t, write(f.in), rejectedExpected(X7))
}
func TestSamplingEquivalentThroughJudge(t *testing.T) {
	f := compareFixture()
	for i, v := range arr(f.in["trials"]) {
		s := `{"b":[100,true,null],"a":-0}`
		if i%2 == 0 {
			s = `{"a":0.0,"b":[1e2,true,null]}`
		}
		value, ok := readDocument([]byte(s), nil)
		if !ok {
			t.Fatal("fixture")
		}
		obj(v)["sampling"] = value
	}
	assertFixture(t, f)
}
func TestIntegerOutputSpelling(t *testing.T) {
	f := conformanceFixture()
	e := baselineEntry("outside", "plain", "steering", "held")
	e["k"], e["n"] = number("0"), number("0")
	f.in["baseline"] = object{"entries": array{e}}
	entries := append(conformanceEntries(f), e)
	sortObjects(entries, "check_id")
	makeAnchorExpected(f, N7, nil, array{}, entries, array{})
	// Insert the spelling into raw bytes: write normalizes integer values.
	input := bytes.ReplaceAll(write(f.in), []byte(`"k":0`), []byte(`"k":-0`))
	input = bytes.ReplaceAll(input, []byte(`"n":0`), []byte(`"n":-0`))
	if !bytes.Contains(input, []byte(`"k":-0`)) || !bytes.Contains(input, []byte(`"n":-0`)) {
		t.Fatal("fixture did not contain negative zero")
	}
	assertBytes(t, input, f.want)
	out, code, defect := Safe(Judge, input)
	if defect != nil || code != 0 || bytes.Contains(out, []byte(`-0`)) || !bytes.Contains(out, []byte(`"k":0`)) || !bytes.Contains(out, []byte(`"n":0`)) {
		t.Fatalf("integer spelling: code=%d defect=%v output=%s", code, defect, out)
	}
	// Preserve coverage of large integer output separately from baseline carry.
	p := steeringFixture()
	p.in["trials"] = array{}
	p.c("prompt")["declared_count"] = number("9007199254740991")
	out, _, defect = Safe(Judge, write(p.in))
	if defect != nil || !bytes.Contains(out, []byte(`"to_run":9007199254740991`)) {
		t.Fatalf("maximum integer spelling: %s defect=%v", out, defect)
	}
}
func scaleInput(n int) []byte {
	in := baseInput()
	in["scope"] = array{"steering"}
	cs, ts, ms, es := array{}, array{}, array{}, array{}
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("check-%06d", i)
		c := required(id, "prompt", "steering")
		c["declared_count"] = number("16")
		c["condition"] = "current"
		cs = append(cs, c)
		ts = append(ts, trial(id, "", "current", 0))
		e := baselineEntry(id, "prompt", "steering", "held")
		es = append(es, e)
		// Marker records live on out-of-scope plain checks and remain subject to X4.
		mid := fmt.Sprintf("marker-%06d", i)
		cs = append(cs, required(mid, "plain", "execution"))
		ms = append(ms, object{"record": mid, "dimension": "execution"})
	}
	in["required"], in["trials"], in["markers"], in["baseline"] = cs, ts, ms, object{"entries": es}
	return write(in)
}
func TestScaling(t *testing.T) {
	small, large := scaleInput(1000), scaleInput(4000)

	measure := func(input []byte) uint64 {
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		out, code, err := Safe(Judge, input)
		runtime.ReadMemStats(&after)
		if err != nil || code != 4 || len(out) == 0 {
			t.Fatalf("scaling fixture: %d %v", code, err)
		}
		return after.TotalAlloc - before.TotalAlloc
	}
	s, l := measure(small), measure(large)
	ratio := float64(l) / float64(s)
	t.Logf("4x input: %.2fx allocated bytes", ratio)
	if ratio >= 8 {
		t.Fatalf("allocation ratio %.2f approaches quadratic growth", ratio)
	}
	t.Run("time", func(t *testing.T) {
		if testing.Short() {
			t.Skip("timing large inputs is omitted under -short")
		}
		large := scaleInput(16000)
		measureTime := func(input []byte) time.Duration {
			runtime.GC()
			start := time.Now()
			out, code, defect := Safe(Judge, input)
			elapsed := time.Since(start)
			if defect != nil || code != 4 || len(out) == 0 {
				t.Fatalf("timing fixture: code=%d defect=%v", code, defect)
			}
			return elapsed
		}
		// The scaling guard catches quadratic CPU work. Interleave inputs so
		// both see similar load, and alternate order to avoid a first-run bias.
		// Minima suppress pauses; require all independent attempts to exceed
		// the bound before failing on a shared, potentially busy machine.
		const attempts, rounds = 3, 10
		for attempt := 0; attempt < attempts; attempt++ {
			smallTime, largeTime := time.Duration(1<<63-1), time.Duration(1<<63-1)
			for round := 0; round < rounds; round++ {
				if round%2 == 0 {
					smallTime = min(smallTime, measureTime(small))
					largeTime = min(largeTime, measureTime(large))
				} else {
					largeTime = min(largeTime, measureTime(large))
					smallTime = min(smallTime, measureTime(small))
				}
			}
			ratio := float64(largeTime) / float64(smallTime)
			t.Logf("attempt %d: 16x input: %.2fx time (%s, %s)", attempt+1, ratio, smallTime, largeTime)
			if ratio <= 40 {
				return
			}
		}
		t.Fatalf("time ratio exceeded 40 in all %d attempts; approaches quadratic growth", attempts)
	})

	// A shared id must not cause each check to scan all matching trials.
	in := baseInput()
	in["scope"] = array{"steering"}
	for i := 0; i < 4000; i++ {
		c := required("shared", "prompt", "steering")
		c["declared_count"] = number("9007199254740991")
		c["condition"] = "current"
		in["required"] = append(arr(in["required"]), c)
		v := trial("shared", "", "current", i)
		v["declared_count"] = number("9007199254740991")
		in["trials"] = append(arr(in["trials"]), v)
	}
	assertBytes(t, write(in), rejectedExpected(X10))
}
func TestMoreReaderGrammar(t *testing.T) {
	for _, tt := range []struct {
		s  string
		ok bool
	}{{`[true,false,null,0,-0,1.2e+3]`, true}, {" \t\r\n{} \r\n", true}, {`"\b\f\n\r\t\/\\\""`, true}, {`"\u0000\uffff"`, true}, {`/*comment*/null`, false}, {`true//comment`, false}, {`[.1]`, false}, {`[-]`, false}, {`[1e+]`, false}, {`[1.e1]`, false}, {`[-01]`, false}, {`{"a" 1}`, false}, {`{a:1}`, false}, {"\"unterminated", false}, {"\"\xff\"", false}, {"\"\x80\"", false}, {"\"\xe2\x82\"", false}, {strings.Repeat(`{"x":`, 64) + `0` + strings.Repeat(`}`, 64), true}, {strings.Repeat(`{"x":`, 65) + `0` + strings.Repeat(`}`, 65), false}} {
		_, ok := readDocument([]byte(tt.s), nil)
		if ok != tt.ok {
			t.Fatalf("%q accepted=%v", tt.s[:min(50, len(tt.s))], ok)
		}
	}
}

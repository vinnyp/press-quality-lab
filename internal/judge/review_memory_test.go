package judge

import (
	"bytes"
	"fmt"
	"runtime"
	"testing"
)

func TestUnusedValuesMemory(t *testing.T) {
	f := conformanceFixture()
	// A flat array stresses discarded values without duplicate-key storage.
	// Use an unrestricted evidence field, leaving room for the complete request.
	input := write(f.in)
	needle := []byte(`"expected":null`)
	count := (MaxInput - len(input) - 2) / 2
	payload := append([]byte{'['}, bytes.Repeat([]byte("0,"), count)...)
	payload[len(payload)-1] = ']'
	input = bytes.Replace(input, needle, append([]byte(`"expected":`), payload...), 1)
	input = append(input, bytes.Repeat([]byte{' '}, MaxInput-len(input))...)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	output, code, defect := Safe(Judge, input)
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(input)
	if defect != nil || code != 0 || !bytes.Equal(output, write(f.want)) {
		t.Fatalf("large accepted input: code=%d defect=%v", code, defect)
	}
	// Total allocations also bound the peak heap attributable to this call.
	// This fails even if a materializing parser frees its tree before returning.
	allocated := after.TotalAlloc - before.TotalAlloc
	t.Logf("allocated %d bytes for %d input bytes", allocated, len(input))
	if allocated > 3*uint64(len(input)) {
		t.Fatalf("unused values allocated %d bytes for %d input bytes", allocated, len(input))
	}
}

func TestReviewDiscardedValuesStillValidate(t *testing.T) {
	f := conformanceFixture()
	f.e("execution")["expected"] = "payload"
	prefix := write(f.in)
	for _, row := range []struct {
		name, raw string
		accepted  bool
	}{
		{"arbitrary", `{"x":[true,null,-1e9999,"\ud834\udd1e"]}`, true},
		{"escaped_duplicate", `{"x":0,"\u0078":1}`, false},
		{"nested_duplicate", `[{"x":0,"x":1}]`, false},
		{"bad_surrogate", `["\ud800"]`, false},
		{"trailing_comma", `[0,]`, false},
		{"number_32", `-1234567890123456789012345678901`, true},
		{"number_33", `-12345678901234567890123456789012`, false},
		{"exponent_4", `1e9999`, true},
		{"exponent_5", `1e10000`, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			input := bytes.Replace(prefix, []byte(`"expected":"payload"`), append([]byte(`"expected":`), []byte(row.raw)...), 1)
			want := f.want
			if !row.accepted {
				want = rejectedExpected(X12)
			}
			assertBytes(t, input, want)
		})
	}
	for _, depth := range []int{64, 65} {
		t.Run(fmt.Sprintf("depth_%d", depth), func(t *testing.T) {
			// Root object, evidence array and evidence object account for three levels.
			raw := bytes.Repeat([]byte{'['}, depth-3)
			raw = append(raw, '0')
			raw = append(raw, bytes.Repeat([]byte{']'}, depth-3)...)
			input := bytes.Replace(prefix, []byte(`"expected":"payload"`), append([]byte(`"expected":`), raw...), 1)
			want := f.want
			if depth == 65 {
				want = rejectedExpected(X12)
			}
			assertBytes(t, input, want)
		})
	}
}

func TestReviewRetainedFields(t *testing.T) {
	f := conformanceFixture()
	f.e("execution")["expected"] = array{object{"nested": array{number("1"), true}}}
	v, ok := readDocument(write(f.in), inputShape)
	if !ok {
		t.Fatal("fixture rejected")
	}
	e := find(arr(obj(v)["evidence"]), "check_id", "execution")
	for _, field := range []string{"expected", "observed", "targets", "evaluator", "failure_kind"} {
		if e[field] != nil {
			t.Fatalf("unused field %s was retained", field)
		}
	}
	p := steeringFixture()
	obj(arr(p.in["trials"])[0])["grade"] = "full"
	obj(arr(p.in["trials"])[0])["sampling"] = object{"x": number("-0.5")}
	v, ok = readDocument(write(p.in), inputShape)
	if !ok {
		t.Fatal("sampling fixture rejected")
	}
	if obj(arr(obj(v)["trials"])[0])["grade"] != nil {
		t.Fatal("unused trial grade was retained")
	}
	if !equalJSON(obj(arr(obj(v)["trials"])[0])["sampling"], object{"x": number("-0.5")}) {
		t.Fatal("sampling must be retained for K2")
	}
}

func TestFreeStringMemory(t *testing.T) {
	for _, field := range []string{"precondition", "model", "expected"} {
		for _, encoding := range []string{"plain", "escaped"} {
			t.Run(field+"_"+encoding, func(t *testing.T) {
				f := conformanceFixture()
				// Free strings include discarded wildcard evidence; precondition notes presence.
				var node object
				if field == "precondition" {
					node = obj(arr(f.in["required"])[0])
				} else if field == "expected" {
					node = f.e("execution")
				} else {
					node = obj(f.e("execution")["evaluator"])
				}
				node[field] = "payload"
				// Establish the expected decision for the non-null precondition too.
				small := write(f.in)
				want, code, defect := Safe(Judge, small)
				if defect != nil || code != 0 {
					t.Fatalf("control code=%d defect=%v", code, defect)
				}
				size := MaxInput - len(small) + len("payload")
				unit := []byte("x")
				if encoding == "escaped" {
					unit = []byte(`xxxxxx\n`)
				}
				payload := bytes.Repeat(unit, size/len(unit))
				payload = append(payload, bytes.Repeat([]byte{'x'}, size%len(unit))...)
				input := bytes.Replace(small, []byte(`"payload"`), append(append([]byte{'"'}, payload...), '"'), 1)
				if len(input) != MaxInput {
					t.Fatalf("fixture size %d", len(input))
				}
				runtime.GC()
				var before, after runtime.MemStats
				runtime.ReadMemStats(&before)
				output, code, defect := Safe(Judge, input)
				runtime.ReadMemStats(&after)
				runtime.KeepAlive(input)
				if defect != nil || code != 0 || !bytes.Equal(output, want) {
					t.Fatalf("large string code=%d defect=%v", code, defect)
				}
				allocated := after.TotalAlloc - before.TotalAlloc
				t.Logf("allocated %d bytes for %d input bytes", allocated, len(input))
				if allocated > 2<<20 {
					t.Fatalf("free string allocated %d bytes", allocated)
				}
			})
		}
	}
}

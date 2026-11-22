package judge

import (
	"bytes"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"testing"
)

func TestReviewEarlierPoolsAcrossArms(t *testing.T) {
	f := compareFixture()
	// Two 16/16 prompt pools give aggregate arm bounds [0.892821, 1].
	f.out("compare")["d_lo"], f.out("compare")["d_hi"] = "-0.107179", "0.107179"
	c := clone(f.c("prompt"))
	obj(c)["id"] = "z-prompt"
	f.in["required"] = append(arr(f.in["required"]), c)
	o := clone(f.out("prompt"))
	obj(o)["id"] = "z-prompt"
	f.want["checks"] = append(arr(f.want["checks"]), o)
	for _, v := range append(array{}, arr(f.in["trials"])...) {
		tr := obj(clone(v))
		tr["check_id"] = "z-prompt"
		f.in["trials"] = append(arr(f.in["trials"]), tr)
	}
	// Input deliberately reverses all three sort keys, with B before A.
	for _, id := range []string{"z-prompt", "prompt"} {
		for _, arm := range []string{"B", "A"} {
			for _, condition := range []string{"old-z", "old-a"} {
				f.in["trials"] = append(array{trial(id, arm, condition, 0)}, arr(f.in["trials"])...)
			}
		}
	}
	// Put B's earlier trials first regardless of the current-pool input order.
	ts := arr(f.in["trials"])
	ordered := array{}
	for _, arm := range []string{"B", "A"} {
		for _, v := range ts {
			if obj(v)["arm"] == arm {
				ordered = append(ordered, v)
			}
		}
	}
	f.in["trials"] = ordered
	// Expected lists are specified independently in contract order.
	for _, id := range []string{"prompt", "z-prompt"} {
		for _, arm := range []string{"A", "B"} {
			for _, condition := range []string{"old-a", "old-z"} {
				f.want["set_aside"] = append(arr(f.want["set_aside"]), object{"check_id": id, "arm": arm, "condition": condition, "trial_index": int64(0), "reason": "earlier_pool"})
				f.want["earlier_pools"] = append(arr(f.want["earlier_pools"]), object{"check_id": id, "arm": arm, "condition": condition, "k": int64(1), "n": int64(1), "rule": P1})
			}
		}
	}
	assertFixture(t, f)
}

func TestReviewDepthAtSizeCap(t *testing.T) {
	// Limit the child test's stack so a late depth check fails safely and promptly.
	// The production reader still receives the entire cap-sized document via Judge.
	if os.Getenv("PQL_DEPTH_CHILD") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestReviewDepthAtSizeCap$", "-test.count=1")
		cmd.Env = append(os.Environ(), "PQL_DEPTH_CHILD=1")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("depth child failed: %v\n%.4000s", err, output)
		}
		return
	}
	debug.SetMaxStack(8 << 20)
	f := steeringFixture()
	input := write(f.in)
	needle := []byte(`"sampling":{}`)
	// Each {"":...} level costs five bytes including its closing brace.
	levels := (MaxInput - len(input) + 2 - 1) / 5
	raw := bytes.Repeat([]byte(`{"":`), levels)
	raw = append(raw, '0')
	raw = append(raw, bytes.Repeat([]byte{'}'}, levels)...)
	input = bytes.Replace(input, needle, append([]byte(`"sampling":`), raw...), 1)
	input = append(input, bytes.Repeat([]byte{' '}, MaxInput-len(input))...)
	if len(input) != MaxInput || levels < 3000000 {
		t.Fatal("fixture does not reach size cap")
	}
	assertBytes(t, input, rejectedExpected(X12))
}

func TestUnusedObjectKeysMemory(t *testing.T) {
	f := conformanceFixture()
	input := write(f.in)
	needle := []byte(`"expected":null`)
	budget := MaxInput - len(input) + len("null")
	// A discarded object must retain decoded keys to detect duplicates. Unlike
	// a flat array, millions of distinct keys therefore need substantial memory.
	raw := make([]byte, 1, budget)
	raw[0] = '{'
	for i := 0; ; i++ {
		key := strconv.Itoa(i)
		extra := len(key) + 5 // quotes, colon, zero, comma (or closing brace)
		if len(raw)+extra > budget {
			break
		}
		if len(raw) > 1 {
			raw = append(raw, ',')
		}
		raw = append(raw, '"')
		raw = append(raw, key...)
		raw = append(raw, '"', ':', '0')
	}
	raw = append(raw, '}')
	input = bytes.Replace(input, needle, append([]byte(`"expected":`), raw...), 1)
	input = append(input, bytes.Repeat([]byte{' '}, MaxInput-len(input))...)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	output, code, defect := Safe(Judge, input)
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(input)
	if defect != nil || code != 0 || !bytes.Equal(output, write(f.want)) {
		t.Fatalf("large object: code=%d defect=%v", code, defect)
	}
	allocated := after.TotalAlloc - before.TotalAlloc
	t.Logf("distinct keys allocated %d bytes for %d input bytes", allocated, len(input))
	// Allow duplicate-key bookkeeping and map growth; retaining the values as
	// another full object exceeds this bound. Total allocation bounds peak heap.
	if allocated > 16*uint64(len(input)) {
		t.Fatalf("unused object allocated %d bytes for %d input bytes", allocated, len(input))
	}
}

func TestReviewAddressableRowNames(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^(TestEverySchemaBoundary|TestSamplingValuesThroughWholeDocuments)$", "-test.v", "-test.count=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("row tests failed: %v\n%s", err, output)
	}
	rows := 0
	allowed := regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	for _, line := range bytes.Split(output, []byte{'\n'}) {
		if !bytes.HasPrefix(line, []byte("=== RUN   ")) {
			continue
		}
		parts := bytes.SplitN(line, []byte{'/'}, 2)
		if len(parts) != 2 {
			continue
		}
		name := string(parts[1])
		if name == "" || !allowed.MatchString(name) {
			t.Fatalf("row name is not directly addressable: %q", name)
		}
		rows++
	}
	if rows < 100 {
		t.Fatalf("only %d rows inspected", rows)
	}
}

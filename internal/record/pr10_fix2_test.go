package record

import (
	"bytes"
	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"strings"
	"testing"
	"time"
)

func TestFix2Periodic(t *testing.T) {
	// QE18, E-44: a complete periodic match survives a cut display head.
	value := "x" + strings.Repeat("a", 4000)
	m := NewMasker(nil, []string{"aaaaaaaa"})
	got := Stream(nil, 2<<20, true, observe.StreamDetails{Count: "2097152", Head: value}, m)
	if got["head"] != "x"+strings.Repeat("*", 4000) {
		t.Fatalf("head %v", got["head"])
	}
}
func TestFix2BoundedWriter(t *testing.T) {
	// QE18, E-44: self-overlapping matches consume bounded hold-back space.
	m := NewMasker(nil, []string{"aaaaaaaa"})
	var out bytes.Buffer
	w := m.Writer(&out)
	start := time.Now()
	for i := 0; i < 256; i++ {
		if _, err := w.Write(bytes.Repeat([]byte("a"), 4096)); err != nil {
			t.Fatal(err)
		}
		if len(w.pending) > 24 {
			t.Fatalf("pending %d", len(w.pending))
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("1 MiB exceeded five seconds")
	}
	if out.String() != strings.Repeat("*", 1<<20) {
		t.Fatal("masked run lost")
	}
}
func TestFix2Collision(t *testing.T) {
	// QE10, QE18: a masked key collision faults rather than losing a member.
	p := plan.Plan{Checks: []plan.Check{{ID: "check", Required: plan.Object{}, Preimage: plan.Object{}}}}
	_, _, err := Build(Input{Plan: p, Results: map[string]Resolution{"check": {Result: decide.Result{Outcome: "held"}, Expected: Object{"abcdefgh": 1, "ijklmnop": 2}}}, Masker: NewMasker(nil, []string{"abcdefgh", "ijklmnop"})})
	if err == nil || err.Error() != "QE10: record cannot be built" {
		t.Fatalf("collision %v", err)
	}
}
func TestFix2ArrayCopy(t *testing.T) {
	// QE18: masking never mutates raw evidence arrays.
	raw := Array{"abcdefgh"}
	got, ok := maskValue(raw, NewMasker(nil, []string{"abcdefgh"})).(Array)
	if !ok {
		t.Fatal("masked array missing")
	}
	if raw[0] != "abcdefgh" || got[0] != "********" {
		t.Fatalf("raw %v masked %v", raw, got)
	}
}
func TestFix2RawWriter(t *testing.T) {
	// QE18: pending diagnostic text precedes an already masked document.
	var out bytes.Buffer
	w := NewMasker(nil, []string{"abcdefgh"}).Writer(&out)
	if _, err := w.Write([]byte("abc")); err != nil {
		t.Fatal(err)
	}
	if _, err := RawWriter(w).Write([]byte("{}")); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "abc{}" {
		t.Fatalf("order %q", out.String())
	}
}

func TestFix2TailCoverage(t *testing.T) {
	// QE18, E-44: a covered prefix stays masked when a different partial match never completes.
	m := NewMasker(nil, []string{"abcdefgh", "ghijklmn"})
	var out bytes.Buffer
	w := m.Writer(&out)
	for _, s := range []string{"abcdefgh", "!"} {
		if _, err := w.Write([]byte(s)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "********!" {
		t.Fatalf("tail %q", out.String())
	}
}

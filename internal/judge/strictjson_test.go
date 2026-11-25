package judge

import (
	"bytes"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func TestStrictLeafMatchesJudge(t *testing.T) {
	for _, raw := range []string{`{"x":1,"x":2}`, `"\ud800"`, strings.Repeat("[", 65) + strings.Repeat("]", 65), `0 true`, "0" + strings.Repeat(" ", MaxInput)} {
		if _, ok := strictjson.Read([]byte(raw), contractBounds, nil); ok {
			t.Fatal("leaf accepted")
		}
		assertBytes(t, []byte(raw), rejectedExpected(X12))
	}
	f := conformanceFixture()
	f.e("execution")["expected"] = "payload"
	prefix := write(f.in)
	for _, row := range []struct{ name, raw string }{
		{"depth_at", strings.Repeat("[", 61) + "0" + strings.Repeat("]", 61)},
		{"number_at", strings.Repeat("1", 32)}, {"exponent_at", "1e9999"},
	} {
		t.Run(row.name, func(t *testing.T) {
			input := bytes.Replace(prefix, []byte(`"expected":"payload"`), []byte(`"expected":`+row.raw), 1)
			if _, ok := strictjson.Read(input, contractBounds, nil); !ok {
				t.Fatal("leaf rejected at limit")
			}
			assertBytes(t, input, f.want)
		})
	}
	t.Run("bytes_at", func(t *testing.T) {
		input := append(prefix, bytes.Repeat([]byte{' '}, MaxInput-len(prefix))...)
		if _, ok := strictjson.Read(input, contractBounds, nil); !ok {
			t.Fatal("leaf rejected byte limit")
		}
		assertBytes(t, input, f.want)
	})
	for _, row := range []struct {
		name  string
		input []byte
	}{
		{"bad_escape", bytes.Replace(prefix, []byte(`"expected":"payload"`), []byte(`"expected":"\x"`), 1)},
		{"unknown_key_same_count", bytes.Replace(prefix, []byte(`"expected":`), []byte(`"unexpected":`), 1)},
	} {
		t.Run(row.name, func(t *testing.T) { assertBytes(t, row.input, rejectedExpected(X12)) })
	}

}
func TestExportedDimensionTables(t *testing.T) {
	// Rulebook §3 scope rows and §6's five structural exclusions, as literal names.
	conformance := strings.Fields("availability execution graceful-failure robustness response-correctness determinism provenance-honesty completeness-signaling output-conformance flag-conformance bounding discoverability error-message-quality token-economy side-effect-integrity idempotency pii-secret-hygiene")
	all := strings.Fields("availability execution graceful-failure robustness response-correctness determinism cli-mcp-parity provenance-honesty completeness-signaling output-conformance flag-conformance bounding discoverability error-message-quality token-economy side-effect-integrity idempotency destructive-guard pii-secret-hygiene steering tool-description-efficacy")
	exclusions := strings.Fields("bounding idempotency destructive-guard side-effect-integrity flag-conformance")
	contains := func(names []string, s string) bool {
		for _, n := range names {
			if n == s {
				return true
			}
		}
		return false
	}
	for _, d := range all {
		if AlwaysApplicable(Dimension(d)) == contains(exclusions, d) {
			t.Fatalf("applicability %s", d)
		}
		for _, s := range []Scope{ConformanceScope, DestructiveScope, SteeringScope, CompareScope} {
			want := s == ConformanceScope && contains(conformance, d) || s == DestructiveScope && d == "destructive-guard" || s == SteeringScope && d == "steering" || s == CompareScope && d == "tool-description-efficacy"
			if ScopeIncludes(s, Dimension(d)) != want {
				t.Fatalf("scope %s dimension %s", s, d)
			}
		}
	}
}
func TestAnchorPromptNegativeZero(t *testing.T) {
	f := conformanceFixture()
	e := baselineEntry("outside", "prompt", "steering", "held")
	e["condition"] = "old"
	e["k"] = number("0")
	e["n"] = number("16")
	f.in["baseline"] = object{"entries": array{e}}
	entries := append(conformanceEntries(f), e)
	sortObjects(entries, "check_id")
	makeAnchorExpected(f, N7, nil, array{}, entries, array{})
	input := bytes.ReplaceAll(write(f.in), []byte(`"k":0`), []byte(`"k":-0`))
	if !bytes.Contains(input, []byte(`"k":-0`)) {
		t.Fatal("fixture")
	}
	assertBytes(t, input, f.want)
	out, code, defect := Safe(Judge, input)
	if defect != nil || code != 0 || !bytes.Contains(out, []byte(`"k":0`)) || bytes.Contains(out, []byte(`"k":-0`)) {
		t.Fatalf("output %s code %d defect %v", out, code, defect)
	}
}

func TestUnknownScopePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("unknown scope did not panic")
		}
	}()
	ScopeIncludes(Scope("unknown"), Availability)
}

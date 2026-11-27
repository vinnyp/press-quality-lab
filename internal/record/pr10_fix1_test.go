package record

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/vinnyp/press-quality-lab/internal/observe"
)

func TestFix1Overlaps(t *testing.T) {
	// QE18, E-44: every covered position is masked, including across writes.
	canary := "pqlcanary0123456789abcdef0123456789abcdef"
	for _, row := range []struct{ name, a, b, text, want string }{
		{"Canary", "mysecretp", canary, "mysecret" + canary, "********pqlcanary" + strings.Repeat("0", 32)},
		{"Tail", "abcdefgh", "ghijklmn", "abcdefghijklmn", strings.Repeat("*", 14)},
		{"Prefix", "prefix12", "12suffixABCD", "prefix12suffixABCD", strings.Repeat("*", 18)},
		{"Longer", "abcdefgh", "abcdefghij", "abcdefghij", strings.Repeat("*", 10)},
	} {
		t.Run(row.name, func(t *testing.T) {
			m := NewMasker(nil, []string{row.a})
			if row.b == canary {
				m.AddCanary(canary)
			} else {
				m.AddDeclared(row.b)
			}
			if got := m.Mask(row.text); got != row.want {
				t.Fatalf("got %q want %q", got, row.want)
			}
			for split := 0; split <= len(row.text); split++ {
				var out bytes.Buffer
				w := m.Writer(&out)
				if _, err := w.Write([]byte(row.text[:split])); err != nil {
					t.Fatal(err)
				}
				if _, err := w.Write([]byte(row.text[split:])); err != nil {
					t.Fatal(err)
				}
				if err := w.Flush(); err != nil {
					t.Fatal(err)
				}
				if out.String() != row.want {
					t.Fatalf("split %d: %q", split, out.String())
				}
			}
		})
	}
}
func TestFix1RecordedPartial(t *testing.T) {
	// QE18, QT5: an incomplete value at a cut head must not be displayed.
	m := NewMasker(nil, []string{"abcdefgh"})
	got := Stream(nil, 2<<20, true, observe.StreamDetails{Count: "2097152", Digest: strings.Repeat("a", 64), Head: "safe abcde"}, m)
	if got["head"] != "safe " {
		t.Fatalf("head %q", got["head"])
	}
}
func TestFix1Lookalike(t *testing.T) {
	// QE18: application maps, including their keys, are data regardless of shape.
	lookalike := Object{"bytes": 8, "sha256": "abcdefgh", "cut": false, "head": "abcdefgh"}
	if got, ok := maskValue(lookalike, NewMasker(nil, []string{"abcdefgh"})).(Object); !ok || got["head"] != "********" {
		t.Fatal("shape bypass")
	}
	v := Object{"bytes": 8, "sha256": "abcdefgh", "cut": false, "head": "abcdefgh", "abcdefgh": "abcdefgh"}
	got, ok := maskValue(v, NewMasker(nil, []string{"abcdefgh"})).(Object)
	if !ok {
		t.Fatal("missing masked map")
	}
	if got["sha256"] != "********" || got["head"] != "********" || got["********"] != "********" {
		t.Fatalf("unmasked map: %#v", got)
	}
}
func TestFix1LiteralEncodings(t *testing.T) {
	// QE18, QP12: literal encoder oracles exercise both base64 alphabets.
	for _, row := range []struct {
		value string
		want  []string
	}{
		{"~~~???ab", []string{"fn5+Pz8/YWI=", "fn5+Pz8/YWI", "fn5-Pz8_YWI=", "fn5-Pz8_YWI"}},
		{"pqlcanary0123456789abcdef0123456789abcdef", []string{"cHFsY2FuYXJ5MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=", "cHFsY2FuYXJ5MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY", "cHFsY2FuYXJ5MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=", "cHFsY2FuYXJ5MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY"}},
	} {
		forms := EncodeForms(row.value)
		for i, want := range row.want {
			if forms[i+1].Text != want {
				t.Fatalf("form %d: %q", i, forms[i+1].Text)
			}
		}
	}
}

func TestFix1MaskingIndex(t *testing.T) {
	// QE18: declared sets must not multiply the cost of scanning unrelated bytes.
	m := NewMasker(nil, nil)
	for i := 0; i < 1000; i++ {
		m.AddDeclared(fmt.Sprintf("declared_value_%08d", i))
	}
	input := strings.Repeat("!", 128<<10)
	start := time.Now()
	if m.Mask(input) != input {
		t.Fatal("unrelated input changed")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("unrelated bytes exceeded two seconds")
	}
}

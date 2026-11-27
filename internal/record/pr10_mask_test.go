package record

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func referenceForms(s string) []string {
	b := []byte(s)
	upper, lower := "", ""
	for _, c := range b {
		upper += fmt.Sprintf("%%%02X", c)
		lower += fmt.Sprintf("%%%02x", c)
	}
	return []string{s, base64.StdEncoding.EncodeToString(b), base64.RawStdEncoding.EncodeToString(b), base64.URLEncoding.EncodeToString(b), base64.RawURLEncoding.EncodeToString(b), hex.EncodeToString(b), strings.ToUpper(hex.EncodeToString(b)), upper, lower}
}

func TestPR10Encoder(t *testing.T) {
	// QE18, QP12: credit-and-routes section 4, independent library reference.
	for _, v := range []string{"pqlcanary0123456789abcdef0123456789abcdef", "~~~???ab"} {
		want := referenceForms(v)
		forms := EncodeForms(v)
		names := []string{"plain", "base64", "base64-nopad", "base64url", "base64url-nopad", "hex-lower", "hex-upper", "percent-upper", "percent-lower"}
		if len(forms) != 9 {
			t.Fatalf("forms %d", len(forms))
		}
		for i, got := range forms {
			if got.Name != names[i] {
				t.Fatalf("name %q", got.Name)
			}
			t.Run(fmt.Sprintf("Form%d_Value%d", i, len(v)), func(t *testing.T) {
				if got.Text != want[i] {
					t.Fatalf("got %q want %q", got.Text, want[i])
				}
			})
		}
	}
}

func TestPR10Masking(t *testing.T) {
	// QE18, E-39: mask whole forms before the head and kept-byte cuts.
	canary := "pqlcanary0123456789abcdef0123456789abcdef"
	for n, v := range []string{canary, "~~~???ab", "éééé", "1234567"} {
		for i, form := range referenceForms(v) {
			t.Run(fmt.Sprintf("Value%d_Form%d", n, i), func(t *testing.T) {
				m := NewMasker([]string{canary}, []string{v})
				replacement := strings.Repeat("*", utf8.RuneCountInString(form))
				if n == 0 {
					replacement = referenceForms("pqlcanary" + strings.Repeat("0", 32))[i]
				}
				if n == 3 {
					replacement = form
				}
				raw := "[" + form + "]"
				if !strings.Contains(raw, form) {
					t.Fatal("injection absent")
				}
				if got := m.Mask(raw); got != "["+replacement+"]" {
					t.Fatalf("got %q want %q", got, replacement)
				}
				for _, edge := range []int{64 << 10, 1 << 20} {
					prefix := strings.Repeat("!", edge-len(form)/2)
					raw := prefix + form + "!"
					if !strings.Contains(raw, form) {
						t.Fatal("edge injection absent")
					}
					got := m.Mask(raw)
					if got != prefix+replacement+"!" {
						t.Fatal("edge replacement differs")
					}
				}
				for split := 0; split <= len(form); split++ {
					var out bytes.Buffer
					w := m.Writer(&out)
					if _, err := w.Write([]byte(form[:split])); err != nil {
						t.Fatal(err)
					}
					if _, err := w.Write([]byte(form[split:])); err != nil {
						t.Fatal(err)
					}
					if err := w.Flush(); err != nil {
						t.Fatal(err)
					}
					if out.String() != replacement {
						t.Fatalf("split %d: %q want %q", split, out.String(), replacement)
					}
				}
			})
		}
	}
}

func TestPR10MaskErrors(t *testing.T) {
	// QE18: a failed output write never becomes successful masking evidence.
	m := NewMasker(nil, []string{"12345678"})
	w := m.Writer(shortMaskWriter{})
	if _, err := w.Write([]byte("unrelated")); err == nil {
		t.Fatal("short write accepted")
	}
	if err := w.Flush(); err == nil {
		t.Fatal("write error lost")
	}
}

type shortMaskWriter struct{}

func (shortMaskWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

func TestPR10RecordText(t *testing.T) {
	// QE18: display paths and computation diagnostics mask; facts do not change.
	value := "not_held"
	m := NewMasker(nil, []string{value})
	ev := Object{"expected": Object{"stderr": value}, "observed": Object{"home_changes": Array{Object{"path": "home/" + value}}}, "targets": Array{Object{"kind": "artifact", "id": "home/" + value}}, "outcome": value, "failure_kind": "predicate", "sha256": value, "bytes": 8}
	raw := Object{"evidence": Array{ev}}
	if object(ev["expected"])["stderr"] != value {
		t.Fatal("missing injection")
	}
	maskText(raw, m)
	want := Object{"expected": Object{"stderr": "********"}, "observed": Object{"home_changes": Array{Object{"path": "home/********"}}}, "targets": Array{Object{"kind": "artifact", "id": "home/********"}}, "outcome": value, "failure_kind": "predicate", "sha256": value, "bytes": 8}
	if !reflect.DeepEqual(ev, want) {
		t.Fatalf("got %#v", ev)
	}
}

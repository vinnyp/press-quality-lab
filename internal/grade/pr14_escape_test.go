package grade

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/vinnyp/press-quality-lab/internal/scorecard"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func TestPR14AllCodePoints(t *testing.T) {
	// KTD16: text and JSON escape the entire terminal-unsafe class set, and JSON round-trips.
	unsafe := func(r rune) bool {
		return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == 0x2028 || r == 0x2029 || r >= 0xe0000 && r <= 0xe007f
	}
	for r := rune(0); r <= utf8.MaxRune; r++ {
		if !utf8.ValidRune(r) {
			continue
		}
		value := string(r)
		escaped := Escape(value)
		if unsafe(r) {
			if escaped != fmt.Sprintf("\\u{%04x}", r) {
				t.Fatalf("text U+%04X: %q", r, escaped)
			}
		} else if escaped != value {
			t.Fatalf("safe U+%04X changed", r)
		}
		raw := terminalJSON(strictjson.Write(value))
		// The final document newline is structural, not a subject value.
		for _, v := range strings.TrimSuffix(string(raw), "\n") {
			if unsafe(v) {
				t.Fatalf("JSON raw U+%04X", v)
			}
		}
		var got string
		if e := json.Unmarshal(raw, &got); e != nil || got != value {
			t.Fatalf("JSON U+%04X: %q %v", r, got, e)
		}
	}
}
func TestPR14EscapedPlaces(t *testing.T) {
	// KTD16, QE18, QA4, QS7, QP7: each subject/stream position passes the common escaping path, with plain twins.
	for _, row := range []struct{ name, value string }{{"Plain", "P"}, {"C0", "\x01"}, {"C1", "\u0085"}, {"Cf", "\u200d"}, {"Line", "\u2028"}, {"Paragraph", "\u2029"}, {"Tag", "\U000e0001"}} {
		for _, place := range []string{"Words", "Runbook", "Step", "Computation", "Discovery", "Offline", "Bookkeeping", "Stderr", "Cleanup"} {
			t.Run(row.name+place, func(t *testing.T) {
				v := "before" + row.value + "after"
				inv := strictjson.Object{"words": strictjson.Array{"plain"}, "runbook": "book", "step": "step", "computation": strictjson.Array{"compute"}}
				ob := strictjson.Object{"probe": nil, "invocations": strictjson.Array{inv}, "streams": strictjson.Array{strictjson.Object{"stderr": strictjson.Object{"head": "plain", "cut": false}}}}
				doc := strictjson.Object{"request": "grade", "exit_code": 4, "judge_output": strictjson.Object{"verdict": strictjson.Object{"plain": "not_measured", "sub_label": "gaps"}, "checks": strictjson.Array{strictjson.Object{"id": "check", "result": "unobserved"}}}, "judge_input": strictjson.Object{"evidence": strictjson.Array{strictjson.Object{"check_id": "check", "observed": ob}}}}
				faults := []string{}
				switch place {
				case "Words":
					inv["words"] = strictjson.Array{v}
				case "Runbook":
					inv["runbook"] = v
				case "Step":
					inv["step"] = v
				case "Computation":
					inv["computation"] = strictjson.Array{v}
				case "Discovery":
					ob["discovery"] = strictjson.Object{"ran": true, "found_names": strictjson.Array{strictjson.Array{v}}}
				case "Offline":
					doc["offline_settings"] = strictjson.Array{strictjson.Object{"args": strictjson.Array{v}}}
				case "Bookkeeping":
					doc["bookkeeping_excluded"] = strictjson.Array{v}
				case "Stderr":
					subjectObject(subjectObject(subjectArray(ob["streams"])[0])["stderr"])["head"] = v
				case "Cleanup":
					faults = []string{"cleanup failed; directory left behind: " + v}
				}
				var b bytes.Buffer
				if e := scorecard.Render(&b, doc, nil, Escape, faults); e != nil {
					t.Fatal(e)
				}
				got := b.String()
				// Array fields are serialized as JSON before terminal escaping; their
				// C0 spelling is therefore JSON's escape, rather than Escape's spelling.
				want := Escape(v)
				if place == "Words" || place == "Computation" || place == "Discovery" || place == "Offline" || place == "Bookkeeping" {
					want = Escape(strings.Trim(string(strictjson.AppendValue(nil, v)), "\""))
					if place == "Bookkeeping" {
						want = Escape(v)
					}
				}
				if !strings.Contains(got, want) {
					t.Fatal("missing escaped value", want, got)
				}
				if row.name != "Plain" && strings.Contains(got, row.value) {
					t.Fatal("raw unsafe value", got)
				}
			})
		}
	}
}

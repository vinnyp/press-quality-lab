package scorecard

import (
	"bytes"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"strings"
	"testing"
)

func TestMissing(t *testing.T) {
	// QE14: absent fields are displayed as missing rather than derived.
	var b bytes.Buffer
	if err := Render(&b, nil, nil, func(s string) string { return s }, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "pql missing: missing (exit missing)") {
		t.Fatal(b.String())
	}
}
func TestAuthority(t *testing.T) {
	// QE14: render the document's words and counts even when they contradict the exit.
	doc := strictjson.Object{"request": "grade", "exit_code": 0, "judge_output": strictjson.Object{"verdict": strictjson.Object{"plain": "red", "sub_label": "gaps"}, "dimensions": strictjson.Array{strictjson.Object{"dimension": "execution", "state": "RED", "reason": nil}}, "counts": strictjson.Object{"xfail": 7, "xpass": 9}}}
	var b bytes.Buffer
	if err := Render(&b, doc, nil, func(s string) string { return s }, nil); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"pql grade: red (exit 0) (gaps)", "dimension execution: RED", "counts: xfail=7 xpass=9"} {
		if !strings.Contains(b.String(), want) {
			t.Fatal(want, b.String())
		}
	}
}

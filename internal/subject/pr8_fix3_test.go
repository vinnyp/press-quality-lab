package subject

import (
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func TestPR8BudgetSpelling(t *testing.T) {
	// QS2, QS6: integer overflow is a schema bound; decimal and exponent forms are not whole seconds.
	for _, row := range []struct{ name, value, rule string }{
		{"Overflow", "9223372036854775808", "QS2"},
		{"NegativeOverflow", "-99999999999999999999", "QS2"},
		{"LargeOverflow", "100000000000000000000", "QS2"},
		{"Exponent", "1e1", "QS6"},
		{"Decimal", "10.0", "QS6"},
	} {
		t.Run(row.name, func(t *testing.T) {
			s := parsedFixture(t)
			checkFixture(t, s, "")
			obj(arr(s["commands"])[0])["budget_seconds"] = strictjson.Number(row.value)
			checkFixture(t, s, row.rule)
		})
	}
}

func TestPR8CaptureGrammar(t *testing.T) {
	// QA4: only schema step names form capture tokens, including tokens inside literal prefixes.
	for _, row := range []struct{ name, value, rule string }{
		{"Brace", "{CAPTURE:a{b}", ""},
		{"Colon", "{CAPTURE:a:b}", ""},
		{"NestedSpace", "{CAPTURE:x {CAPTURE:b}", ""},
		{"NestedBrace", "{CAPTURE:{CAPTURE:b}}", ""},
		{"LongUnlisted", "{CAPTURE:" + strings.Repeat("a", 257) + "}", "QA4"},
		{"ControlLiteral", "{CAPTURE:a\nb}", ""},
		{"Unlisted", "{CAPTURE:missing}", "QA4"},
	} {
		t.Run(row.name, func(t *testing.T) {
			s := parsedFixture(t)
			checkFixture(t, s, "")
			book := obj(arr(s["runbooks"])[0])
			steps := arr(book["steps"])
			obj(steps[0])["name"] = "b"
			c := testComputation("/bin/echo")
			c["argv"] = strictjson.Array{"/bin/echo", row.value}
			c["captures"] = strictjson.Array{"b"}
			book["steps"] = append(steps, strictjson.Object{"name": "later", "words": obj(steps[0])["words"], "budget_seconds": strictjson.Number("1"), "checks": strictjson.Array{strictjson.Object{"dimension": "response-correctness", "predicate": "stdout_equals", "path": nil, "expected": strictjson.Object{"computation": c}, "precondition": nil, "exit_codes": nil}}})
			checkFixture(t, s, row.rule)
		})
	}
}

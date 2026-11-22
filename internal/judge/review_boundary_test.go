package judge

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// Every rejected fixture starts with an otherwise accepted request. In
// particular, malformed UTF-8 lives in an ignored string, so no schema or
// decision rule can mask a missing byte check.
func TestReviewRawByteBoundaryJudge(t *testing.T) {
	f := conformanceFixture()
	input := bytes.TrimSpace(write(f.in))
	check := func(t *testing.T, data []byte, accepted bool) {
		t.Helper()
		want := rejectedExpected(X12)
		if accepted {
			want = f.want
		}
		assertBytes(t, data, want)
	}
	t.Run("unadorned", func(t *testing.T) { check(t, input, true) })
	for _, space := range []byte{' ', '\t', '\r', '\n'} {
		t.Run(fmt.Sprintf("whitespace_%02x", space), func(t *testing.T) {
			check(t, append([]byte{space}, input...), true)
			check(t, append(bytes.Clone(input), space), true)
		})
	}
	for _, row := range []struct{ name, raw string }{
		{"empty", ""},
		{"whitespace_only", " \t\r\n"},
		{"leading_bom", "\xef\xbb\xbf" + string(input)},
		{"leading_comment", "/* comment */" + string(input)},
		{"trailing_comment", string(input) + "/* comment */"},
		{"trailing_junk", string(input) + "x"},
		{"second_document", string(input) + " {}"},
	} {
		t.Run(row.name, func(t *testing.T) { check(t, []byte(row.raw), false) })
	}
	for _, row := range []struct{ name, raw string }{
		{"vertical_tab", "\v"}, {"form_feed", "\f"}, {"nul", "\x00"},
		{"nonbreaking_space", "\u00a0"}, {"unicode_space", "\u2003"},
	} {
		t.Run("leading_"+row.name, func(t *testing.T) {
			check(t, append([]byte(row.raw), input...), false)
		})
		t.Run("trailing_"+row.name, func(t *testing.T) {
			check(t, append(bytes.Clone(input), []byte(row.raw)...), false)
		})
	}
	for _, size := range []int{MaxInput - 1, MaxInput, MaxInput + 1} {
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			data := append(bytes.Clone(input), bytes.Repeat([]byte{' '}, size-len(input))...)
			check(t, data, size <= MaxInput)
		})
	}
	for _, row := range []struct {
		name, raw string
		accepted  bool
	}{
		{"utf8_replacement_character", "\xef\xbf\xbd", true},
		{"utf8_maximum", "\xf4\x8f\xbf\xbf", true},
		{"utf8_surrogate_low_edge", "\xed\xa0\x80", false},
		{"utf8_surrogate_high_edge", "\xed\xbf\xbf", false},
		{"utf8_overlong_two", "\xc0\xaf", false},
		{"utf8_overlong_three", "\xe0\x80\xaf", false},
		{"utf8_overlong_four", "\xf0\x80\x80\xaf", false},
		{"utf8_above_maximum", "\xf4\x90\x80\x80", false},
		{"utf8_invalid_lead", "\xff", false},
		{"utf8_stray_continuation", "\x80", false},
		{"utf8_truncated", "\xe2\x82", false},
	} {
		t.Run(row.name, func(t *testing.T) {
			data := bytes.Replace(input, []byte(`"expected":null`), []byte(`"expected":"`+row.raw+`"`), 1)
			check(t, data, row.accepted)
		})
	}
}

// Exercise grammar and text limits in a schema-permitted ignored value as well
// as directly in TestReader. Each bad value has just one cause of rejection.
func TestReviewGrammarThroughJudge(t *testing.T) {
	f := conformanceFixture()
	input := write(f.in)
	for _, row := range []struct {
		name, raw string
		accepted  bool
	}{
		{"nan", `NaN`, false}, {"infinity", `Infinity`, false},
		{"leading_plus", `+1`, false}, {"leading_zero", `01`, false},
		{"negative_leading_zero", `-01`, false},
		{"trailing_array_comma", `[1,]`, false}, {"trailing_object_comma", `{"x":1,}`, false},
		{"missing_colon", `{"x" 1}`, false}, {"unquoted_key", `{x:1}`, false},
		{"comment", `/* comment */null`, false},
		{"missing_integer", `.1`, false}, {"bare_minus", `-`, false},
		{"missing_fraction", `1.`, false}, {"missing_exponent", `1e`, false},
		{"missing_signed_exponent", `1e+`, false},
		{"unterminated_string", `"unterminated`, false},
		{"unknown_escape", `"\q"`, false}, {"unescaped_control", "\"\x01\"", false},
		{"unpaired_high_surrogate", `"\ud800"`, false},
		{"unpaired_low_surrogate", `"\udc00"`, false},
		{"high_then_non_low_surrogate", `"\ud800\u0041"`, false},
		{"surrogate_pair", `"\ud800\udc00"`, true},
		{"decoded_duplicate_key", `{"a":0,"\u0061":1}`, false},
		{"distinct_unnormalized_keys", `{"é":0,"e\u0301":1}`, true},
		{"number_32", strings.Repeat("1", 32), true},
		{"number_33", strings.Repeat("1", 33), false},
		{"negative_number_32", "-" + strings.Repeat("1", 31), true},
		{"negative_number_33", "-" + strings.Repeat("1", 32), false},
		{"exponent_4", `1e9999`, true}, {"exponent_5", `1e10000`, false},
		{"negative_exponent_4", `1e-9999`, true}, {"negative_exponent_5", `1e-10000`, false},
		{"zero_padded_exponent_5", `1e00001`, false},
		// Three container levels enclose expected: root, evidence array, evidence object.
		{"depth_64", strings.Repeat("[", 61) + "0" + strings.Repeat("]", 61), true},
		{"depth_65", strings.Repeat("[", 62) + strings.Repeat("]", 62), false},
	} {
		t.Run(row.name, func(t *testing.T) {
			data := bytes.Replace(input, []byte(`"expected":null`), []byte(`"expected":`+row.raw), 1)
			want := f.want
			if !row.accepted {
				want = rejectedExpected(X12)
			}
			assertBytes(t, data, want)
		})
	}
	for _, row := range []struct {
		name, raw string
		accepted  bool
	}{
		{"integer_max", "9007199254740991", true},
		{"integer_above_max", "9007199254740992", false},
		{"integer_negative", "-1", false}, {"integer_negative_zero", "-0", true},
		{"integer_fraction", "1.0", false}, {"integer_exponent", "1e0", false},
	} {
		t.Run(row.name, func(t *testing.T) {
			data := bytes.Replace(input, []byte(`"target_size":1`), []byte(`"target_size":`+row.raw), 1)
			want := f.want
			if !row.accepted {
				want = rejectedExpected(X12)
			}
			assertBytes(t, data, want)
		})
	}
}

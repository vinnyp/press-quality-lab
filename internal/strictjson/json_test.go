package strictjson

import (
	"bytes"
	"go/build"
	"strings"
	"testing"
)

func testBounds() Bounds {
	return Bounds{Bytes: 1 << 20, Depth: 64, NumberChars: 32, ExponentDigits: 4}
}
func TestReaderLimits(t *testing.T) {
	for _, row := range []struct {
		name, raw string
		ok        bool
	}{
		{"escape_x", `"\x"`, false},
		{"escape_a", `"\a"`, false},
		{"escape_quote", `"\'"`, false},
		{"escape_short_unicode", `"\u12"`, false},
		{"escape_bad_hex", `"\u12g4"`, false},
		{"escape_unterminated", `"\`, false},
		{"size_at", "0" + strings.Repeat(" ", (1<<20)-1), true},
		{"size_over", "0" + strings.Repeat(" ", 1<<20), false},
		{"duplicate", `{"x":0,"\u0078":1}`, false},
		{"surrogate", `"\ud800"`, false},
		{"low_surrogate", `"\udc00"`, false},
		{"pair", `"\ud834\udd1e"`, true},
		{"depth_at", strings.Repeat("[", 64) + "0" + strings.Repeat("]", 64), true},
		{"depth_over", strings.Repeat("[", 65) + strings.Repeat("]", 65), false},
		{"trailing", "0 true", false},
		{"number_at", strings.Repeat("1", 32), true},
		{"number_over", strings.Repeat("1", 33), false},
		{"exponent_at", "1e9999", true},
		{"exponent_over", "1e00001", false},
		{"utf8", string([]byte{34, 0xed, 0xa0, 0x80, 34}), false},
	} {
		t.Run(row.name, func(t *testing.T) {
			_, ok := Read([]byte(row.raw), testBounds(), nil)
			if ok != row.ok {
				t.Fatalf("accepted=%v want %v", ok, row.ok)
			}
		})
	}
	b := testBounds()
	b.Bytes = 1
	if _, ok := Read([]byte("0 "), b, nil); ok {
		t.Fatal("caller byte bound ignored")
	}
	b = testBounds()
	b.NumberChars = 1
	if _, ok := Read([]byte("10"), b, nil); ok {
		t.Fatal("caller number bound ignored")
	}
	b = testBounds()
	b.ExponentDigits = 1
	if _, ok := Read([]byte("1e10"), b, nil); ok {
		t.Fatal("caller exponent bound ignored")
	}
	b = testBounds()
	b.Depth = 1
	if _, ok := Read([]byte("[[]]"), b, nil); ok {
		t.Fatal("caller depth bound ignored")
	}
}
func TestExactEquality(t *testing.T) {
	for _, row := range []struct {
		a, b  string
		equal bool
	}{
		{"9007199254740993", "9007199254740992", false}, {"1", "1.0", true}, {"1", "1e0", true},
		{`{"x":[1,true,null]}`, `{"x":[1e0,true,null]}`, true}, {`{"x":null}`, `{"y":null}`, false},
		{"-0", "0", true}, {"1e9999", "10e9998", true},
	} {
		a, ok := Read([]byte(row.a), testBounds(), nil)
		if !ok {
			t.Fatal(row.a)
		}
		b, ok := Read([]byte(row.b), testBounds(), nil)
		if !ok {
			t.Fatal(row.b)
		}
		if EqualJSON(a, b) != row.equal {
			t.Fatalf("%s == %s", row.a, row.b)
		}
	}
	if Normalize(Number("1.0")) != "1e0" {
		t.Fatal("normal form")
	}
}
func TestWriter(t *testing.T) {
	type named string
	for _, row := range []struct {
		value any
		want  string
	}{
		{Number("123456789012345678901"), "123456789012345678901\n"},
		{Number("12345678901234567890123456789012"), "12345678901234567890123456789012\n"},
		{Number("-123456789012345678901"), "-123456789012345678901\n"},
		{Number("-12345678901234567890123456789012"), "-12345678901234567890123456789012\n"},
		{Number("-0"), "0\n"}, {Number("1.00e+2"), "1.00e+2\n"},
		{named("text"), "\"text\"\n"},
		{Object{"z": Array{true, nil}, "a": "\b\f\n\r\t\"\\"}, "{\"a\":\"\\u0008\\u000c\\n\\r\\t\\\"\\\\\",\"z\":[true,null]}\n"},
	} {
		got := Write(row.value)
		if !bytes.Equal(got, []byte(row.want)) {
			t.Fatalf("got %q want %q", got, row.want)
		}
		if Length(row.value) != len(row.want) {
			t.Fatal("length")
		}
		bounds := testBounds()
		bounds.NumberChars = 33 // A 32-digit negative integer includes its sign.
		v, ok := Read(got, bounds, nil)
		if !ok {
			t.Fatal("writer produced invalid JSON")
		}
		if n, ok := row.value.(Number); ok && !EqualJSON(n, v) {
			t.Fatal("round trip")
		}
	}
}

// A synthetic caller uses the same opaque policy for root and array nodes.
type retentionHook struct {
	mode      Mode
	arrayItem bool
}

func (h retentionHook) Retention() Mode                    { return h.mode }
func (h retentionHook) Materialize(keep bool, _ byte) bool { return keep }
func (h retentionHook) Valid(any) bool                     { return true }
func (h retentionHook) Container(byte) bool                { return true }
func (h retentionHook) Child(k string) (Hook, bool) {
	mode := Keep
	switch k {
	case "discard":
		mode = Discard
	case "note":
		mode = Note
	}
	return retentionHook{mode: mode}, true
}
func (h retentionHook) Item() Hook {
	if h.arrayItem {
		return retentionHook{mode: Note}
	}
	return nil
}
func (h retentionHook) End(int) bool { return true }
func TestRetentionHook(t *testing.T) {
	for _, row := range []struct {
		raw  string
		hook retentionHook
		want string
	}{
		{`{"keep":1,"discard":[0],"note":{"x":0}}`, retentionHook{}, `{"discard":null,"keep":1,"note":""}` + "\n"},
		{`[null,1,{}]`, retentionHook{arrayItem: true}, `[null,"",""]` + "\n"},
		{`{"x":1}`, retentionHook{mode: Discard}, "null\n"},
		{`0`, retentionHook{mode: Note}, "\"\"\n"},
		{`null`, retentionHook{mode: Note}, "null\n"},
	} {
		v, ok := Read([]byte(row.raw), testBounds(), row.hook)
		if !ok || string(Write(v)) != row.want {
			t.Fatalf("%s: got %q accepted=%v", row.raw, Write(v), ok)
		}
	}
	for _, raw := range []string{`{"discard":{"x":0,"x":1}}`, `{"note":"\ud800"}`} {
		if _, ok := Read([]byte(raw), testBounds(), retentionHook{}); ok {
			t.Fatal("discarded grammar accepted")
		}
	}
}

type contractHook struct {
	retentionHook
	reject string
}

func (h contractHook) Valid(any) bool                     { return h.reject != "valid" }
func (h contractHook) Container(byte) bool                { return h.reject != "container" }
func (h contractHook) End(int) bool                       { return h.reject != "end" }
func (h contractHook) Child(k string) (Hook, bool)        { return nil, k != "forbidden" }
func (h contractHook) Materialize(keep bool, _ byte) bool { return keep && h.reject != "materialize" }
func TestHookContract(t *testing.T) {
	for _, row := range []struct {
		name, raw string
		ok        bool
		want      string
	}{
		{"valid", "0", false, "null"}, {"container", "[]", false, "null"},
		{"end", "[]", false, "null"}, {"child", `{"forbidden":0}`, false, "null"},
		{"materialize", `"contents"`, true, `""`},
	} {
		t.Run(row.name, func(t *testing.T) {
			v, ok := Read([]byte(row.raw), testBounds(), contractHook{reject: row.name})
			if ok != row.ok || ok && string(AppendValue(nil, v)) != row.want {
				t.Fatalf("value=%v accepted=%v", v, ok)
			}
		})
	}
}
func TestInvalidConstructedInteger(t *testing.T) {
	for _, raw := range []string{"", "-", "+1", "01", "-00", "1x", "123456789012345678901x"} {
		t.Run(strings.ReplaceAll(strings.ReplaceAll(raw, "-", "minus"), "+", "plus")+"invalid", func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("invalid integer did not panic")
				}
			}()
			Write(Number(raw))
		})
	}
}
func TestStandardLibraryOnly(t *testing.T) {
	p, err := build.Default.ImportDir(".", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Imports) == 0 {
		t.Fatal("empty imports")
	}
	for _, name := range p.Imports {
		dep, err := build.Default.Import(name, ".", build.FindOnly)
		if err != nil {
			t.Fatal(err)
		}
		if !dep.Goroot {
			t.Errorf("non-standard dependency %s", name)
		}
	}
}

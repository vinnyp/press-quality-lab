package judge

import (
	"strings"
	"testing"
)

func TestReader(t *testing.T) {
	for _, tt := range []struct {
		s  string
		ok bool
	}{
		{`null`, true}, {`-0`, true}, {`1e9999`, true}, {`1e-9999`, true}, {`1e10000`, false}, {`1e00001`, false}, {`1e`, false}, {`1.`, false}, {`01`, false}, {`+1`, false}, {`NaN`, false}, {``, false}, {` `, false}, {"\xef\xbb\xbf{}", false}, {"\vnull", false}, {`null false`, false}, {`[1,]`, false}, {`{"a":1,}`, false}, {`{"a":0,"\u0061":1}`, false}, {`"\ud800"`, false}, {`"\udc00"`, false}, {`"\ud800\u0041"`, false}, {`"\ud800\udc00"`, true}, {"\"\x01\"", false}, {"\"\xed\xa0\x80\"", false}, {"\"\xc0\xaf\"", false}, {"\"\xf4\x90\x80\x80\"", false}, {`"\q"`, false},
		{strings.Repeat("[", 64) + "0" + strings.Repeat("]", 64), true}, {strings.Repeat("[", 65) + strings.Repeat("]", 65), false}, {strings.Repeat("[", 1000000), false}, {strings.Repeat("1", 32), true}, {strings.Repeat("1", 33), false}, {"-" + strings.Repeat("1", 31), true}, {"-" + strings.Repeat("1", 32), false}, {"0" + strings.Repeat(" ", MaxInput-1), true}, {"0" + strings.Repeat(" ", MaxInput), false},
	} {
		t.Run(tt.s[:min(len(tt.s), 40)], func(t *testing.T) {
			_, ok := readDocument([]byte(tt.s), nil)
			if ok != tt.ok {
				t.Fatalf("accepted=%v want %v", ok, tt.ok)
			}
		})
	}
}
func TestIntegerSchema(t *testing.T) {
	for _, tt := range []struct {
		s  string
		ok bool
	}{{"9007199254740991", true}, {"9007199254740992", false}, {"-0", true}, {"-1", false}, {"16.0", false}, {"1e1", false}} {
		v, ok := readDocument([]byte(tt.s), nil)
		if !ok || valid(v, scalar("i")) != tt.ok {
			t.Errorf("integer %s", tt.s)
		}
	}
}
func TestSafe(t *testing.T) {
	out, code, err := Safe(func([]byte) ([]byte, int) { panic("injected defect") }, nil)
	if out != nil || code != 70 || err == nil || !strings.Contains(err.Error(), "injected defect") || !strings.Contains(err.Error(), "goroutine") {
		t.Fatalf("%s %d %v", out, code, err)
	}
}
func TestValidatedAccessors(t *testing.T) {
	v, ok := readDocument([]byte(`{"a":["x",true,-0]}`), nil)
	if !ok {
		t.Fatal("fixture")
	}
	a := arr(obj(v)["a"])
	if str(a[0]) != "x" || !boolean(a[1]) || integer(a[2]) != 0 {
		t.Fatal("accessors")
	}
}
func TestWriterEscapes(t *testing.T) {
	got := string(write(object{"value": "\x00\b\f\t\r\n\"\\\u2028𐀀"}))
	want := "{\"value\":\"\\u0000\\u0008\\u000c\\t\\r\\n\\\"\\\\\u2028𐀀\"}\n"
	if got != want {
		t.Fatalf("writer: %q want %q", got, want)
	}
}

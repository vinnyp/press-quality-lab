// Package strictjson implements the shared strict JSON grammar and contract encoding.
package strictjson

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Object is a JSON object, indexed by decoded member names.
type Object map[string]any

// Array is a JSON array.
type Array []any

// Number retains a JSON number's original decimal spelling.
type Number string

// Bounds limits input bytes, container depth, number characters and exponent digits.
type Bounds struct{ Bytes, Depth, NumberChars, ExponentDigits int }

// Mode controls retention; Note retains only non-null presence as an empty string.
type Mode uint8

const (
	// Keep retains the decoded value.
	Keep Mode = iota
	// Discard validates without retaining the value.
	Discard
	// Note retains non-null presence as an empty string.
	Note
)

// Hook is opaque caller policy. It validates nodes while the reader owns grammar.
// Retention controls each node, including the root and array elements.
// Materialize requests decoded scalar contents, even for discarded values.
// Child selects object field policy; End validates the member or element count.
type Hook interface {
	// Materialize controls scalar decoding for the judge memory bound; false yields an empty string for a string token.
	Materialize(keep bool, token byte) bool
	// Valid accepts or rejects a decoded scalar.
	Valid(any) bool
	// Container accepts or rejects an opening container token.
	Container(byte) bool
	// Retention selects whether to keep, discard or note this node.
	Retention() Mode
	// Child selects a member hook; false rejects the member name.
	Child(string) (Hook, bool)
	// Item selects the hook for each array element.
	Item() Hook
	// End accepts or rejects the final member or element count.
	End(int) bool
}
type reader struct {
	data   []byte
	pos    int
	bad    bool
	bounds Bounds
}

// Read checks one document using the caller's bounds. A nil hook retains all values.
func Read(data []byte, bounds Bounds, hook Hook) (any, bool) {
	if len(data) > bounds.Bytes || !utf8.Valid(data) {
		return nil, false
	}
	r := reader{data: data, bounds: bounds}
	v := r.node(0, hook, true)
	r.space()
	return v, !r.bad && r.pos == len(data)
}
func (r *reader) space() {
	for r.pos < len(r.data) {
		c := r.data[r.pos]
		if c != ' ' && c != '\t' && c != '\r' && c != '\n' {
			break
		}
		r.pos++
	}
}
func (r *reader) take(c byte) bool {
	if r.pos < len(r.data) && r.data[r.pos] == c {
		r.pos++
		return true
	}
	return false
}
func (r *reader) node(depth int, s Hook, keep bool) (result any) {
	r.space()
	if r.bad || r.pos == len(r.data) {
		r.bad = true
		return nil
	}
	c := r.data[r.pos]
	if s != nil {
		mode := s.Retention()
		if keep && mode == Note && c != 'n' {
			defer func() { result = "" }()
		}
		keep = keep && mode == Keep
	}
	if c != '{' && c != '[' {
		materialize := keep
		if s != nil {
			materialize = s.Materialize(keep, c)
		}
		v := r.atom(materialize)

		if s != nil && !s.Valid(v) {
			r.bad = true
		}
		if !keep {
			return nil
		}
		return v
	}
	depth++
	if depth > r.bounds.Depth {
		r.bad = true
		return nil
	}
	r.pos++
	if s != nil && !s.Container(c) {
		r.bad = true
		return nil
	}
	if c == '{' {
		seen := map[string]bool{}
		var m Object
		if keep {
			m = Object{}
		}
		r.space()
		if !r.take('}') {
			for !r.bad {
				r.space()
				if r.pos == len(r.data) || r.data[r.pos] != '"' {
					r.bad = true
					break
				}
				k := r.string()
				if seen[k] {
					r.bad = true
					break
				}
				seen[k] = true
				var child Hook
				if s != nil {
					var ok bool
					child, ok = s.Child(k)
					if !ok {
						r.bad = true
						break
					}
				}
				r.space()
				if !r.take(':') {
					r.bad = true
					break
				}

				r.space()
				v := r.node(depth, child, keep)

				if keep {
					m[k] = v
				}
				r.space()
				if r.take('}') {
					break
				}
				if !r.take(',') {
					r.bad = true
					break
				}
			}
		}
		if s != nil && !s.End(len(seen)) {
			r.bad = true
		}
		if !keep {
			return nil
		}
		return m
	}
	var a Array
	if keep {
		a = Array{}
	}
	count := 0
	var item Hook
	if s != nil {
		item = s.Item()
	}
	r.space()
	if !r.take(']') {
		for !r.bad {
			v := r.node(depth, item, keep)
			count++
			if keep {
				a = append(a, v)
			}
			r.space()
			if r.take(']') {
				break
			}
			if !r.take(',') {
				r.bad = true
				break
			}
		}
	}
	if s != nil && !s.End(count) {
		r.bad = true
	}
	if !keep {
		return nil
	}
	return a
}
func (r *reader) atom(materialize bool) any {

	r.space()
	if r.bad || r.pos == len(r.data) {
		r.bad = true
		return nil
	}
	c := r.data[r.pos]
	if c == '"' {
		return r.text(materialize)
	}
	for _, lit := range []string{"true", "false", "null"} {
		if c == lit[0] {
			if len(r.data)-r.pos < len(lit) || string(r.data[r.pos:r.pos+len(lit)]) != lit {
				r.bad = true
				return nil
			}
			r.pos += len(lit)
			if lit == "null" {
				return nil
			}
			return lit == "true"
		}
	}
	start := r.pos
	r.take('-')
	if !r.take('0') {
		if r.pos == len(r.data) || r.data[r.pos] < '1' || r.data[r.pos] > '9' {
			r.bad = true
			return nil
		}
		for r.pos < len(r.data) && r.data[r.pos] >= '0' && r.data[r.pos] <= '9' {
			r.pos++
		}
	}
	if r.take('.') {
		p := r.pos
		for r.pos < len(r.data) && r.data[r.pos] >= '0' && r.data[r.pos] <= '9' {
			r.pos++
		}
		if p == r.pos {
			r.bad = true
		}
	}
	if r.take('e') || r.take('E') {
		if !r.take('+') {
			r.take('-')
		}
		p := r.pos
		for r.pos < len(r.data) && r.data[r.pos] >= '0' && r.data[r.pos] <= '9' {
			r.pos++
		}
		if r.pos == p || r.pos-p > r.bounds.ExponentDigits {
			r.bad = true
		}
	}
	if r.pos-start > r.bounds.NumberChars {
		r.bad = true
	}
	if !materialize {
		return nil
	}
	return Number(r.data[start:r.pos])
}
func (r *reader) hex() rune {
	var n rune
	for i := 0; i < 4; i++ {
		if r.pos == len(r.data) {
			r.bad = true
			return 0
		}
		c := r.data[r.pos]
		r.pos++
		n *= 16
		switch {
		case c >= '0' && c <= '9':
			n += rune(c - '0')
		case c >= 'a' && c <= 'f':
			n += rune(c - 'a' + 10)
		case c >= 'A' && c <= 'F':
			n += rune(c - 'A' + 10)
		default:
			r.bad = true
		}
	}
	return n
}
func (r *reader) string() string { return r.text(true) }
func (r *reader) text(materialize bool) string {
	r.pos++
	start := r.pos
	var b strings.Builder
	for r.pos < len(r.data) {
		c := r.data[r.pos]
		r.pos++
		if c == '"' {
			if !materialize {
				return ""
			}
			if b.Len() == 0 {
				return string(r.data[start : r.pos-1])
			}
			b.Write(r.data[start : r.pos-1])
			return b.String()
		}
		if c < 32 {
			r.bad = true
			return ""
		}
		if c != '\\' {
			continue
		}
		if materialize {
			b.Write(r.data[start : r.pos-1])
		}
		if r.pos == len(r.data) {
			break
		}
		c = r.data[r.pos]
		r.pos++
		switch c {
		case '"', '\\', '/':
			if materialize {
				b.WriteByte(c)
			}
		case 'b':
			if materialize {
				b.WriteByte('\b')
			}
		case 'f':
			if materialize {
				b.WriteByte('\f')
			}
		case 'n':
			if materialize {
				b.WriteByte('\n')
			}
		case 'r':
			if materialize {
				b.WriteByte('\r')
			}
		case 't':
			if materialize {
				b.WriteByte('\t')
			}
		case 'u':
			u := r.hex()
			if u >= 0xd800 && u <= 0xdbff {
				if !r.take('\\') || !r.take('u') {
					r.bad = true
					return ""
				}
				v := r.hex()
				if v < 0xdc00 || v > 0xdfff {
					r.bad = true
					return ""
				}
				u = utf16.DecodeRune(u, v)
			} else if u >= 0xdc00 && u <= 0xdfff {
				r.bad = true
				return ""
			}
			if materialize {
				b.WriteRune(u)
			}
		default:
			r.bad = true
			return ""
		}
		start = r.pos
	}
	r.bad = true
	return ""
}
func quote(dst []byte, s string) []byte {
	dst = append(dst, '"')
	for _, c := range s {
		switch c {
		case '"', '\\':
			dst = append(dst, '\\', byte(c))
		case '\n':
			dst = append(dst, '\\', 'n')
		case '\r':
			dst = append(dst, '\\', 'r')
		case '\t':
			dst = append(dst, '\\', 't')
		default:
			if c < 32 {
				dst = append(dst, fmt.Sprintf("\\u%04x", c)...)
			} else {
				dst = utf8.AppendRune(dst, c)
			}
		}
	}
	return append(dst, '"')
}

// Write encodes a valid value as a contract document with a final newline.
func Write(v any) []byte { return append(AppendValue(nil, v), '\n') }

// AppendValue appends a contract value without a newline. Object keys are sorted.
func AppendValue(dst []byte, v any) []byte {
	switch x := v.(type) {
	case nil:
		return append(dst, "null"...)
	case string:
		return quote(dst, x)
	case bool:
		return strconv.AppendBool(dst, x)
	case int:
		return strconv.AppendInt(dst, int64(x), 10)
	case int64:
		return strconv.AppendInt(dst, x, 10)
	case Number:
		if strings.ContainsAny(string(x), ".eE") {
			return append(dst, string(x)...)
		}
		digits := strings.TrimPrefix(string(x), "-")
		if digits == "" || len(digits) > 1 && digits[0] == '0' {
			panic("invalid integer Number")
		}
		for _, c := range digits {
			if c < '0' || c > '9' {
				panic("invalid integer Number")
			}
		}
		if i, err := strconv.ParseInt(string(x), 10, 64); err == nil {
			return strconv.AppendInt(dst, i, 10)
		}
		return append(dst, string(x)...)
	case Array:
		dst = append(dst, '[')
		for i, a := range x {
			if i > 0 {
				dst = append(dst, ',')
			}
			dst = AppendValue(dst, a)
		}
		return append(dst, ']')
	case Object:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		dst = append(dst, '{')
		for i, k := range keys {
			if i > 0 {
				dst = append(dst, ',')
			}
			dst = quote(dst, k)
			dst = append(dst, ':')
			dst = AppendValue(dst, x[k])
		}
		return append(dst, '}')
	default:
		if reflect.TypeOf(v).Kind() == reflect.String {
			return quote(dst, reflect.ValueOf(v).String())
		}
		panic(fmt.Sprintf("unwritable value %T", v))
	}
}

// Length returns the encoded document length, including its final newline.
func Length(v any) int { return len(Write(v)) }

// Normalize returns an exact decimal comparison key for a valid Number.
func Normalize(n Number) string {
	s := string(n)
	negative := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	exponent := 0
	if p := strings.IndexAny(s, "eE"); p >= 0 {
		var e error
		exponent, e = strconv.Atoi(s[p+1:])
		if e != nil {
			panic(e)
		}
		s = s[:p]
	}
	if p := strings.IndexByte(s, '.'); p >= 0 {
		exponent -= len(s) - p - 1
		s = s[:p] + s[p+1:]
	}
	s = strings.TrimLeft(s, "0")
	if s == "" {
		return "0"
	}
	trim := strings.TrimRight(s, "0")
	exponent += len(s) - len(trim)
	if negative {
		trim = "-" + trim
	}
	return trim + "e" + strconv.Itoa(exponent)
}

// EqualJSON compares decoded JSON values using exact decimal number equality.
func EqualJSON(a, b any) bool {
	switch x := a.(type) {
	case nil:
		return b == nil
	case string:
		y, ok := b.(string)
		return ok && x == y
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case Number:
		y, ok := b.(Number)
		return ok && Normalize(x) == Normalize(y)
	case Array:
		y, ok := b.(Array)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !EqualJSON(x[i], y[i]) {
				return false
			}
		}
		return true
	case Object:
		y, ok := b.(Object)
		if !ok || len(x) != len(y) {
			return false
		}
		for k, v := range x {
			w, exists := y[k]
			if !exists || !EqualJSON(v, w) {
				return false
			}
		}
		return true
	default:
		panic("invalid comparison node")
	}
}

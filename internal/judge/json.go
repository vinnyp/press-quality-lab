package judge

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// MaxInput is the contract's byte limit.
const MaxInput = 16777216
const maxInteger int64 = 9007199254740991

type object map[string]any
type array []any
type number string

type reader struct {
	data []byte
	pos  int
	bad  bool
}

// readDocument is the sole raw-byte boundary. A nil shape checks JSON grammar
// and text limits alone; inputShape also validates the request schema and
// retains only values the rules read. Both modes use the same boundary checks.
func readDocument(data []byte, s *shape) (any, bool) {
	if len(data) > MaxInput || !utf8.Valid(data) {
		return nil, false
	}
	r := reader{data: data}
	v := r.node(0, s, true)
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
func (r *reader) node(depth int, s *shape, keep bool) any {
	r.space()
	if r.bad || r.pos == len(r.data) {
		r.bad = true
		return nil
	}
	c := r.data[r.pos]
	if c != '{' && c != '[' {
		materialize := keep || s != nil && s.kinds != "*"
		// A discarded free string still needs its grammar checked, but its
		// decoded contents cannot affect schema acceptance.
		if !keep && c == '"' && s != nil && s.kinds == "s" && s.values == nil {
			materialize = false
		}
		v := r.atom(materialize)

		if s != nil && !valid(v, s) {
			r.bad = true
		}
		if !keep {
			return nil
		}
		return v
	}
	depth++
	if depth > 64 {
		r.bad = true
		return nil
	}
	r.pos++
	if s != nil && s.kinds != "*" && ((c == '{' && s.kinds != "o") || (c == '[' && s.kinds != "a")) {
		r.bad = true
		return nil
	}
	if c == '{' {
		seen := map[string]bool{}
		var m object
		if keep {
			m = object{}
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
				var child *shape
				if s != nil && s.fields != nil {
					child = s.fields[k]
					if child == nil {
						r.bad = true
						break
					}
				}
				r.space()
				if !r.take(':') {
					r.bad = true
					break
				}
				retain := keep
				if s == inputShape.fields["evidence"].item {
					switch k {
					case "expected", "observed", "failure_kind", "targets", "evaluator":
						retain = false
					}
				}
				if s == inputShape.fields["trials"].item && k == "grade" {
					retain = false
				}
				presenceOnly := s == inputShape.fields["required"].item && k == "precondition"
				if presenceOnly {
					retain = false
				}
				r.space()
				present := r.pos < len(r.data) && r.data[r.pos] != 'n'
				v := r.node(depth, child, retain)
				if presenceOnly && present {
					v = ""
				}

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
		if s != nil && s.fields != nil && len(seen) != len(s.fields) {
			r.bad = true
		}
		if !keep {
			return nil
		}
		return m
	}
	var a array
	if keep {
		a = array{}
	}
	count := 0
	var item *shape
	if s != nil {
		item = s.item
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
	if s != nil && count < s.min {
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
		if r.pos == p || r.pos-p > 4 {
			r.bad = true
		}
	}
	if r.pos-start > 32 {
		r.bad = true
	}
	if !materialize {
		return nil
	}
	return number(r.data[start:r.pos])
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
			if materialize {
				b.Write(r.data[start : r.pos-1])
			}
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
func obj(v any) object {
	m, ok := v.(object)
	if !ok {
		panic("expected validated object")
	}
	return m
}
func arr(v any) array {
	a, ok := v.(array)
	if !ok {
		panic("expected validated array")
	}
	return a
}
func str(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case Dimension:
		return string(x)
	case Group:
		return string(x)
	case Scope:
		return string(x)
	case Kind:
		return string(x)
	case Tier:
		return string(x)
	case Request:
		return string(x)
	case Arm:
		return string(x)
	case EvidenceOutcome:
		return string(x)
	case CheckResult:
		return string(x)
	case DimensionStatus:
		return string(x)
	case GroupStatus:
		return string(x)
	case VerdictToken:
		return string(x)
	case SubLabel:
		return string(x)
	case PlainVerdict:
		return string(x)
	case Outcome:
		return string(x)
	case EntryClass:
		return string(x)
	case ComparisonResult:
		return string(x)
	case ComparisonDetail:
		return string(x)
	case BaselineResult:
		return string(x)
	case Reason:
		return string(x)
	case EvaluatorKind:
		return string(x)
	case BoundingFact:
		return string(x)
	case NextAction:
		return string(x)
	}
	s, ok := v.(string)
	if !ok {
		panic("expected validated string")
	}
	return s
}
func boolean(v any) bool { b, ok := v.(bool); return ok && b }
func integer(v any) int64 {
	n, ok := v.(number)
	if !ok {
		panic("expected validated integer")
	}
	i, e := strconv.ParseInt(string(n), 10, 64)
	if e != nil {
		panic(e)
	}
	return i
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
func write(v any) []byte { return append(appendValue(nil, v), '\n') }
func appendValue(dst []byte, v any) []byte {
	switch x := v.(type) {
	case nil:
		return append(dst, "null"...)
	case string:
		return quote(dst, x)
	case Dimension:
		return quote(dst, string(x))
	case Group:
		return quote(dst, string(x))
	case Scope:
		return quote(dst, string(x))
	case Kind:
		return quote(dst, string(x))
	case Tier:
		return quote(dst, string(x))
	case Request:
		return quote(dst, string(x))
	case Arm:
		return quote(dst, string(x))
	case EvidenceOutcome:
		return quote(dst, string(x))
	case CheckResult:
		return quote(dst, string(x))
	case DimensionStatus:
		return quote(dst, string(x))
	case GroupStatus:
		return quote(dst, string(x))
	case VerdictToken:
		return quote(dst, string(x))
	case SubLabel:
		return quote(dst, string(x))
	case PlainVerdict:
		return quote(dst, string(x))
	case Outcome:
		return quote(dst, string(x))
	case EntryClass:
		return quote(dst, string(x))
	case ComparisonResult:
		return quote(dst, string(x))
	case ComparisonDetail:
		return quote(dst, string(x))
	case BaselineResult:
		return quote(dst, string(x))
	case Reason:
		return quote(dst, string(x))
	case EvaluatorKind:
		return quote(dst, string(x))
	case BoundingFact:
		return quote(dst, string(x))
	case NextAction:
		return quote(dst, string(x))
	case Rule:
		return quote(dst, string(x))
	case bool:
		return strconv.AppendBool(dst, x)
	case int:
		return strconv.AppendInt(dst, int64(x), 10)
	case int64:
		return strconv.AppendInt(dst, x, 10)
	case number:
		if strings.ContainsAny(string(x), ".eE") {
			return append(dst, string(x)...)
		}
		i, e := strconv.ParseInt(string(x), 10, 64)
		if e != nil {
			panic(e)
		}
		return strconv.AppendInt(dst, i, 10)
	case array:
		dst = append(dst, '[')
		for i, a := range x {
			if i > 0 {
				dst = append(dst, ',')
			}
			dst = appendValue(dst, a)
		}
		return append(dst, ']')
	case object:
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
			dst = appendValue(dst, x[k])
		}
		return append(dst, '}')
	default:
		panic(fmt.Sprintf("unwritable value %T", v))
	}
}

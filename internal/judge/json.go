package judge

import (
	"strconv"

	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

// MaxInput is the judge contract input byte limit.
const MaxInput = 16777216
const maxInteger int64 = 9007199254740991

type object = strictjson.Object
type array = strictjson.Array
type number = strictjson.Number

var contractBounds = strictjson.Bounds{Bytes: MaxInput, Depth: 64, NumberChars: 32, ExponentDigits: 4}

// readDocument is the sole raw-byte boundary for judge input, enforcing grammar,
// contract bounds and the optional shape with its retention policy.
func readDocument(data []byte, s *shape) (any, bool) {
	var h strictjson.Hook
	if s != nil {
		h = shapeHook{s: s}
	}
	return strictjson.Read(data, contractBounds, h)
}
func write(v any) []byte { return strictjson.Write(v) }
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

type shapeHook struct {
	s    *shape
	mode strictjson.Mode
}

func (h shapeHook) Retention() strictjson.Mode { return h.mode }

func (h shapeHook) Materialize(keep bool, c byte) bool {
	s := h.s
	if !keep && c == '"' && s.kinds == "s" && s.values == nil {
		return false
	}
	return keep || s.kinds != "*"
}
func (h shapeHook) Valid(v any) bool { return valid(v, h.s) }
func (h shapeHook) Container(c byte) bool {
	return h.s.kinds == "*" || c == '{' && h.s.kinds == "o" || c == '[' && h.s.kinds == "a"
}
func (h shapeHook) Child(k string) (strictjson.Hook, bool) {
	s := h.s
	var child strictjson.Hook
	if s.fields != nil {
		f := s.fields[k]
		if f == nil {
			return nil, false
		}
		child = shapeHook{s: f}
	}
	mode := strictjson.Keep
	if s == inputShape.fields["evidence"].item {
		switch k {
		case "expected", "observed", "failure_kind", "targets", "evaluator":
			mode = strictjson.Discard
		}
	}
	if s == inputShape.fields["trials"].item && k == "grade" {
		mode = strictjson.Discard
	}
	if s == inputShape.fields["required"].item && k == "precondition" {
		mode = strictjson.Note
	}
	if c, ok := child.(shapeHook); ok {
		c.mode = mode
		child = c
	}
	return child, true
}
func (h shapeHook) Item() strictjson.Hook {
	if h.s.item == nil {
		return nil
	}
	return shapeHook{s: h.s.item}
}
func (h shapeHook) End(n int) bool {
	if h.s.fields != nil {
		return n == len(h.s.fields)
	}
	return n >= h.s.min
}

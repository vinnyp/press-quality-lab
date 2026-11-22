package judge

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

func specimen(s *shape) any {
	if s.kinds == "*" {
		return nil
	}
	if len(s.values) > 0 {
		return s.values[0]
	}
	switch s.kinds {
	case "s":
		return "sample"
	case "b":
		return true
	case "i":
		return number("0")
	case "a":
		return array{specimen(s.item)}
	case "o":
		o := object{}
		for k, f := range s.fields {
			o[k] = specimen(f)
		}
		return o
	default:
		panic("invalid shape")
	}
}
func clone(v any) any {
	copy, ok := readDocument(write(v), nil)
	if !ok {
		panic("cannot clone fixture")
	}
	return copy
}
func TestEverySchemaBoundary(t *testing.T) {
	data, err := os.ReadFile("../../docs/specs/judge-1.0/schema/judge-input.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	document, ok := readDocument(data, nil)
	if !ok {
		t.Fatal("invalid sealed schema")
	}
	bounds := map[*shape][2]number{}
	reference := obj(document)
	var derive func(object) *shape
	derive = func(o object) *shape {
		if ref, exists := o["$ref"]; exists {
			name := strings.TrimPrefix(str(ref), "#/$defs/")
			return derive(obj(obj(reference["$defs"])[name]))
		}
		if branches, exists := o["oneOf"]; exists {
			for _, branch := range arr(branches) {
				b := obj(branch)
				if b["type"] != "null" {
					return nullable(derive(b))
				}
			}
			t.Fatal("unsupported oneOf")
		}
		if values, exists := o["enum"]; exists {
			out := &shape{kinds: "s", values: []string{}}
			for _, v := range arr(values) {
				if v == nil {
					out.nullable = true
				} else {
					out.values = append(out.values, str(v))
				}
			}
			return out
		}
		typ, exists := o["type"]
		if !exists {
			return scalar("*")
		}
		isNull := false
		if ts, union := typ.(array); union {
			for _, v := range ts {
				if v == "null" {
					isNull = true
				} else {
					typ = v
				}
			}
		}
		kind := map[string]string{"object": "o", "array": "a", "string": "s", "integer": "i", "boolean": "b"}[str(typ)]
		if kind == "" {
			t.Fatalf("unsupported type %v", typ)
		}
		out := &shape{kinds: kind, nullable: isNull}
		if props, exists := o["properties"]; exists {
			out.fields = map[string]*shape{}
			if o["additionalProperties"] != false || len(arr(o["required"])) != len(obj(props)) {
				t.Fatal("unexpected object schema")
			}
			for _, key := range arr(o["required"]) {
				k := str(key)
				out.fields[k] = derive(obj(obj(props)[k]))
			}
		}
		if item, exists := o["items"]; exists {
			out.item = derive(obj(item))
		}
		if minimum, exists := o["minItems"]; exists {
			out.min = int(integer(minimum))
		}
		if kind == "i" {
			low, lok := o["minimum"].(number)
			high, hok := o["maximum"].(number)
			if !lok || !hok {
				t.Fatal("integer bounds missing")
			}
			bounds[out] = [2]number{low, high}
		}
		return out
	}
	specShape := derive(reference)
	base := specimen(specShape)
	// Compare schema acceptance directly: the synthetic specimen need not obey X1–X11.
	check := func(t *testing.T, input []byte, accept bool) {
		t.Helper()
		v, ok := readDocument(input, nil)
		got := ok && valid(v, inputShape)
		_, streaming := readDocument(input, inputShape)
		if got != accept || streaming != accept {
			t.Fatalf("schema acceptance: tree=%v streaming=%v want=%v: %s", got, streaming, accept, input)
		}
	}
	check(t, write(base), true)

	type location struct {
		path []any
		s    *shape
	}
	locations := []location{}
	var walk func(any, *shape, []any)
	walk = func(v any, s *shape, path []any) {
		locations = append(locations, location{append([]any{}, path...), s})
		if s.fields != nil {
			for k, f := range s.fields {
				walk(obj(v)[k], f, append(append([]any{}, path...), k))
			}
		}
		if s.item != nil {
			walk(arr(v)[0], s.item, append(append([]any{}, path...), 0))
		}
	}
	walk(base, specShape, nil)
	for _, loc := range locations {
		alter := func(value any, remove bool, extra bool) []byte {
			v := clone(base)
			cur := v
			for i, p := range loc.path {
				last := i == len(loc.path)-1
				switch k := p.(type) {
				case string:
					if last {
						if remove {
							delete(obj(cur), k)
						} else {
							obj(cur)[k] = value
						}
						if extra {
							obj(obj(cur)[k])["unknown"] = true
						}
						return write(v)
					}
					cur = obj(cur)[k]
				case int:
					if last {
						arr(cur)[k] = value
						if extra {
							obj(arr(cur)[k])["unknown"] = true
						}
						return write(v)
					}
					cur = arr(cur)[k]
				default:
					panic("bad path")
				}
			}
			if extra {
				obj(v)["unknown"] = true
				return write(v)
			}
			return write(value)
		}
		name := testRowName(fmt.Sprint(loc.path))
		for _, v := range []any{"text", true, number("0"), array{}, object{}} {
			matches := valid(v, loc.s)
			if !matches {
				t.Run(testRowName(name+" type "+fmt.Sprintf("%T", v)), func(t *testing.T) { check(t, alter(v, false, false), false) })
			}
		}

		if loc.s.kinds != "*" {
			t.Run(testRowName(name+" wrong type"), func(t *testing.T) {
				bad := any(array{})
				if loc.s.kinds == "a" {
					bad = object{}
				}
				check(t, alter(bad, false, false), false)
			})
		}
		if loc.s.nullable || loc.s.kinds == "*" {
			t.Run(testRowName(name+" null allowed"), func(t *testing.T) { check(t, alter(nil, false, false), true) })
		}
		if !loc.s.nullable && loc.s.kinds != "*" {
			t.Run(testRowName(name+" null forbidden"), func(t *testing.T) { check(t, alter(nil, false, false), false) })
		}
		if len(loc.path) > 0 {
			if _, ok := loc.path[len(loc.path)-1].(string); ok {
				t.Run(testRowName(name+" required"), func(t *testing.T) { check(t, alter(nil, true, false), false) })
			}
		}
		if loc.s.fields != nil {
			t.Run(testRowName(name+" unknown key"), func(t *testing.T) { check(t, alter(specimen(loc.s), false, true), false) })
		}
		if loc.s.min > 0 {
			t.Run(testRowName(name+" minItems"), func(t *testing.T) { check(t, alter(array{}, false, false), false) })
		}
		for _, value := range loc.s.values {
			t.Run(testRowName(name+" enum "+value), func(t *testing.T) { check(t, alter(value, false, false), true) })
		}
		if len(loc.s.values) > 0 {
			t.Run(testRowName(name+" enum"), func(t *testing.T) { check(t, alter("invalid-enum", false, false), false) })
		}
		if loc.s.kinds == "i" {
			limit := bounds[loc.s]
			low, high := integer(limit[0]), integer(limit[1])
			for _, n := range []int64{low, high} {
				t.Run(testRowName(name+" integer accepted "+strconv.FormatInt(n, 10)), func(t *testing.T) { check(t, alter(number(strconv.FormatInt(n, 10)), false, false), true) })
			}
			for _, s := range []string{strconv.FormatInt(low-1, 10), strconv.FormatInt(high+1, 10), "1.0", "1e0"} {

				t.Run(testRowName(name+" integer "+s), func(t *testing.T) { check(t, alter(number(s), false, false), false) })
			}
		}
	}
}

// testRowName makes each row directly addressable by go test -run and -skip.
func testRowName(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			return r
		}
		return '_'
	}, s)
}

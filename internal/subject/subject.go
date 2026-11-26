// Package subject validates subject-file bytes without deriving checks or reading files.
package subject

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

// File retains all schema fields, including placeholders and exact literal numbers.
// Objects and arrays use the shared strictjson value types.
type File = strictjson.Object

// Error identifies a rejecting rule without exposing subject values.
type Error struct{ Rule string }

func (e *Error) Error() string { return e.Rule + ": invalid subject file" }

// Parse checks strict reading, schema shape and declaration semantics. File
// evidence is supplied separately by the grade reader through ParseWithFiles.
func Parse(data []byte) (File, *Error) { return ParseWithFiles(data, nil) }

// ParseWithFiles combines file evidence with declaration checks in E-9 order.
// The callback is the caller's sole file reader and must tolerate invalid shapes.
func ParseWithFiles(data []byte, inspect func(File) []string) (File, *Error) {
	value, ok := strictjson.Read(data, strictjson.Bounds{Bytes: 16 << 20, Depth: 64, NumberChars: 32, ExponentDigits: 4}, nil)
	if !ok {
		return nil, &Error{Rule: "QS2"}
	}
	root, ok := value.(strictjson.Object)
	if !ok {
		return nil, &Error{Rule: "QS2"}
	}
	failures := map[string]bool{}
	if root["subject_file_version"] != "1.0" {
		failures["QS1"] = true
	}
	subjectShape().check(root, failures)
	semantic(root, failures)
	if inspect != nil {
		for _, rule := range inspect(root) {
			failures[rule] = true
		}
	}
	for _, rule := range []string{
		"QS1",
		"QS3",
		"QS4",
		"QS5",
		"QS6",
		"QS7",
		"QS8",
		"QS9",
		"QS10",
		"QS11",
		"QS12",
		"QP7",
		"QP10",
		"QA4",
		"QA9",
		"QS2",
	} {
		if failures[rule] {
			return nil, &Error{Rule: rule}
		}
	}
	return root, nil
}

type shape struct {
	kind                  string
	rule                  string
	min, max              int
	fields                map[string]*shape
	item                  *shape
	choices               []*shape
	values                []string
	nullable, placeholder bool
	pattern               string
}

func (s *shape) check(v any, failures map[string]bool) {
	if s.kind == "any" {
		return
	}
	bad := func() {
		rule := s.rule
		if rule == "" {
			rule = "QS2"
		}
		failures[rule] = true
	}
	if v == nil && s.nullable {
		return
	}
	if o, ok := v.(strictjson.Object); ok {
		if _, present := o["placeholder"]; present {
			if s.placeholder && len(o) == 1 && str(1, 4096).valid(o["placeholder"]) {
				return
			}
			failures["QS5"] = true
			return
		}
	}
	if len(s.choices) > 0 {
		o, ok := v.(strictjson.Object)
		if !ok {
			bad()
			return
		}
		for _, c := range s.choices {
			for key := range c.fields {
				if _, present := o[key]; present {
					c.check(v, failures)
					return
				}
			}
		}
		bad()
		return
	}
	switch s.kind {
	case "any":
		return
	case "string":
		x, ok := v.(string)
		if !ok || utf8.RuneCountInString(x) < s.min || utf8.RuneCountInString(x) > s.max {
			bad()
			return
		}
		if len(s.values) > 0 {
			found := false
			for _, candidate := range s.values {
				if x == candidate {
					found = true
				}
			}
			if !found {
				bad()
			}
		}
		switch s.pattern {
		case "relative":
			if strings.HasPrefix(x, "/") || strings.ContainsRune(x, 0) {
				bad()
			}
		case "name", "id":
			for _, r := range x {
				if r < 32 && s.pattern == "name" {
					failures["QS11"] = true
				} else if r < 32 {
					bad()
				}
				if r == '{' || r == '}' || r == ':' {
					bad()
				}
			}
		case "env":
			for i, r := range x {
				if !(r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || i > 0 && r >= '0' && r <= '9') {
					bad()
				}
			}
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			bad()
		}
	case "integer":
		x, ok := v.(strictjson.Number)
		if !ok {
			bad()
			return
		}
		n, err := strconv.ParseInt(string(x), 10, 64)
		if err != nil || n < int64(s.min) || n > int64(s.max) {
			bad()
		}
	case "array":
		a, ok := v.(strictjson.Array)
		if !ok {
			if s.rule == "QS11" {
				failures["QS2"] = true
				return
			}
			bad()
			return
		}
		if len(a) < s.min || len(a) > s.max {
			if s.rule == "QS11" {
				failures["QS11"] = true
			} else {
				bad()
			}
		}
		for _, x := range a {
			s.item.check(x, failures)
		}
	case "object":
		o, ok := v.(strictjson.Object)
		if !ok {
			bad()
			return
		}
		if s.rule == "QP10" && o["kind"] == "seeded_state" && o["reason"] == nil {
			failures["QP10"] = true
		}
		for key, c := range s.fields {
			x, present := o[key]
			if !present {
				bad()
				continue
			}
			c.check(x, failures)
		}
		for key, x := range o {
			if _, present := s.fields[key]; !present {
				if s.item == nil {
					bad()
				} else {
					s.item.check(x, failures)
					if !envName(key) {
						bad()
					}
				}
			}
		}
		if s.item != nil && len(o) > s.max {
			bad()
		}
	}
}
func (s *shape) valid(v any) bool         { f := map[string]bool{}; s.check(v, f); return len(f) == 0 }
func envName(v string) bool               { s := str(1, 4096); s.pattern = "env"; return s.valid(v) }
func str(minimum, maximum int) *shape     { return &shape{kind: "string", min: minimum, max: maximum} }
func integer(minimum, maximum int) *shape { return &shape{kind: "integer", min: minimum, max: maximum} }
func array(minimum, maximum int, item *shape) *shape {
	return &shape{kind: "array", min: minimum, max: maximum, item: item}
}
func object(rule string, fields map[string]*shape) *shape {
	return &shape{kind: "object", rule: rule, fields: fields}
}
func named(rule string, s *shape) *shape  { s.rule = rule; return s }
func optional(s *shape) *shape            { s.nullable = true; return s }
func fill(s *shape) *shape                { s.placeholder = true; return s }
func enumeration(values ...string) *shape { s := str(0, 4096); s.values = values; return s }
func words() *shape                       { return array(0, 64, str(1, 256)) }
func name() *shape                        { s := str(1, 256); s.pattern = "id"; return s }
func runbookName() *shape                 { s := name(); s.pattern = "name"; return s }
func relative() *shape                    { s := str(1, 4096); s.pattern = "relative"; s.rule = "QS9"; return s }
func codes() *shape                       { return optional(array(1, 256, integer(0, 255))) }
func computation() *shape {
	return underRule("QA4", object("QA4", map[string]*shape{"argv": array(1, 64, str(0, 4096)), "budget_seconds": integer(1, 600), "declared_files": array(0, 64, str(1, 4096)), "captures": array(0, 64, name())}))
}
func dimension() *shape {
	return enumeration("availability", "execution", "graceful-failure", "robustness", "response-correctness", "determinism", "provenance-honesty", "completeness-signaling", "output-conformance", "flag-conformance", "bounding", "discoverability", "error-message-quality", "token-economy", "side-effect-integrity", "idempotency", "destructive-guard", "pii-secret-hygiene")
}

// underRule attributes the schema constraints within a named declaration.
func underRule(rule string, s *shape) *shape {
	if s.rule == "" {
		s.rule = rule
	}
	for _, child := range s.fields {
		underRule(rule, child)
	}
	if s.item != nil {
		underRule(rule, s.item)
	}
	return s
}
func subjectShape() *shape {
	facts := object("QS4", map[string]*shape{})
	for _, key := range []string{"read_only", "has_destructive_ops", "store_backed", "has_cli"} {
		facts.fields[key] = fill(named("QS4", &shape{kind: "boolean"}))
	}
	facts.fields["bounding"] = fill(named("QS4", enumeration("none", "declared")))
	command := underRule("QS6", object("QS6", map[string]*shape{"words": named("QS6", words()), "class": fill(named("QS6", enumeration("safe", "destructive"))), "writes": fill(named("QS6", &shape{kind: "boolean"})), "exit_codes": named("QS6", codes()), "byte_bound": optional(named("QS6", integer(1, 1073741824))), "budget_seconds": named("QS6", integer(1, 3600))}))
	expected := &shape{choices: []*shape{object("", map[string]*shape{"literal": {kind: "any"}}), object("QA4", map[string]*shape{"computation": computation()})}}
	check := object("", map[string]*shape{"dimension": dimension(), "predicate": named("QA9", enumeration("exit_code", "stdout_equals", "stdout_json_equals", "stdout_contains", "stdout_not_contains", "stderr_contains", "stdout_bytes_at_most", "json_path_equals", "row_count", "stdout_empty")), "path": optional(str(0, 1024)), "expected": expected, "precondition": optional(object("", map[string]*shape{"computation": computation()})), "exit_codes": codes()})
	step := object("", map[string]*shape{"name": runbookName(), "words": words(), "budget_seconds": integer(1, 3600), "checks": array(1, 64, check)})
	env := &shape{kind: "object", max: 64, item: str(0, 4096)}
	secret := optional(fill(underRule("QP10", object("QP10", map[string]*shape{"path": relative(), "kind": named("QP10", enumeration("credential", "seeded_state")), "reason": optional(named("QP10", str(1, 4096))), "token": named("QP10", str(8, 256)), "commands": named("QP10", array(1, 64, words()))}))))
	return object("QS3", map[string]*shape{
		"subject_file_version": {kind: "any"}, "binary": fill(str(1, 4096)), "facts": facts, "commands": named("QS11", array(0, 512, command)), "declared_count": fill(integer(0, 100000)), "seed": fill(optional(str(1, 4096))), "store": fill(optional(relative())),
		"offline": array(0, 256, underRule("QS7", object("QS7", map[string]*shape{"applies_to": array(1, 1024, words()), "args": array(0, 64, str(0, 4096)), "env": env}))), "bookkeeping": array(0, 256, relative()), "secret": secret,
		"runbooks": array(0, 1024, object("", map[string]*shape{"name": runbookName(), "steps": array(1, 256, step)})), "markers": named("QS11", array(0, 1024, object("", map[string]*shape{"record": name(), "dimension": dimension()}))), "prompts": object("", map[string]*shape{"version": str(0, 64)}),
	})
}

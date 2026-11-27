package decide

import (
	"bytes"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"strconv"
	"strings"
)

func authoredPredicate(ch plan.Check, e Evidence) Result {
	var indices []int
	for i, inv := range e.Invocations {
		if inv.Key["computation"] == nil {
			indices = append(indices, i)
		}
	}
	if len(indices) != 1 {
		panic("QA9: missing or repeated step run")
	}
	run := e.Invocations[indices[0]].Run
	if err := observe.ValidateCounts(run, false); err != nil {
		panic(err)
	}
	p := ch.Preimage["predicate"]
	if p != "exit_code" && p != "stdout_bytes_at_most" && (p == "stderr_contains" && run.StderrCut || p != "stderr_contains" && run.StdoutCut) {
		return Result{Outcome: NotObserved, Reason: "harness_fault", Level: 8}
	}
	v := Expected(ch, e)
	held := false
	var size *int64
	setSize := func(n int64) { size = &n }
	switch p {
	case "exit_code":
		held = strictjson.EqualJSON(v, strictjson.Number(strconv.Itoa(run.End.Code)))
		if !held {
			panic("QO5: mismatched exit reached predicate")
		}
	case "stdout_equals":
		held = bytes.Equal(run.Stdout, []byte(stringValue(v)))
	case "stdout_contains":
		held = bytes.Contains(run.Stdout, []byte(stringValue(v)))
	case "stdout_not_contains":
		held = !bytes.Contains(run.Stdout, []byte(stringValue(v)))
		setSize(int64(len(run.Stdout)))
	case "stderr_contains":
		held = bytes.Contains(run.Stderr, []byte(stringValue(v)))
	case "stdout_bytes_at_most":
		held = countEqualsOrBelow(run.StdoutSeen, v)
		setSize(run.StdoutSeen)
	case "stdout_empty":
		held = run.StdoutSeen == 0
	case "stdout_json_equals", "json_path_equals", "row_count":
		value, ok := strictjson.Read(run.Stdout, strictjson.Bounds{Bytes: 1 << 20, Depth: 64, NumberChars: 32, ExponentDigits: 4}, nil)
		if ok && p != "stdout_json_equals" {
			value, ok = jsonPointer(value, stringValue(ch.Preimage["path"]))
		}
		if p == "row_count" {
			a, isArray := value.(strictjson.Array)
			if ok && isArray {
				held = strictjson.EqualJSON(strictjson.Number(strconv.Itoa(len(a))), v)
				if object(object(ch.Preimage["expected"])["computation"]) != nil {
					setSize(int64(len(a)))
				}
			}
		} else {
			held = ok && strictjson.EqualJSON(value, v)
		}
	default:
		panic("QA9: unknown predicate")
	}
	r := Result{Outcome: "held", Level: 8, Expected: v, TargetSize: size}
	if !held {
		r.Outcome = "not_held"
		r.FailureKind = "predicate"
	}
	return authoredFields(ch, e, r)
}
func countEqualsOrBelow(n int64, v any) bool {
	if !whole(v) {
		panic("QA9: invalid whole number")
	}
	number, ok := v.(strictjson.Number)
	if !ok {
		panic("QA9: expected number")
	}
	bound, err := strconv.ParseInt(string(number), 10, 64)
	if err != nil {
		panic(err)
	}
	return n <= bound
}
func jsonPointer(v any, p string) (any, bool) {
	if p == "" {
		return v, true
	}
	if !strings.HasPrefix(p, "/") {
		return nil, false
	}
	for _, token := range strings.Split(p[1:], "/") {
		for i := 0; i < len(token); i++ {
			if token[i] == '~' {
				i++
				if i == len(token) || (token[i] != '0' && token[i] != '1') {
					return nil, false
				}
			}
		}
		token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
		switch x := v.(type) {
		case strictjson.Object:
			var ok bool
			v, ok = x[token]
			if !ok {
				return nil, false
			}
		case strictjson.Array:
			if token == "" || len(token) > 1 && token[0] == '0' {
				return nil, false
			}
			for _, c := range token {
				if c < '0' || c > '9' {
					return nil, false
				}
			}
			i, err := strconv.ParseUint(token, 10, 64)
			if err != nil || i >= uint64(len(x)) {
				return nil, false
			}
			v = x[i]
		default:
			return nil, false
		}
	}
	return v, true
}

func stringValue(v any) string {
	s, ok := v.(string)
	if !ok {
		panic("QA9: expected string")
	}
	return s
}

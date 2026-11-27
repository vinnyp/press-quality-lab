package decide

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"reflect"
	"strings"
	"testing"
)

func TestAuthoredPredicateRows(t *testing.T) {
	// QA2, QA9: every predicate must distinguish held from predicate failure.
	for _, row := range []struct {
		name, predicate, output string
		expected                any
	}{
		{"Equals", "stdout_equals", "x", "x"},
		{"Contains", "stdout_contains", "ax", "x"},
		{"NotContains", "stdout_not_contains", "a", "x"},
		{"Stderr", "stderr_contains", "x", "x"},
		{"JSON", "stdout_json_equals", "1e9999", strictjson.Number("10e9998")},
		{"Path", "json_path_equals", `{"a":1}`, strictjson.Number("1")},
		{"Rows", "row_count", `{"a":[1]}`, strictjson.Number("1")},
		{"Bytes", "stdout_bytes_at_most", "x", strictjson.Number("1")},
		{"Empty", "stdout_empty", "", nil},
	} {
		t.Run(row.name, func(t *testing.T) {
			ch := plan.Check{Preimage: plan.Object{"predicate": row.predicate, "path": "/a", "expected": plan.Object{"literal": row.expected}}}
			run := observe.Run{Stdout: []byte(row.output), Stderr: []byte(row.output), StdoutSeen: int64(len(row.output))}
			e := Evidence{Invocations: []observe.Invocation{{Run: run}}}
			if got := level8(ch, passing(), e); got.Outcome != "held" {
				t.Fatalf("positive: %+v", got)
			}
			switch row.predicate {
			case "stdout_not_contains":
				run.Stdout = []byte("x")
			case "stdout_bytes_at_most":
				run.StdoutSeen = 2
			case "stdout_empty":
				run.StdoutSeen = 1
			default:
				run.Stdout = []byte("z")
				run.Stderr = []byte("z")
			}
			e.Invocations[0].Run = run
			if got := level8(ch, passing(), e); got.Outcome != "not_held" || got.FailureKind != "predicate" {
				t.Fatalf("negative: %+v", got)
			}
		})
	}
}

func mustFault(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Error("missing engine fault")
		}
	}()
	f()
}
func computedCheck(p string, v any) (plan.Check, Evidence) {
	c := plan.Object{"argv": plan.Array{"/bin/echo"}, "captures": plan.Array{}}
	ch := plan.Check{Subject: plan.Object{}, Preimage: plan.Object{"probe": nil, "earlier": plan.Array{}, "predicate": p, "expected": plan.Object{"computation": c}}}
	e := Evidence{Invocations: []observe.Invocation{{Key: observe.Key{"computation": c["argv"]}, Run: observe.Run{Markers: observe.Markers{Confinement: true}, Details: observe.RunDetails{Printed: &observe.PrintedValue{Value: v}}}}}}
	return ch, e
}
func TestComputationTypes(t *testing.T) {
	// QA6, QA9, E-48, E-50: expected computations obey the predicate's type.
	for _, row := range []struct {
		name  string
		value any
		valid bool
	}{
		{"Zero", strictjson.Number("0"), true}, {"NegativeZero", strictjson.Number("-0"), true}, {"Maximum", strictjson.Number("9007199254740991"), true},
		{"TooLarge", strictjson.Number("9007199254740992"), false}, {"Negative", strictjson.Number("-1"), false}, {"Fraction", strictjson.Number("1.0"), false}, {"Exponent", strictjson.Number("1e1"), false},
	} {
		t.Run(row.name, func(t *testing.T) {
			for _, p := range []string{"exit_code", "stdout_bytes_at_most", "row_count"} {
				ch, e := computedCheck(p, row.value)
				r := level6(ch, passing(), e)
				if row.valid {
					if r != (Result{}) {
						t.Fatal(r)
					}
				} else if r.Reason != "computation_failed" || r.Level != 6 {
					t.Fatal(r)
				}
			}
		})
	}
	for _, p := range []string{"stdout_equals", "stdout_contains", "stdout_not_contains", "stderr_contains"} {
		for _, row := range []struct {
			name  string
			v     any
			valid bool
		}{{"String", "x", true}, {"NotString", true, false}, {"Empty", "", false}} {
			t.Run(p+"_"+row.name, func(t *testing.T) {
				ch, e := computedCheck(p, row.v)
				r := level6(ch, passing(), e)
				if (r.Level == 0) != row.valid {
					t.Fatal(r)
				}
			})
		}
	}
}
func TestComputationEnds(t *testing.T) {
	// QO6, QA6: every non-normal computation end fails its check, preconditions included.
	for _, row := range []struct {
		name   string
		change func(*observe.Run)
	}{
		{"Unconfined", func(r *observe.Run) { r.Markers.Confinement = false }},
		{"CouldNotStart", func(r *observe.Run) { r.Markers.CouldNotStart = true }},
		{"Confinement", func(r *observe.Run) { r.End.Signal = 9 }},
		{"Budget", func(r *observe.Run) {
			r.End = observe.End{Signal: 9, EngineKilled: true, BudgetExpired: true, RunningAtBudget: true}
		}},
		{"EngineOutsideBudget", func(r *observe.Run) { r.End = observe.End{Signal: 9, EngineKilled: true} }},
		{"Memory", func(r *observe.Run) { r.End = observe.End{Signal: 9, MemoryKilled: true} }},
		{"CodeSigning", func(r *observe.Run) { r.End = observe.End{Signal: 9, CodeSigningKilled: true} }},
		{"OtherSignal", func(r *observe.Run) { r.End.Signal = 11 }},
		{"ExitOne", func(r *observe.Run) { r.End.Code = 1 }},
		{"InvalidValue", func(r *observe.Run) { r.Details.Printed = nil }},
	} {
		t.Run(row.name, func(t *testing.T) {
			for _, pre := range []bool{false, true} {
				ch, e := computedCheck("stdout_json_equals", nil)
				if pre {
					ex := object(ch.Preimage["expected"])
					ex["precondition"] = plan.Object{"computation": ex["computation"]}
					delete(ex, "computation")
					ex["literal"] = nil
				}
				if r := level6(ch, passing(), e); r != (Result{}) {
					t.Fatal("positive", r)
				}
				row.change(&e.Invocations[0].Run)
				if r := level6(ch, passing(), e); r.Reason != "computation_failed" || r.Level != 6 {
					t.Fatal(r)
				}
			}
		})
	}
}
func TestComputationGuards(t *testing.T) {
	// QA4, QA6, QO1: capture cutting precedes computation failures, but follows QO1.
	ch, e := computedCheck("stdout_json_equals", nil)
	c := object(object(ch.Preimage["expected"])["computation"])
	c["captures"] = plan.Array{"earlier"}
	mustFault(t, func() { level6(ch, passing(), e) })
	e.Captures = []observe.Invocation{{Key: observe.Key{"step": "earlier"}, Run: observe.Run{StdoutCut: true, End: observe.End{Code: 7}}}}
	e.Invocations = nil
	if r := BeforeComputations(ch, passing(), e); r.Reason != "harness_fault" {
		t.Fatal(r)
	}
	if r := level6(ch, passing(), e); r.Reason != "harness_fault" {
		t.Fatal(r)
	}
	e.Invocations = []observe.Invocation{{Run: observe.Run{}}}
	if r := Resolve(ch, passing(), e); r.Level != 1 {
		t.Fatal(r)
	}
	if r := BeforeComputations(ch, passing(), e); r.Level != 1 {
		t.Fatal(r)
	}
	e.Captures[0].Run.StdoutCut = false
	e.Invocations = nil
	mustFault(t, func() { level6(ch, passing(), e) })
}
func TestRunbookStopping(t *testing.T) {
	// QA7, QA9, E-49: only required checks stop a runbook, from pre-override outcomes.
	for _, name := range []string{"predicate", "crashed", "over_budget", "could_not_start", "computation_failed", "placeholder_unfilled"} {
		t.Run(name, func(t *testing.T) {
			r := Result{Outcome: "not_held", FailureKind: name}
			if name == "could_not_start" || name == "computation_failed" || name == "placeholder_unfilled" {
				r = Result{Outcome: NotObserved, Reason: Reason(name)}
			}
			m := map[string]bool{"a": true}
			res := map[string]Result{"a": {Outcome: "held"}}
			if Stopped([]string{"a"}, res, m) {
				t.Fatal("held stopped")
			}
			res["a"] = r
			if !Stopped([]string{"a"}, res, m) {
				t.Fatal("failure did not stop")
			}
			ch := plan.Check{Preimage: plan.Object{"earlier": plan.Array{"a"}}}
			if r := level4(ch, passing(), Evidence{Earlier: res, Membership: m}); r.Reason != "earlier_step_failed" {
				t.Fatal(r)
			}
		})
	}
	if Stopped([]string{"a"}, nil, map[string]bool{"a": false}) {
		t.Fatal("outside stopped")
	}
	mustFault(t, func() { Stopped([]string{"a"}, nil, map[string]bool{"a": true}) })
	mustFault(t, func() { Stopped([]string{"a"}, nil, nil) })
	for _, name := range []string{"Runs", "Stopped", "Placeholder", "Confinement", "SelfTest", "System", "Copy"} {
		t.Run(name, func(t *testing.T) {
			f := StepFacts{Controls: passing()}
			switch name {
			case "Stopped":
				f.Stopped = true
			case "Placeholder":
				f.Placeholder = true
			case "Confinement":
				f.Controls.ConfinementApplied = false
			case "SelfTest":
				f.Controls.SelfTestPassed = false
			case "System":
				f.Controls.SystemNamed = false
			case "Copy":
				f.Controls.PrivateCopyUnchanged = false
			}
			if StepRuns(f) != (name == "Runs") {
				t.Fatal(f)
			}
		})
	}
}

func TestExitExpected(t *testing.T) {
	// QA2, QO5, QA9: expected code wins; capture runs never supply the subject exit.
	for _, computed := range []bool{false, true} {
		ch, e := computedCheck("exit_code", strictjson.Number("7"))
		if !computed {
			ch.Preimage["expected"] = plan.Object{"literal": strictjson.Number("7")}
			e.Invocations = nil
		}
		e.Invocations = append(e.Invocations, observe.Invocation{Run: observe.Run{Markers: observe.Markers{Confinement: true}, End: observe.End{Code: 7}}})
		e.Captures = []observe.Invocation{{Run: observe.Run{End: observe.End{Code: 99}}}}
		if r := Resolve(ch, passing(), e); r.Outcome != "held" {
			t.Fatal(r)
		}
		e.Invocations[len(e.Invocations)-1].Run.End.Code = 0
		if r := Resolve(ch, passing(), e); r.Level != 7 || r.FailureKind != "wrong_exit" {
			t.Fatal(r)
		}
		mustFault(t, func() { level8(ch, passing(), e) })
	}
	ch := plan.Check{Subject: plan.Object{"commands": plan.Array{plan.Object{"words": plan.Array{}, "exit_codes": plan.Array{strictjson.Number("9")}}}}, Preimage: plan.Object{"parameters": plan.Object{"exit_codes": plan.Array{strictjson.Number("0"), strictjson.Number("7")}}}}
	for _, row := range []struct {
		code int
		want bool
	}{{0, true}, {7, true}, {9, false}} {
		if acceptedCode(ch, row.code) != row.want {
			t.Fatal(row)
		}
	}
}
func TestAuthoredCutStreams(t *testing.T) {
	// QA9: each stream predicate refuses its own cut stream, with byte-count and exit exceptions.
	for _, p := range []string{"exit_code", "stdout_equals", "stdout_contains", "stdout_not_contains", "stderr_contains", "stdout_json_equals", "json_path_equals", "row_count", "stdout_bytes_at_most", "stdout_empty"} {
		t.Run(p, func(t *testing.T) {
			var expected any = "x"
			switch p {
			case "exit_code":
				expected = strictjson.Number("0")
			case "stdout_json_equals", "json_path_equals":
				expected = nil
			case "row_count", "stdout_bytes_at_most":
				expected = strictjson.Number("1")
			case "stdout_empty":
				expected = nil
			}
			ch := plan.Check{Preimage: plan.Object{"predicate": p, "path": "", "expected": plan.Object{"literal": expected}}}
			for _, stderr := range []bool{false, true} {
				run := observe.Run{Stdout: []byte("null"), Stderr: []byte("x")}
				if stderr {
					run.StderrCut = true
				} else {
					run.StdoutCut = true
				}
				r := level8(ch, passing(), Evidence{Invocations: []observe.Invocation{{Run: run}}})
				want := p != "exit_code" && p != "stdout_bytes_at_most" && (stderr == (p == "stderr_contains"))
				if (r.Reason == "harness_fault") != want {
					t.Fatal(stderr, r)
				}
			}
		})
	}
}
func TestAuthoredJSONRules(t *testing.T) {
	// QA9, QS2: strict §3 grammar and exact JSON equality apply to output.
	for _, row := range []struct {
		name, out string
		expected  any
		held      bool
	}{
		{"ExactHuge", "1e9999", strictjson.Number("10e9998"), true},
		{"DistinctIntegers", "9007199254740993", strictjson.Number("9007199254740992"), false},
		{"NFCNFD", `"é"`, "e\u0301", false},
		{"ObjectOrder", `{"b":2,"a":1}`, plan.Object{"a": strictjson.Number("1"), "b": strictjson.Number("2")}, true},
		{"ArrayOrder", `[2,1]`, plan.Array{strictjson.Number("1"), strictjson.Number("2")}, false},
		{"RepeatedKey", `{"a":1,"a":1}`, plan.Object{"a": strictjson.Number("1")}, false},
		{"Surrogate", `"\ud800"`, "x", false},
		{"LongNumber", "100000000000000000000000000000000", strictjson.Number("1e32"), false},
		{"Depth65", strings.Repeat("[", 65) + "0" + strings.Repeat("]", 65), nil, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			ch := plan.Check{Preimage: plan.Object{"predicate": "stdout_json_equals", "expected": plan.Object{"literal": row.expected}}}
			r := level8(ch, passing(), Evidence{Invocations: []observe.Invocation{{Run: observe.Run{Stdout: []byte(row.out)}}}})
			if (r.Outcome == "held") != row.held {
				t.Fatal(r)
			}
		})
	}
	// QA9: byte comparisons never repair invalid UTF-8.
	ch := plan.Check{Preimage: plan.Object{"predicate": "stdout_equals", "expected": plan.Object{"literal": "�"}}}
	if r := level8(ch, passing(), Evidence{Invocations: []observe.Invocation{{Run: observe.Run{Stdout: []byte{255}}}}}); r.FailureKind != "predicate" {
		t.Fatal(r)
	}
}
func TestPointerRows(t *testing.T) {
	// QA9: literal RFC 6901 §5 example table, plus array grammar and missing paths.
	v := plan.Object{"foo": plan.Array{"bar", "baz"}, "": strictjson.Number("0"), "a/b": strictjson.Number("1"), "c%d": strictjson.Number("2"), "e^f": strictjson.Number("3"), "g|h": strictjson.Number("4"), "i\\j": strictjson.Number("5"), "k\"l": strictjson.Number("6"), " ": strictjson.Number("7"), "m~n": strictjson.Number("8")}
	for _, row := range []struct {
		name, path string
		want       any
		ok         bool
	}{
		{"Root", "", v, true}, {"Foo", "/foo", plan.Array{"bar", "baz"}, true}, {"Index", "/foo/0", "bar", true}, {"EmptyKey", "/", strictjson.Number("0"), true},
		{"Slash", "/a~1b", strictjson.Number("1"), true}, {"Percent", "/c%d", strictjson.Number("2"), true}, {"Caret", "/e^f", strictjson.Number("3"), true},
		{"Pipe", "/g|h", strictjson.Number("4"), true}, {"Backslash", "/i\\j", strictjson.Number("5"), true}, {"Quote", "/k\"l", strictjson.Number("6"), true},
		{"Space", "/ ", strictjson.Number("7"), true}, {"Tilde", "/m~0n", strictjson.Number("8"), true}, {"LeadingZero", "/foo/00", nil, false}, {"Dash", "/foo/-", nil, false},
		{"Missing", "/missing", nil, false}, {"NestedMissing", "/foo/0/x", nil, false}, {"Plus", "/foo/+0", nil, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			got, ok := jsonPointer(v, row.path)
			if ok != row.ok || ok && !strictjson.EqualJSON(got, row.want) {
				t.Fatal(got, ok)
			}
		})
	}
	for _, out := range []string{`{"a":1}`, `{"b":[]}`, `not_json`} {
		ch := plan.Check{Preimage: plan.Object{"predicate": "row_count", "path": "/a", "expected": plan.Object{"literal": strictjson.Number("1")}}}
		if r := level8(ch, passing(), Evidence{Invocations: []observe.Invocation{{Run: observe.Run{Stdout: []byte(out)}}}}); r.FailureKind != "predicate" || r.TargetSize != nil {
			t.Fatal(r)
		}
	}
}
func TestComputationParserAndDepth(t *testing.T) {
	// QA6, E-12: one MiB is inclusive; depth derives from the actual evidence shape.
	for _, row := range []struct {
		name  string
		data  string
		valid bool
	}{
		{"AtLimit", `"` + strings.Repeat("x", (1<<20)-2) + `"`, true},
		{"PastLimit", `"` + strings.Repeat("x", (1<<20)-1) + `"`, false},
		{"Depth61", strings.Repeat("[", 61) + "0" + strings.Repeat("]", 61), true},
		{"Depth62", strings.Repeat("[", 62) + "0" + strings.Repeat("]", 62), false},
		{"Null", "null", true}, {"TwoValues", "0 0", false}, {"Duplicate", `{"a":0,"a":0}`, false}, {"Surrogate", `"\ud800"`, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			_, ok := ParseComputed([]byte(row.data), false, int64(len(row.data)))
			if ok != row.valid {
				t.Fatal(ok)
			}
			if _, ok := ParseComputed([]byte(row.data), true, int64(len(row.data))); ok {
				t.Fatal("cut accepted")
			}
		})
	}
	depth := 0
	for ; depth <= 64; depth++ {
		v := any(nil)
		for i := 0; i < depth; i++ {
			v = plan.Array{v}
		}
		doc := plan.Object{"evidence": plan.Array{plan.Object{"expected": v}}}
		_, ok := strictjson.Read(strictjson.Write(doc), strictjson.Bounds{Bytes: 1 << 20, Depth: 64, NumberChars: 32, ExponentDigits: 4}, nil)
		if !ok {
			break
		}
	}
	if depth-1 != ComputedValueDepth {
		t.Fatalf("derived depth %d constant %d", depth-1, ComputedValueDepth)
	}
}
func TestSubstitutionRows(t *testing.T) {
	// QA4, E-25: literal forms and executable are preserved, recognized tokens replaced once.
	argv := []string{"{SEED}", "{SEED}/{SUBJECT_DIR}/{CAPTURE:first}", "{CAPTURE:}", "{CAPTURE:bad:name}", "{CAPTURE:unterminated", "{CAPTURE:{CAPTURE:first}}"}
	want := []string{"{SEED}", "seed/subject/capture", "{CAPTURE:}", "{CAPTURE:bad:name}", "{CAPTURE:unterminated", "{CAPTURE:capture}"}
	got := Substitute(argv, "seed", "subject", map[string]string{"first": "capture"})
	if !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	if argv[1] != "{SEED}/{SUBJECT_DIR}/{CAPTURE:first}" {
		t.Fatal("mutated arguments")
	}
	if got := Substitute([]string{"x", "{SEED}"}, "{SUBJECT_DIR}", "subject", nil); got[1] != "{SUBJECT_DIR}" {
		t.Fatal(got)
	}
}
func TestPreconditionsAndSizes(t *testing.T) {
	// QA9, QE8, QS5: observed preconditions survive wrong_exit, and overrides clear fields.
	for _, v := range []any{true, false, "true"} {
		ch, e := computedCheck("stdout_empty", v)
		ex := object(ch.Preimage["expected"])
		ex["precondition"] = plan.Object{"computation": ex["computation"]}
		delete(ex, "computation")
		ex["literal"] = nil
		e.Invocations = append(e.Invocations, observe.Invocation{Run: observe.Run{Markers: observe.Markers{Confinement: true}}})
		for _, code := range []int{0, 1} {
			e.Invocations[1].Run.End.Code = code
			r := Resolve(ch, passing(), e)
			if r.PreconditionObserved == nil || *r.PreconditionObserved != (v == true) {
				t.Fatal(r)
			}
		}
	}
	for _, p := range []string{"stdout_not_contains", "stdout_bytes_at_most", "row_count"} {
		ch, e := computedCheck(p, strictjson.Number("0"))
		switch p {
		case "stdout_not_contains":
			ch.Preimage["expected"] = plan.Object{"literal": "x"}
			e.Invocations = nil
		case "stdout_bytes_at_most":
			ch.Preimage["expected"] = plan.Object{"literal": strictjson.Number("0")}
			e.Invocations = nil
		case "row_count":
			ch.Preimage["path"] = ""
		}
		run := observe.Run{Markers: observe.Markers{Confinement: true}}
		if p == "row_count" {
			run.Stdout = []byte("[]")
		}
		e.Invocations = append(e.Invocations, observe.Invocation{Run: run})
		r := Resolve(ch, passing(), e)
		if r.Outcome != "held" || r.TargetSize == nil || *r.TargetSize != 0 {
			t.Fatal(p, r)
		}
		ch.Subject["binary"] = plan.Object{"placeholder": "supply binary"}
		r = FinalOverride(ch, r)
		if r.TargetSize != nil || r.PreconditionObserved != nil || r.Outcome != NotObserved {
			t.Fatal(r)
		}
	}
}
func TestCapturePlaceholder(t *testing.T) {
	// QS5, E-51: both an included and an outside captured check inherit the dependency.
	for _, required := range []bool{false, true} {
		ch, _ := computedCheck("stdout_json_equals", nil)
		ch.Preimage["runbook"] = "book"
		object(object(ch.Preimage["expected"])["computation"])["captures"] = plan.Array{"first"}
		checks := plan.Array{}
		if required {
			checks = append(checks, plan.Object{"dimension": "availability"})
		}
		ch.Subject = plan.Object{"commands": plan.Array{plan.Object{"words": plan.Array{"blocked"}, "class": plan.Object{"placeholder": "supply class"}, "writes": false}}, "runbooks": plan.Array{plan.Object{"name": "book", "steps": plan.Array{plan.Object{"name": "first", "words": plan.Array{"blocked"}, "checks": checks}}}}}
		ch.Preimage["words"] = plan.Array{}
		if r := level2(ch, passing(), Evidence{}); r.Level != 2 || r.Reason != "placeholder_unfilled" {
			t.Fatal(r)
		}
	}
}

func TestExpectedSourcesForPredicates(t *testing.T) {
	// QA2, QA9: every predicate consumes the same literal or computed expected value.
	for _, row := range []struct {
		p, out string
		v      any
	}{
		{"exit_code", "", strictjson.Number("0")},
		{"stdout_equals", "x", "x"},
		{"stdout_contains", "x", "x"},
		{"stdout_not_contains", "y", "x"},
		{"stderr_contains", "x", "x"},
		{"stdout_json_equals", "1e9999", strictjson.Number("10e9998")},
		{"json_path_equals", "1e9999", strictjson.Number("10e9998")},
		{"row_count", "[1]", strictjson.Number("1")},
		{"stdout_bytes_at_most", "x", strictjson.Number("1")},
	} {
		t.Run(row.p, func(t *testing.T) {
			for _, computed := range []bool{false, true} {
				ch, e := computedCheck(row.p, row.v)
				ch.Preimage["path"] = ""
				if !computed {
					ch.Preimage["expected"] = plan.Object{"literal": row.v}
					e.Invocations = nil
				}
				e.Invocations = append(e.Invocations, observe.Invocation{Run: observe.Run{Stdout: []byte(row.out), Stderr: []byte(row.out), StdoutSeen: int64(len(row.out)), Markers: observe.Markers{Confinement: true}}})
				r := Resolve(ch, passing(), e)
				if r.Outcome != "held" || !strictjson.EqualJSON(r.Expected, row.v) {
					t.Fatal(computed, r)
				}
			}
		})
	}
}
func TestDestructiveSentinel(t *testing.T) {
	// QA8: any destructive step makes the included runbook wait for PR13.
	ch := plan.Check{Subject: plan.Object{"commands": plan.Array{plan.Object{"words": plan.Array{"delete"}, "class": "destructive"}}, "runbooks": plan.Array{plan.Object{"name": "book", "steps": plan.Array{plan.Object{"words": plan.Array{"delete"}}}}}}, Preimage: plan.Object{"runbook": "book", "earlier": plan.Array{}}}
	r := level4(ch, passing(), Evidence{})
	if r.Unbuilt == nil || r.Unbuilt.Rule != "QA8" || r.Level != 4 {
		t.Fatal(r)
	}
}
func TestByteBoundSizes(t *testing.T) {
	// QA9, QE8: the full drained count decides, even when output was cut.
	for _, row := range []struct {
		name  string
		count int64
		held  bool
	}{{"Below", 0, true}, {"At", 1, true}, {"Past", 2, false}} {
		t.Run(row.name, func(t *testing.T) {
			ch := plan.Check{Preimage: plan.Object{"predicate": "stdout_bytes_at_most", "expected": plan.Object{"literal": strictjson.Number("1")}}}
			r := level8(ch, passing(), Evidence{Invocations: []observe.Invocation{{Run: observe.Run{StdoutCut: true, StdoutSeen: row.count}}}})
			if (r.Outcome == "held") != row.held || r.TargetSize == nil || *r.TargetSize != row.count {
				t.Fatal(r)
			}
		})
	}
}

func TestComputedRowCountMissingArray(t *testing.T) {
	// QA9, QE8: a computed row count reports null for a missing or non-array target.
	for _, out := range []string{`{"a":1}`, `{"b":[]}`, `not_json`} {
		ch, e := computedCheck("row_count", strictjson.Number("0"))
		ch.Preimage["path"] = "/a"
		e.Invocations = append(e.Invocations, observe.Invocation{Run: observe.Run{Stdout: []byte(out), Markers: observe.Markers{Confinement: true}}})
		r := Resolve(ch, passing(), e)
		if r.FailureKind != "predicate" || r.TargetSize != nil {
			t.Fatal(r)
		}
	}
}
func TestSharedComputedValue(t *testing.T) {
	// QA2, E-52: two equal computations read the one available run.
	a, e := computedCheck("stdout_json_equals", strictjson.Number("1"))
	b, _ := computedCheck("json_path_equals", strictjson.Number("1"))
	if !strictjson.EqualJSON(Expected(a, e), strictjson.Number("1")) || !strictjson.EqualJSON(Expected(b, e), strictjson.Number("1")) {
		t.Fatal("shared value changed")
	}
}
func TestDiscoveryOverrideFields(t *testing.T) {
	// QE8, QD3, QS5: both overrides clear sizes and preconditions on not_observed.
	n := int64(1)
	b := true
	ch := plan.Check{Subject: plan.Object{}, Preimage: plan.Object{"dimension": "availability"}}
	r := Result{Outcome: "held", Level: 8, TargetSize: &n, PreconditionObserved: &b}
	r = DiscoveryOverride(ch, r, &Discovery{Ran: true, Undeclared: true})
	if r.Outcome != NotObserved || r.TargetSize != nil || r.PreconditionObserved != nil {
		t.Fatal(r)
	}
}
func TestComputationIdentityGuards(t *testing.T) {
	// QA4, QA6: a run from another step or runbook is never the missing computation or capture.
	for _, key := range []string{"runbook", "step"} {
		ch, e := computedCheck("stdout_json_equals", nil)
		e.Invocations[0].Key[key] = "other"
		mustFault(t, func() { level6(ch, passing(), e) })
	}
	ch, e := computedCheck("stdout_json_equals", nil)
	object(object(ch.Preimage["expected"])["computation"])["captures"] = plan.Array{"first"}
	e.Captures = []observe.Invocation{{Key: observe.Key{"step": "first", "runbook": "other"}}}
	mustFault(t, func() { level6(ch, passing(), e) })
}

func TestNotContainsTargetSize(t *testing.T) {
	// QA9, QE8: evaluated byte sizes are retained for held and predicate failures only.
	ch := plan.Check{Subject: plan.Object{}, Preimage: plan.Object{"probe": nil, "earlier": plan.Array{}, "predicate": "stdout_not_contains", "expected": plan.Object{"literal": "x"}}}
	for _, out := range []string{"abc", "x"} {
		e := Evidence{Invocations: []observe.Invocation{{Run: observe.Run{Stdout: []byte(out), StdoutSeen: int64(len(out)), Markers: observe.Markers{Confinement: true}}}}}
		r := Resolve(ch, passing(), e)
		if r.TargetSize == nil || *r.TargetSize != int64(len(out)) || (r.Outcome == "held") != (out == "abc") {
			t.Fatal(r)
		}
		e.Invocations[0].Run.End.Code = 1
		r = Resolve(ch, passing(), e)
		if r.FailureKind != "wrong_exit" || r.TargetSize != nil {
			t.Fatal(r)
		}
	}
}

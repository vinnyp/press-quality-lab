package subject

import (
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"strings"
	"testing"
)

func TestPR8Declarations(t *testing.T) {
	// QS2, QS5, QS6, QS7, QA4, QP10: each mutation starts at an accepted control.
	rows := []struct {
		name, rule string
		change     func(File)
	}{
		{"CommandFraction", "QS6", func(s File) { testObject(testArray(s["commands"])[0])["budget_seconds"] = strictjson.Number("1.5") }},
		{"CommandString", "QS6", func(s File) { testObject(testArray(s["commands"])[0])["budget_seconds"] = "10" }},
		{"CommandDecimal", "QS6", func(s File) { testObject(testArray(s["commands"])[0])["budget_seconds"] = strictjson.Number("10.0") }},
		{"ComputationFraction", "QA4", func(s File) {
			c := testComputation("/bin/echo")
			c["budget_seconds"] = strictjson.Number("1.5")
			firstCheck(s)["expected"] = strictjson.Object{"computation": c}
		}},
		{"ComputationString", "QA4", func(s File) {
			c := testComputation("/bin/echo")
			c["budget_seconds"] = "10"
			firstCheck(s)["expected"] = strictjson.Object{"computation": c}
		}},
		{"ComputationDecimal", "QA4", func(s File) {
			c := testComputation("/bin/echo")
			c["budget_seconds"] = strictjson.Number("10.0")
			firstCheck(s)["expected"] = strictjson.Object{"computation": c}
		}},
		{"ByteBoundType", "QS6", func(s File) { obj(arr(s["commands"])[0])["byte_bound"] = "10" }},
		{"EnvType", "QS7", func(s File) { obj(arr(s["offline"])[0])["env"] = strictjson.Object{"OK": true} }},
		{"AppliesElementBound", "QS2", func(s File) {
			w := strictjson.Array{}
			for i := 0; i < 65; i++ {
				w = append(w, "x")
			}
			obj(arr(s["commands"])[0])["words"] = strictjson.Array{"x"}
			obj(arr(s["offline"])[0])["applies_to"] = strictjson.Array{w}
		}},
		{"AppliesWordType", "QS7", func(s File) {
			obj(arr(s["offline"])[0])["applies_to"] = strictjson.Array{strictjson.Array{strictjson.Number("1")}}
		}},
		{"AppliesElementType", "QS7", func(s File) { obj(arr(s["offline"])[0])["applies_to"] = strictjson.Array{true} }},
		{"AppliesType", "QS7", func(s File) { obj(arr(s["offline"])[0])["applies_to"] = "x" }},
		{"ReservedEmpty", "QS7", func(s File) {
			o := obj(arr(s["offline"])[0])
			o["applies_to"] = strictjson.Array{}
			o["env"] = strictjson.Object{"HOME": "x"}
		}},
		{"ArgvType", "QA4", func(s File) {
			c := testComputation("/bin/echo")
			c["argv"] = "x"
			firstCheck(s)["expected"] = strictjson.Object{"computation": c}
		}},
		{"ArgvElement", "QA4", func(s File) {
			c := testComputation("/bin/echo")
			c["argv"] = strictjson.Array{"/bin/echo", true}
			firstCheck(s)["expected"] = strictjson.Object{"computation": c}
		}},
		{"ArgvEmpty", "QA4", func(s File) {
			c := testComputation("/bin/echo")
			c["argv"] = strictjson.Array{}
			firstCheck(s)["expected"] = strictjson.Object{"computation": c}
		}},
		{"PrefixControl_dyld_x", "", func(s File) { obj(arr(s["offline"])[0])["env"] = strictjson.Object{"dyld_x": "x"} }},
		{"PrefixControl___cf_x", "", func(s File) { obj(arr(s["offline"])[0])["env"] = strictjson.Object{"__cf_x": "x"} }},
		{"PrefixControl_XDYLD_A", "", func(s File) { obj(arr(s["offline"])[0])["env"] = strictjson.Object{"XDYLD_A": "x"} }},
		{"PrefixControl_DYLD", "", func(s File) { obj(arr(s["offline"])[0])["env"] = strictjson.Object{"DYLD": "x"} }},
		{"Accepteddyld_x", "", func(s File) { obj(arr(s["offline"])[0])["env"] = strictjson.Object{"dyld_x": "x"} }},
		{"Accepted__cf_x", "", func(s File) { obj(arr(s["offline"])[0])["env"] = strictjson.Object{"__cf_x": "x"} }},
		{"AcceptedXDYLD_A", "", func(s File) { obj(arr(s["offline"])[0])["env"] = strictjson.Object{"XDYLD_A": "x"} }},
		{"AcceptedDYLD", "", func(s File) { obj(arr(s["offline"])[0])["env"] = strictjson.Object{"DYLD": "x"} }},
		{"NestedCapture", "", func(s File) {
			book := obj(arr(s["runbooks"])[0])
			steps := arr(book["steps"])
			obj(steps[0])["name"] = "b"
			c := testComputation("/bin/echo")
			c["argv"] = strictjson.Array{"/bin/echo", "{CAPTURE:x {CAPTURE:b}"}
			c["captures"] = strictjson.Array{"b"}
			second := strictjson.Object{"name": "later", "words": obj(steps[0])["words"], "budget_seconds": strictjson.Number("1"), "checks": strictjson.Array{strictjson.Object{"dimension": "response-correctness", "predicate": "stdout_equals", "path": nil, "expected": strictjson.Object{"computation": c}, "precondition": nil, "exit_codes": nil}}}
			book["steps"] = append(steps, second)
		}},
		{"DYLD", "QS7", func(s File) { testObject(testArray(s["offline"])[0])["env"] = strictjson.Object{"DYLD_SYNTHETIC": "x"} }},
		{"CF", "QS7", func(s File) { testObject(testArray(s["offline"])[0])["env"] = strictjson.Object{"__CF_SYNTHETIC": "x"} }},
		{"LiteralPlaceholder", "", func(s File) {
			firstCheck(s)["expected"] = strictjson.Object{"literal": strictjson.Object{"placeholder": "x"}}
		}},
		{"LiteralLookalike", "", func(s File) {
			firstCheck(s)["expected"] = strictjson.Object{"literal": strictjson.Object{"placeholder": "x", "k": strictjson.Number("1")}}
		}},
		{"UnterminatedCapture", "", func(s File) {
			c := testComputation("/bin/echo")
			c["argv"] = strictjson.Array{"/bin/echo", "{CAPTURE:"}
			firstCheck(s)["expected"] = strictjson.Object{"computation": c}
		}},
		{"CommandBudget", "QS2", func(s File) { testObject(testArray(s["commands"])[0])["budget_seconds"] = strictjson.Number("0") }},
		{"ComputationBudget", "QS2", func(s File) {
			c := testComputation("/bin/echo")
			c["budget_seconds"] = strictjson.Number("0")
			firstCheck(s)["expected"] = strictjson.Object{"computation": c}
		}},
		{"EnvironmentName", "QS2", func(s File) { testObject(testArray(s["offline"])[0])["env"] = strictjson.Object{"1BAD": "x"} }},
		{"CredentialReasonAbsent", "QS2", func(s File) { delete(obj(s["secret"]), "reason") }},
		{"PlaceholderDescriptionBound", "QS2", func(s File) { s["binary"] = strictjson.Object{"placeholder": strings.Repeat("x", 4097)} }},
		{"RootExtra", "QS2", func(s File) { s["extra"] = true }},
		{"SecretBareSuffix", "QS6", func(s File) { obj(s["secret"])["commands"] = strictjson.Array{strictjson.Array{"undeclared"}} }},
		{"SecretPrefix", "QP10", func(s File) {
			w := testObject(testArray(s["commands"])[1])["words"]
			words := append(strictjson.Array{}, testArray(w)...)
			words = append(words, "extra")
			testObject(s["secret"])["commands"] = strictjson.Array{words}
		}},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			s := parsedFixture(t)
			checkFixture(t, s, "")
			r.change(s)
			if r.name == "AppliesElementBound" || r.name == "AppliesWordType" {
				// QS2, QS7: inspect schema failures before semantic refusal can mask them.
				failures := map[string]bool{}
				subjectShape().check(s, failures)
				if len(failures) != 1 || !failures[r.rule] {
					t.Fatalf("schema failures %v", failures)
				}
				if r.name == "AppliesElementBound" {
					return
				}
			}
			checkFixture(t, s, r.rule)
		})
	}
}

func TestPR8FileRules(t *testing.T) {
	// QS9, QP7, QP10: direct file evidence isolates each refusal from token counting.
	for _, row := range []struct {
		name, rule string
		change     func(File, *FileEvidence)
	}{
		{"StoreNoSeed", "QS9", func(s File, e *FileEvidence) { s["seed"] = nil; e.Seed = nil }},
		{"StoreDirectory", "QS9", func(s File, e *FileEvidence) { e.Seed[0].Directory = true }},
		{"BookkeepingAncestor", "QP7", func(s File, e *FileEvidence) {
			s["bookkeeping"] = strictjson.Array{"other"}
			e.Seed = append(e.Seed, FileEntry{Path: "other/file", Digest: "c"})
		}},
		{"SecretAbsent", "QP10", func(s File, e *FileEvidence) { obj(s["secret"])["path"] = "missing" }},
	} {
		t.Run(row.name, func(t *testing.T) {
			s := File{"seed": "seed", "store": "state/data", "bookkeeping": strictjson.Array{}, "secret": strictjson.Object{"path": "token"}}
			e := FileEvidence{Seed: []FileEntry{{Path: "state/data", Digest: "a"}, {Path: "token", Digest: "b"}}, SecretOccurrences: 1}
			if row.name == "StoreNoSeed" {
				s["secret"] = nil
			}
			assertRule(t, CheckFiles(s, e), "")
			row.change(s, &e)
			assertRule(t, CheckFiles(s, e), row.rule)
		})
	}
}

func TestPR8EmptyExpected(t *testing.T) {
	// QA9: stdout_empty requires a null literal even with its required precondition.
	for _, expected := range []strictjson.Object{{"literal": "nonempty"}, {"computation": testComputation("/bin/echo")}} {
		s := parsedFixture(t)
		c := firstCheck(s)
		c["predicate"] = "stdout_empty"
		c["path"] = nil
		c["expected"] = strictjson.Object{"literal": nil}
		c["precondition"] = strictjson.Object{"computation": testComputation("/bin/echo")}
		checkFixture(t, s, "")
		c["expected"] = expected
		checkFixture(t, s, "QA9")
	}
}

func TestPR8ShapeFlags(t *testing.T) {
	// QS2, QS9: schema-only bounds and missing credential reasons have explicit attribution.
	s := named("QS9", schemaText(1))
	s.max = 8
	failures := map[string]bool{}
	s.check("accepted", failures)
	if len(failures) != 0 {
		t.Fatal(failures)
	}
	s.check("too_long_value", failures)
	if len(failures) != 1 || !failures["QS2"] {
		t.Fatal(failures)
	}
	failures = map[string]bool{}
	conditional := object("QS9", map[string]*shape{"note": reason(), "kind": {kind: "any"}})
	conditional.check(strictjson.Object{"kind": "credential", "note": nil}, failures)
	if len(failures) != 0 {
		t.Fatal(failures)
	}
	conditional.check(strictjson.Object{"kind": "credential"}, failures)
	if len(failures) != 1 || !failures["QS2"] {
		t.Fatal(failures)
	}
}

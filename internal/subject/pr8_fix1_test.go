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
		{"BookkeepingAncestor", "QP7", func(s File, e *FileEvidence) { s["bookkeeping"] = strictjson.Array{"state"} }},
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

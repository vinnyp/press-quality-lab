package judge

import (
	"fmt"
	"strings"
	"testing"
)

func TestRequestClauses(t *testing.T) {
	tests := []struct {
		name string
		rule Rule
		edit func(fixture)
	}{
		{"X1 empty", X1, func(f fixture) { f.in["scope"] = array{} }},
		{"X1 duplicate", X1, func(f fixture) { f.in["scope"] = array{"conformance", "conformance"} }},
		{"X1 unknown", X1, func(f fixture) { f.in["scope"] = array{"unknown"} }},
		{"X1 destructive dependency", X1, func(f fixture) { f.in["scope"] = array{"destructive"} }},
		{"X1 compare dependency", X1, func(f fixture) { f.in["scope"] = array{"compare"} }},
		{"X2 unknown evidence", X2, func(f fixture) { f.in["evidence"] = append(arr(f.in["evidence"]), evidence("absent")) }},
		{"X2 unknown trial", X2, func(f fixture) { f.in["trials"] = array{trial("absent", "", "current", 0)} }},
		{"X2 evidence prompt", X2, func(f fixture) {
			c := required("prompt", "prompt", "steering")
			f.in["required"] = append(arr(f.in["required"]), c)
			f.in["evidence"] = append(arr(f.in["evidence"]), evidence("prompt"))
		}},
		{"X2 evidence comparison", X2, func(f fixture) {
			c := required("compare", "comparison", "tool-description-efficacy")
			f.in["required"] = append(arr(f.in["required"]), c)
			f.in["evidence"] = append(arr(f.in["evidence"]), evidence("compare"))
		}},
		{"X2 trial plain", X2, func(f fixture) { f.in["trials"] = array{trial("execution", "", "current", 0)} }},
		{"X2 trial comparison", X2, func(f fixture) {
			f.in["required"] = append(arr(f.in["required"]), required("compare", "comparison", "tool-description-efficacy"))
			f.in["trials"] = array{trial("compare", "", "current", 0)}
		}},
		{"X3 duplicate evidence", X3, func(f fixture) { f.in["evidence"] = append(arr(f.in["evidence"]), evidence("execution")) }},
		{"X3 missing reason", X3, func(f fixture) { f.e("execution")["outcome"] = "not_observed" }},
		{"X3 held reason", X3, func(f fixture) { f.e("execution")["reason"] = "harness_fault" }},
		{"X3 failed reason", X3, func(f fixture) {
			f.e("execution")["outcome"] = "not_held"
			f.e("execution")["reason"] = "harness_fault"
		}},
		{"X4 safety", X4, func(f fixture) { f.mark("idempotency", "idempotency") }},
		{"X4 steering", X4, func(f fixture) {
			f.in["required"] = append(arr(f.in["required"]), required("prompt", "prompt", "steering"))
			f.mark("prompt", "steering")
		}},
		{"X4 parity", X4, func(f fixture) {
			f.in["required"] = append(arr(f.in["required"]), required("parity", "plain", "cli-mcp-parity"))
			f.mark("parity", "cli-mcp-parity")
		}},
		{"X4 no record", X4, func(f fixture) { f.mark("absent", "execution") }},
		{"X4 noncrediting", X4, func(f fixture) { f.c("execution")["crediting"] = false; f.mark("execution", "execution") }},
		{"X5 both empty", X5, func(f fixture) {
			f.c("execution")["empty_satisfiable"] = true
			f.c("execution")["asserts_emptiness"] = true
			f.c("execution")["precondition"] = "sample"
		}},
		{"X5 absent precondition", X5, func(f fixture) { f.c("execution")["asserts_emptiness"] = true }},
		{"X5 prompt credit", X5, func(f fixture) {
			c := required("prompt", "prompt", "steering")
			c["crediting"] = false
			f.in["required"] = append(arr(f.in["required"]), c)
		}},
		{"X5 compare credit", X5, func(f fixture) {
			f.in["scope"] = array{"steering", "compare"}
			c := required("compare", "comparison", "tool-description-efficacy")
			c["crediting"] = false
			f.in["required"] = append(arr(f.in["required"]), c)
		}},
		{"X5 count", X5, func(f fixture) { f.c("execution")["declared_count"] = number("16") }},
		{"X5 condition", X5, func(f fixture) { f.c("execution")["condition"] = "" }},
		{"X5 condition A", X5, func(f fixture) { f.c("execution")["condition_a"] = "" }},
		{"X10 duplicate", X10, func(f fixture) { f.in["required"] = append(arr(f.in["required"]), f.c("execution")) }},
		{"X10 prompt routing", X10, func(f fixture) {
			c := required("prompt", "prompt", "execution")
			c["condition"] = "current"
			c["declared_count"] = number("16")
			f.in["required"] = append(arr(f.in["required"]), c)
		}},
		{"X10 plain routing", X10, func(f fixture) {
			f.in["required"] = append(arr(f.in["required"]), required("plain", "plain", "steering"))
		}},
		{"X10 comparison routing", X10, func(f fixture) {
			f.in["scope"] = array{"steering", "compare"}
			f.in["required"] = append(arr(f.in["required"]), required("compare", "comparison", "execution"))
		}},
		{"X10 comparison absent", X10, func(f fixture) { f.in["scope"] = array{"steering", "compare"} }},
		{"X10 comparison extra", X10, func(f fixture) {
			f.in["scope"] = array{"steering", "compare"}
			f.in["required"] = append(arr(f.in["required"]), required("a", "comparison", "tool-description-efficacy"), required("b", "comparison", "tool-description-efficacy"))
		}},
		{"X10 comparison outside scope", X10, func(f fixture) {
			f.in["required"] = append(arr(f.in["required"]), required("compare", "comparison", "tool-description-efficacy"))
		}},
		{"X11 retired current", X11, func(f fixture) { f.in["request"] = "anchor"; f.in["retired"] = array{"execution"} }},
		{"X11 grade retired", X11, func(f fixture) { f.in["retired"] = array{"absent"} }},
		{"X11 grade accepted", X11, func(f fixture) { f.in["accepted_regressions"] = array{"execution"} }},
		{"X11 duplicate baseline", X11, func(f fixture) {
			e := baselineEntry("execution", "plain", "execution", "held")
			f.in["baseline"] = object{"entries": array{e, e}}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := conformanceFixture()
			tt.edit(f)
			assertBytes(t, write(f.in), rejectedExpected(tt.rule))
		})
	}
	for _, field := range []string{"condition", "k", "n"} {
		t.Run("X11 absent "+field, func(t *testing.T) {
			f := steeringFixture()
			e := baselineEntry("prompt", "prompt", "steering", "held")
			e[field] = nil
			f.in["baseline"] = object{"entries": array{e}}
			assertBytes(t, write(f.in), rejectedExpected(X11))
		})
	}
	for _, tt := range []struct {
		name string
		k, n string
	}{{"zero n", "0", "0"}, {"k above n", "17", "16"}} {
		t.Run("X11 "+tt.name, func(t *testing.T) {
			f := steeringFixture()
			e := baselineEntry("prompt", "prompt", "steering", "held")
			e["k"], e["n"] = number(tt.k), number(tt.n)
			f.in["baseline"] = object{"entries": array{e}}
			assertBytes(t, write(f.in), rejectedExpected(X11))
		})
	}
	t.Run("X11 overflow guard", func(t *testing.T) {
		f := conformanceFixture()
		entries := array{}
		for i := 0; i < 1000; i++ {
			e := baselineEntry(fmt.Sprint(i), "prompt", "steering", "held")
			e["n"] = number("9007199254740991")
			entries = append(entries, e)
		}
		f.in["baseline"] = object{"entries": entries}
		assertBytes(t, write(f.in), rejectedExpected(X11))
	})
}
func TestPromptRequestClauses(t *testing.T) {
	for _, tt := range []struct {
		name string
		rule Rule
		edit func(fixture)
	}{
		{"X6 duplicate", X6, func(f fixture) { f.in["trials"] = append(arr(f.in["trials"]), trial("prompt", "", "current", 0)) }},
		{"X6 gap", X6, func(f fixture) { obj(arr(f.in["trials"])[15])["trial_index"] = number("16") }},
		{"X6 index maximum", X6, func(f fixture) {
			f.in["trials"] = array{trial("prompt", "", "current", 0)}
			obj(arr(f.in["trials"])[0])["trial_index"] = number("9007199254740991")
		}},
		{"X6 count", X6, func(f fixture) { obj(arr(f.in["trials"])[0])["declared_count"] = number("17") }},
		{"X6 wrong tier gap", X6, func(f fixture) {
			f.in["trials"] = array{trial("prompt", "", "current", 1)}
			obj(arr(f.in["trials"])[0])["tier"] = "scripted"
		}},
		{"X7 floor", X7, func(f fixture) { f.in["trials"] = array{}; f.c("prompt")["declared_count"] = number("15") }},
		{"X8 count", X8, func(f fixture) { f.c("prompt")["declared_count"] = nil }},
		{"X8 condition", X8, func(f fixture) { f.c("prompt")["condition"] = nil }},
		{"X8 condition A", X8, func(f fixture) {
			f.in["scope"] = array{"steering", "compare"}
			f.in["required"] = append(arr(f.in["required"]), required("compare", "comparison", "tool-description-efficacy"))
			for _, v := range arr(f.in["trials"]) {
				obj(v)["arm"] = "B"
			}
		}},
		{"X9 outcome", X9, func(f fixture) { obj(arr(f.in["trials"])[0])["outcome"] = "not_observed" }},
		{"X9 evaluator", X9, func(f fixture) { obj(obj(arr(f.in["trials"])[0])["evaluator"])["kind"] = "model" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := steeringFixture()
			tt.edit(f)
			assertBytes(t, write(f.in), rejectedExpected(tt.rule))
		})
	}
}
func TestRequestOrder(t *testing.T) {
	for _, tt := range []struct {
		name string
		rule Rule
		edit func(fixture)
	}{
		{"X12 before X1", X12, func(f fixture) { f.in["scope"] = array{}; f.in["extra"] = true }},
		{"X1 before X2", X1, func(f fixture) { f.in["scope"] = array{}; f.in["evidence"] = array{evidence("absent")} }},
		{"X2 before X3", X2, func(f fixture) { e := evidence("absent"); e["reason"] = "harness_fault"; f.in["evidence"] = array{e} }},
		{"X3 before X4", X3, func(f fixture) {
			e := evidence("prompt")
			e["outcome"] = "not_observed"
			c := required("prompt", "plain", "execution")
			f.in["required"] = array{c}
			f.in["trials"] = array{}
			f.in["evidence"] = array{e}
			f.mark("absent", "execution")
		}},
		{"X4 before X5", X4, func(f fixture) { f.mark("absent", "execution"); f.c("prompt")["crediting"] = false }},
		{"X5 before W1", X5, func(f fixture) { obj(arr(f.in["trials"])[0])["tier"] = "scripted"; f.c("prompt")["crediting"] = false }},
		{"X6 before X7", X6, func(f fixture) { f.c("prompt")["declared_count"] = number("15") }},
		{"X7 before X8", X7, func(f fixture) {
			f.in["trials"] = array{}
			f.c("prompt")["declared_count"] = number("15")
			f.c("prompt")["condition"] = nil
		}},
		{"X8 before X9", X8, func(f fixture) {
			f.c("prompt")["declared_count"] = nil
			obj(arr(f.in["trials"])[0])["outcome"] = "not_observed"
		}},
		{"X9 before X10", X9, func(f fixture) {
			f.in["required"] = append(arr(f.in["required"]), f.c("prompt"))
			obj(arr(f.in["trials"])[0])["outcome"] = "not_observed"
		}},
		{"X10 before X11", X10, func(f fixture) {
			f.in["required"] = append(arr(f.in["required"]), f.c("prompt"))
			f.in["retired"] = array{"prompt"}
		}},
		{"shared id X2 sees every kind", X2, func(f fixture) {
			f.in["required"] = append(arr(f.in["required"]), required("prompt", "plain", "execution"))
		}},
		{"shared id X3 before X10", X3, func(f fixture) {
			f.in["required"] = array{required("same", "plain", "execution"), required("same", "plain", "availability")}
			f.in["trials"] = array{}
			f.in["evidence"] = array{evidence("same"), evidence("same")}
		}},
		{"shared id X6 own conditions", X6, func(f fixture) {
			c := required("prompt", "prompt", "steering")
			c["condition"] = "other"
			c["declared_count"] = number("16")
			f.in["required"] = append(arr(f.in["required"]), c)
			f.in["trials"] = append(arr(f.in["trials"]), trial("prompt", "", "other", 1))
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := steeringFixture()
			tt.edit(f)
			assertBytes(t, write(f.in), rejectedExpected(tt.rule))
		})
	}
}
func TestEscapedKeysAndValues(t *testing.T) {
	f := conformanceFixture()
	data := strings.Replace(string(write(f.in)), `"request":"grade"`, `"requ\u0065st":"gr\u0061de"`, 1)
	assertBytes(t, []byte(data), f.want)
}

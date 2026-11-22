package judge

import (
	"bytes"
	"fmt"
	"testing"
)

func TestX1SingleScopeCauses(t *testing.T) {
	for _, tt := range []struct {
		name  string
		scope array
	}{
		{"empty", array{}}, {"destructive_requires_conformance", array{"destructive"}},
		{"compare_requires_steering", array{"compare"}},
		{"unknown", array{"conformance", "other"}}, {"empty_part", array{"conformance", ""}},
		{"case_sensitive", array{"conformance", "Steering"}}, {"joined_parts", array{"conformance+steering"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := conformanceFixture()
			f.in["scope"] = tt.scope
			assertBytes(t, write(f.in), rejectedExpected(X1))
		})
	}
	for _, part := range []string{"conformance", "destructive", "steering", "compare"} {
		t.Run("duplicate_"+part, func(t *testing.T) {
			f := allScopeFixture(15)
			f.in["scope"] = append(arr(f.in["scope"]), part)
			assertBytes(t, write(f.in), rejectedExpected(X1))
		})
	}
}

// Replace a string placeholder after writing: the production writer deliberately
// canonicalizes integer -0, so number("-0") would not test the input spelling.
func rawIntegerInput(f fixture) []byte {
	return bytes.ReplaceAll(write(f.in), []byte(`"wire-negative-zero"`), []byte(`-0`))
}

func TestX6X7NegativeZeroByValue(t *testing.T) {
	for _, arm := range []string{"", "A", "B"} {
		for _, cause := range []string{"contiguous_index", "duplicate_index", "equal_counts", "unequal_counts", "required_zero"} {
			t.Run(fmt.Sprintf("%s_%s", arm, cause), func(t *testing.T) {
				f := steeringFixture()
				if arm != "" {
					f = compareFixture()
				}
				switch cause {
				case "contiguous_index", "duplicate_index":
					for _, v := range arr(f.in["trials"]) {
						tr := obj(v)
						if str(tr["arm"]) == arm && integer(tr["trial_index"]) == 0 {
							tr["trial_index"] = "wire-negative-zero"
						}
					}
					if cause == "duplicate_index" {
						f.in["trials"] = append(arr(f.in["trials"]), trial("prompt", arm, "current", 0))
					}
				case "equal_counts", "unequal_counts", "required_zero":
					f.c("prompt")["declared_count"] = number("0")
					if cause == "required_zero" {
						f.c("prompt")["declared_count"] = "wire-negative-zero"
					}
					for _, v := range arr(f.in["trials"]) {
						tr := obj(v)
						tr["declared_count"] = number("0")
						if str(tr["arm"]) == arm {
							tr["declared_count"] = "wire-negative-zero"
							if cause == "unequal_counts" {
								tr["declared_count"] = number("1")
							}
						}
					}
				}
				input := rawIntegerInput(f)
				if cause != "unequal_counts" && !bytes.Contains(input, []byte(`-0`)) {
					t.Fatal("missing raw negative zero")
				}
				switch cause {
				case "contiguous_index":
					assertBytes(t, input, f.want)
				case "duplicate_index", "unequal_counts":
					assertBytes(t, input, rejectedExpected(X6))
				default:
					assertBytes(t, input, rejectedExpected(X7))
				}
			})
		}
	}
	// A nonzero required count against -0 isolates X6 from X7.
	t.Run("trial_zero_against_sixteen", func(t *testing.T) {
		f := steeringFixture()
		obj(arr(f.in["trials"])[0])["declared_count"] = "wire-negative-zero"
		assertBytes(t, rawIntegerInput(f), rejectedExpected(X6))
	})
}

func TestX10SingleConditions(t *testing.T) {
	for _, cause := range []string{"duplicate_id", "prompt_route", "comparison_route", "plain_steering", "plain_efficacy", "compare_missing", "compare_multiple", "comparison_without_compare"} {
		t.Run(cause, func(t *testing.T) {
			f := steeringFixture()
			switch cause {
			case "duplicate_id":
				f.in["required"] = append(arr(f.in["required"]), f.c("prompt"))
			case "prompt_route":
				f.c("prompt")["dimension"] = "execution"
			case "comparison_route":
				f = compareFixture()
				f.c("compare")["dimension"] = "steering"
			case "plain_steering", "plain_efficacy":
				d := "steering"
				if cause == "plain_efficacy" {
					d = "tool-description-efficacy"
				}
				f.in["required"] = append(arr(f.in["required"]), required("plain", "plain", d))
			case "compare_missing":
				f = compareFixture()
				f.in["required"] = without(arr(f.in["required"]), "id", "compare")
			case "compare_multiple":
				f = compareFixture()
				f.in["required"] = append(arr(f.in["required"]), required("second", "comparison", "tool-description-efficacy"))
			case "comparison_without_compare":
				f.in["required"] = append(arr(f.in["required"]), required("comparison", "comparison", "tool-description-efficacy"))
			}
			assertBytes(t, write(f.in), rejectedExpected(X10))
		})
	}
}

func TestX11SumUsesEntryOwnKind(t *testing.T) {
	for _, kind := range []string{"plain", "comparison", "prompt"} {
		for _, inside := range []bool{false, true} {
			for _, present := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s_inside=%v_present=%v", kind, inside, present), func(t *testing.T) {
					f := conformanceFixture()
					d, id := "steering", "absent"
					if inside {
						d = "execution"
					}
					if present {
						id = "execution"
					}
					a := baselineEntry("maximum", "prompt", "steering", "held")
					a["n"] = number("9007199254740991")
					b := baselineEntry(id, kind, d, "held")
					b["n"], b["k"], b["condition"] = number("1"), number("1"), "old"
					// The sum depends only on the entry kind, even with obsolete metadata,
					// absent current checks, or a current plain check under the same id.
					b["rulebook_version"] = "1.0-draft"
					if kind == "prompt" {
						f.in["baseline"] = object{"entries": array{a, b}}
						assertBytes(t, write(f.in), rejectedExpected(X11))
						return
					}
					classes := array{entryClass("maximum", "set_aside", B2)}
					result, rule := "none", B1
					newIDs := array{}
					if inside {
						result, rule = "not_measured", R2
						class, er := "missing", E1
						if present {
							class, er = "not_compared", E2
						}
						classes = append(classes, entryClass(id, class, er))
						newIDs = expectedNewConformanceExcept(id)
						f.runVerdict("PROVISIONAL", "gaps", "not_measured", 4)
					} else {
						classes = append(classes, entryClass(id, "set_aside", B2))
					}
					sortObjects(classes, "check_id")
					setBaseline(f, array{a, b}, classes, result, rule, newIDs)
					assertFixture(t, f)
				})
			}
		}
	}
}

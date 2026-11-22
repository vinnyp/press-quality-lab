package judge

import (
	"fmt"
	"testing"
)

func TestRequestScopeCombinations(t *testing.T) {
	parts := []string{"conformance", "destructive", "steering", "compare"}
	for mask := 0; mask < 16; mask++ {
		t.Run(fmt.Sprint(mask), func(t *testing.T) {
			f := conformanceFixture()
			scope := array{}
			for i, s := range parts {
				if mask&(1<<i) != 0 {
					scope = append(scope, s)
				}
			}
			f.in["scope"] = scope
			legal := mask != 0 && (mask&2 == 0 || mask&1 != 0) && (mask&8 == 0 || mask&4 != 0)
			if !legal {
				assertBytes(t, write(f.in), rejectedExpected(X1))
				return
			}
			// Valid scopes, including all supersets, are checked with complete outputs below.
			assertFixture(t, allScopeFixture(mask))
			f.in["scope"] = append(scope, scope[len(scope)-1])
			assertBytes(t, write(f.in), rejectedExpected(X1))
		})
	}
}

func TestMarkerRoutingAndCrediting(t *testing.T) {
	for _, d := range fixtureDimensions {
		if d != "side-effect-integrity" && d != "idempotency" && d != "destructive-guard" && d != "pii-secret-hygiene" && d != "steering" && d != "tool-description-efficacy" && d != "cli-mcp-parity" {
			continue
		}
		t.Run("forbidden_"+d, func(t *testing.T) {
			f := conformanceFixture()
			c := required("marked", "plain", d)
			f.in["required"] = append(arr(f.in["required"]), c)
			f.mark("marked", d)
			assertBytes(t, write(f.in), rejectedExpected(X4))
		})
	}
	for _, kind := range []string{"plain", "prompt", "comparison"} {
		for _, cause := range []string{"no record", "wrong dimension", "not crediting"} {
			t.Run(kind+"_"+cause, func(t *testing.T) {
				f := conformanceFixture()
				c := required("marked", kind, "execution")
				c["record"] = "record"
				f.in["required"] = append(arr(f.in["required"]), c)
				switch cause {
				case "no record":
					f.mark("other", "execution")
				case "wrong dimension":
					f.mark("record", "availability")
				case "not crediting":
					c["crediting"] = false
					f.mark("record", "execution")
				}
				assertBytes(t, write(f.in), rejectedExpected(X4))
			})
		}
	}
	t.Run("empty record is a value", func(t *testing.T) {
		f := conformanceFixture()
		f.c("execution")["record"] = ""
		f.mark("", "execution")
		f.dim("execution", "XPASS", D6, nil)
		f.want["stale_markers"] = array{object{"record": "", "dimension": "execution"}}
		f.want["counts"] = object{"xfail": 0, "xpass": 1}
		assertFixture(t, f)
	})
}
func TestCheckShapeClausesForEveryKind(t *testing.T) {
	for _, kind := range []string{"plain", "prompt", "comparison"} {
		for _, cause := range []string{"both emptiness flags", "missing precondition", "noncrediting", "declared_count", "condition", "condition_a"} {
			if cause == "noncrediting" && kind == "plain" || kind == "prompt" && (cause == "declared_count" || cause == "condition" || cause == "condition_a") {
				continue
			}
			t.Run(kind+"_"+cause, func(t *testing.T) {
				f := conformanceFixture()
				c := required("shape", kind, "execution")
				f.in["required"] = append(arr(f.in["required"]), c)
				if kind == "prompt" {
					f = steeringFixture()
					c = f.c("prompt")
				} else if kind == "comparison" {
					f = compareFixture()
					c = f.c("compare")
				}
				switch cause {
				case "both emptiness flags":
					c["empty_satisfiable"], c["asserts_emptiness"], c["precondition"] = true, true, "present"
				case "missing precondition":
					c["asserts_emptiness"] = true
				case "noncrediting":
					c["crediting"] = false
				case "declared_count":
					c[cause] = number("-0")
				default:
					c[cause] = ""
				}
				assertBytes(t, write(f.in), rejectedExpected(X5))
			})
		}
	}
}
func TestPoolIndexAndCountComparisons(t *testing.T) {
	for _, arm := range []string{"", "A", "B"} {
		for _, cause := range []string{"duplicate", "gap", "wrong-tier duplicate", "wrong-tier gap", "count mismatch", "mixed counts", "maximum count mismatch"} {
			t.Run(arm+"_"+cause, func(t *testing.T) {
				f := steeringFixture()
				if arm != "" {
					f = compareFixture()
				}
				switch cause {
				case "duplicate":
					f.in["trials"] = append(arr(f.in["trials"]), trial("prompt", arm, "current", 0))
				case "gap":
					for _, v := range arr(f.in["trials"]) {
						tr := obj(v)
						if str(tr["arm"]) == arm && integer(tr["trial_index"]) == 15 {
							tr["trial_index"] = number("16")
						}
					}
				case "wrong-tier duplicate", "wrong-tier gap":
					idx := 0
					if cause == "wrong-tier gap" {
						idx = 17
					}
					v := trial("prompt", arm, "current", idx)
					v["tier"] = "scripted"
					f.in["trials"] = append(arr(f.in["trials"]), v)
				default:
					for _, v := range arr(f.in["trials"]) {
						tr := obj(v)
						if str(tr["arm"]) == arm {
							if cause != "mixed counts" || integer(tr["trial_index"]) == 0 {
								tr["declared_count"] = number("17")
								if cause == "maximum count mismatch" {
									tr["declared_count"] = number("9007199254740991")
								}
							}
						}
					}
				}
				assertBytes(t, write(f.in), rejectedExpected(X6))
			})
		}
	}
	t.Run("maximum count equal is not X6", func(t *testing.T) {
		f := steeringFixture()
		f.c("prompt")["declared_count"] = number("9007199254740991")
		for _, v := range arr(f.in["trials"]) {
			obj(v)["declared_count"] = number("9007199254740991")
		}
		o := f.out("prompt")
		decision(o, "unobserved", P1, "trials_short")
		o["to_run"] = int64(9007199254740975)
		f.gap("steering", "Steering", D4, nil)
		assertFixture(t, f)
	})
	for _, n := range []string{"-0", "0", "1", "15"} {
		for _, outside := range []bool{false, true} {
			t.Run("X7_"+n+fmt.Sprint(outside), func(t *testing.T) {
				f := steeringFixture()
				if outside {
					f = conformanceFixture()
					c := required("prompt", "prompt", "steering")
					f.in["required"] = append(arr(f.in["required"]), c)
				}
				f.in["trials"] = array{}
				f.c("prompt")["declared_count"] = number(n)
				assertBytes(t, write(f.in), rejectedExpected(X7))
			})
		}
	}
}
func TestRoutingEveryDimension(t *testing.T) {
	for _, kind := range []string{"plain", "prompt", "comparison"} {
		for _, d := range fixtureDimensions {
			bad := kind == "prompt" && d != "steering" || kind == "comparison" && d != "tool-description-efficacy" || kind == "plain" && (d == "steering" || d == "tool-description-efficacy")
			if !bad {
				continue
			}
			t.Run(kind+"_"+d, func(t *testing.T) {
				f := compareFixture()
				f.in["trials"] = array{}
				f.in["required"] = array{}
				if kind != "comparison" {
					f.in["required"] = append(arr(f.in["required"]), required("compare", "comparison", "tool-description-efficacy"))
				}
				c := required("misrouted", kind, d)
				if kind == "prompt" {
					c["declared_count"], c["condition"], c["condition_a"] = number("16"), "current", "current"
				}
				f.in["required"] = append(arr(f.in["required"]), c)
				assertBytes(t, write(f.in), rejectedExpected(X10))
			})
		}
	}
	for _, kind := range []string{"plain", "prompt", "comparison"} {
		t.Run("duplicate_"+kind, func(t *testing.T) {
			f := conformanceFixture()
			if kind != "plain" {
				f = compareFixture()
			}
			id := "execution"
			if kind == "prompt" {
				id = "prompt"
			}
			if kind == "comparison" {
				id = "compare"
			}
			f.in["required"] = append(arr(f.in["required"]), f.c(id))
			assertBytes(t, write(f.in), rejectedExpected(X10))
		})
	}
}
func TestBaselineIntegerConstraints(t *testing.T) {
	for _, outside := range []bool{false, true} {
		for _, tt := range []struct {
			name, k, n string
			reject     bool
		}{
			{"zero n", "0", "-0", true}, {"greater k", "9007199254740991", "9007199254740990", true},
			{"equal maximum", "9007199254740991", "9007199254740991", false},
		} {
			t.Run(tt.name+fmt.Sprint(outside), func(t *testing.T) {
				f := conformanceFixture()
				d := "execution"
				if outside {
					d = "steering"
				}
				e := baselineEntry("old", "prompt", d, "held")
				e["k"], e["n"] = number(tt.k), number(tt.n)
				f.in["baseline"] = object{"entries": array{e}}
				if tt.reject {
					assertBytes(t, write(f.in), rejectedExpected(X11))
					return
				}
				if outside {
					setBaseline(f, array{e}, array{entryClass("old", "set_aside", B2)}, "none", B1, array{})
				} else {
					setBaseline(f, array{e}, array{entryClass("old", "missing", E6)}, "not_measured", R2, expectedNewConformanceExcept("old"))
					f.runVerdict("PROVISIONAL", "gaps", "not_measured", 4)
				}
				assertFixture(t, f)
			})
		}
	}
	for _, delta := range []int64{-1, 0, 1} {
		t.Run(fmt.Sprintf("sum_max%+d", delta), func(t *testing.T) {
			f := conformanceFixture()
			a := baselineEntry("a", "prompt", "steering", "held")
			b := baselineEntry("b", "prompt", "steering", "held")
			a["n"] = number("9007199254740975")
			b["n"] = number(fmt.Sprint(16 + delta))
			a["k"], b["k"] = number("0"), number("0")
			setBaseline(f, array{a, b}, array{entryClass("a", "set_aside", B2), entryClass("b", "set_aside", B2)}, "none", B1, array{})
			if delta > 0 {
				assertBytes(t, write(f.in), rejectedExpected(X11))
			} else {
				assertFixture(t, f)
			}
		})
	}
	for _, kind := range []string{"plain", "comparison"} {
		t.Run("nonprompt counts ignored_"+kind, func(t *testing.T) {
			f := conformanceFixture()
			e := baselineEntry("old", kind, "steering", "held")
			e["k"], e["n"] = number("9007199254740991"), number("-0")
			setBaseline(f, array{e}, array{entryClass("old", "set_aside", B2)}, "none", B1, array{})
			assertFixture(t, f)
		})
	}
}

func TestX11AllKindsAndOutOfScopeEntries(t *testing.T) {
	for _, kind := range []string{"plain", "prompt", "comparison"} {
		t.Run("retired_"+kind, func(t *testing.T) {
			f := allScopeFixture(13)
			id := "execution"
			if kind == "prompt" {
				id = "prompt"
			}
			if kind == "comparison" {
				id = "compare"
			}
			f.in["request"] = "anchor"
			f.in["retired"] = array{id}
			assertBytes(t, write(f.in), rejectedExpected(X11))
		})
		t.Run("duplicate baseline_"+kind, func(t *testing.T) {
			f := conformanceFixture()
			e := baselineEntry("duplicate", kind, "steering", "held")
			f.in["baseline"] = object{"entries": array{e, e}}
			assertBytes(t, write(f.in), rejectedExpected(X11))
		})
	}
	for _, field := range []string{"condition", "k", "n"} {
		t.Run("out of scope missing_"+field, func(t *testing.T) {
			f := conformanceFixture()
			e := baselineEntry("old", "prompt", "steering", "held")
			e[field] = nil
			f.in["baseline"] = object{"entries": array{e}}
			assertBytes(t, write(f.in), rejectedExpected(X11))
		})
	}
}

func TestMaximumDeclaredCountStillRequiresExactTrialCount(t *testing.T) {
	for _, arm := range []string{"", "A", "B"} {
		t.Run(arm, func(t *testing.T) {
			f := steeringFixture()
			if arm != "" {
				f = compareFixture()
			}
			f.c("prompt")["declared_count"] = number("9007199254740991")
			for _, v := range arr(f.in["trials"]) {
				tr := obj(v)
				tr["declared_count"] = number("9007199254740991")
				if str(tr["arm"]) == arm {
					tr["declared_count"] = number("9007199254740990")
				}
			}
			assertBytes(t, write(f.in), rejectedExpected(X6))
		})
	}
}

func TestX7MinimumBoundary(t *testing.T) {
	for _, count := range []int{15, 16, 17} {
		for _, outside := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d_outside=%v", count, outside), func(t *testing.T) {
				f := steeringFixture()
				f.in["trials"] = array{}
				if outside {
					f = conformanceFixture()
					c := required("prompt", "prompt", "steering")
					f.in["required"] = append(arr(f.in["required"]), c)
					o := expectedCheck("prompt", "out_of_scope", "", nil)
					o["rule"] = nil
					f.want["checks"] = append(arr(f.want["checks"]), o)
					sortObjects(arr(f.want["checks"]), "id")
				}
				f.c("prompt")["declared_count"] = number(fmt.Sprint(count))
				if count < 16 {
					assertBytes(t, write(f.in), rejectedExpected(X7))
					return
				}
				if !outside {
					o := f.out("prompt")
					decision(o, "unobserved", P1, "trials_short")
					o["k"], o["n"], o["lower"], o["upper"], o["to_run"] = int64(0), int64(0), nil, nil, int64(count)
					f.gap("steering", "Steering", D4, nil)
				}
				assertFixture(t, f)
			})
		}
	}
}

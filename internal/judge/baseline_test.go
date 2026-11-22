package judge

import "testing"

func setBaseline(f fixture, entries array, classes array, result string, rule Rule, newIDs array) {
	f.in["baseline"] = object{"entries": entries}
	next := any(nil)
	if rule == R2 {
		next = "re_anchor"
	}
	f.want["comparison"] = object{"result": result, "rule": rule, "next_action": next, "entries": classes, "new": newIDs}
}
func entryClass(id, class string, rule Rule) object {
	return object{"check_id": id, "class": class, "rule": rule}
}
func expectedNewConformanceExcept(id string) array {
	a := array{}
	f := conformanceFixture()
	for _, v := range arr(f.want["checks"]) {
		s := str(obj(v)["id"])
		if s != id {
			a = append(a, s)
		}
	}
	return a
}
func TestPlainBaselineClauses(t *testing.T) {
	for _, tt := range []struct {
		name, class string
		rule        Rule
		edit        func(fixture, object)
	}{
		{"E1 absent", "missing", E1, func(f fixture, e object) { e["check_id"] = "absent" }},
		{"E1 unobserved before E2", "missing", E1, func(f fixture, e object) {
			f.in["evidence"] = without(arr(f.in["evidence"]), "check_id", "execution")
			decision(f.out("execution"), "unobserved", C1, "no_evidence")
			f.gap("execution", "Operability", D4, nil)
			e["tier"] = "mixed"
		}},
		{"E2 version", "not_compared", E2, func(f fixture, e object) { e["rulebook_version"] = "older" }},
		{"E2 tier", "not_compared", E2, func(f fixture, e object) { e["tier"] = "live" }},
		{"E2 kind", "not_compared", E2, func(f fixture, e object) { e["kind"] = "comparison" }},
		{"E2 dimension", "not_compared", E2, func(f fixture, e object) { e["dimension"] = "availability" }},
		{"E3", "regressed", E3, func(f fixture, e object) {
			f.e("execution")["outcome"] = "not_held"
			decision(f.out("execution"), "failed", C6, nil)
			f.red("execution", "Operability")
		}},
		{"E3 marker does not soften", "regressed", E3, func(f fixture, e object) {
			f.e("execution")["outcome"] = "not_held"
			f.mark("execution", "execution")
			decision(f.out("execution"), "failed_expected", C5, nil)
			f.dim("execution", "XFAIL", D5, nil)
			f.group("Operability", "KNOWN-DEFECT", G4)
			f.want["counts"] = object{"xfail": 1, "xpass": 0}
		}},
		{"E4 crediting", "improved", E4, func(f fixture, e object) { e["result"] = "failed" }},
		{"E4 noncrediting", "improved", E4, func(f fixture, e object) {
			e["result"] = "failed"
			f.c("execution")["crediting"] = false
			decision(f.out("execution"), "held_non_crediting", C8, nil)
			f.dim("execution", "UNCOVERED", D8, nil)
			f.group("Operability", "GAPPED", G3)
			f.runVerdict("PROVISIONAL", "gaps", "not_measured", 4)
		}},
		{"E5 held", "same", E5, func(f fixture, e object) {}},
		{"E5 failed", "same", E5, func(f fixture, e object) {
			e["result"] = "failed"
			f.e("execution")["outcome"] = "not_held"
			decision(f.out("execution"), "failed", C6, nil)
			f.red("execution", "Operability")
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := conformanceFixture()
			e := baselineEntry("execution", "plain", "execution", "held")
			tt.edit(f, e)
			id := str(e["check_id"])
			newIDs := expectedNewConformanceExcept(id)
			rule, result := R3, "not_regressed"
			if tt.class == "missing" || tt.class == "not_compared" {
				rule, result = R2, "not_measured"
				f.runVerdict("PROVISIONAL", "gaps", "not_measured", 4)
			}
			if tt.class == "regressed" {
				rule, result = R1, "regressed"
				f.runVerdict("FAILED", nil, "red", 1)
			}
			setBaseline(f, array{e}, array{entryClass(id, tt.class, tt.rule)}, result, rule, newIDs)
			assertFixture(t, f)
		})
	}
	t.Run("B1 empty baseline", func(t *testing.T) {
		f := conformanceFixture()
		f.in["baseline"] = object{"entries": array{}}
		assertFixture(t, f)
	})
	t.Run("B1 B2 all entries out of scope", func(t *testing.T) {
		f := conformanceFixture()
		e := baselineEntry("prompt", "prompt", "steering", "held")
		setBaseline(f, array{e}, array{entryClass("prompt", "set_aside", B2)}, "none", B1, array{})
		assertFixture(t, f)
	})
	t.Run("R1 before R2", func(t *testing.T) {
		f := conformanceFixture()
		f.e("execution")["outcome"] = "not_held"
		decision(f.out("execution"), "failed", C6, nil)
		f.red("execution", "Operability")
		setBaseline(f, array{baselineEntry("absent", "plain", "availability", "held"), baselineEntry("execution", "plain", "execution", "held")}, array{entryClass("absent", "missing", E1), entryClass("execution", "regressed", E3)}, "regressed", R1, expectedNewConformanceExcept("execution"))
		assertFixture(t, f)
	})
	t.Run("new matches out of scope baseline by id", func(t *testing.T) {
		f := conformanceFixture()
		e := baselineEntry("execution", "plain", "steering", "held")
		active := baselineEntry("availability", "plain", "availability", "held")
		ids := expectedNewConformanceExcept("execution")
		ids2 := array{}
		for _, v := range ids {
			if str(v) != "availability" {
				ids2 = append(ids2, v)
			}
		}
		setBaseline(f, array{e, active}, array{entryClass("availability", "same", E5), entryClass("execution", "set_aside", B2)}, "not_regressed", R3, ids2)
		assertFixture(t, f)
	})
}
func TestPromptBaselineClauses(t *testing.T) {
	for _, tt := range []struct {
		name, class string
		rule        Rule
		edit        func(fixture, object)
	}{
		{"E6 absent", "missing", E6, func(f fixture, e object) { e["check_id"] = "absent" }},
		{"E6 P0", "missing", E6, func(f fixture, e object) {
			f.in["trials"] = array{}
			for i := 0; i < 5; i++ {
				v := trial("prompt", "", "current", i)
				v["errored"] = true
				f.in["trials"] = append(arr(f.in["trials"]), v)
			}
			o := f.out("prompt")
			decision(o, "unobserved", P0, "error_cap")
			o["k"], o["n"] = int64(0), int64(0)
			o["lower"], o["upper"] = nil, nil
			f.gap("steering", "Steering", D4, nil)
		}},
		{"E6 P1", "missing", E6, func(f fixture, e object) {
			f.in["trials"] = array{}
			o := f.out("prompt")
			decision(o, "unobserved", P1, "trials_short")
			o["k"], o["n"] = int64(0), int64(0)
			o["lower"], o["upper"] = nil, nil
			o["to_run"] = int64(16)
			f.gap("steering", "Steering", D4, nil)
			e["tier"] = "offline"
		}},
		{"E7 version", "not_compared", E7, func(f fixture, e object) { e["rulebook_version"] = "older" }},
		{"E7 tier", "not_compared", E7, func(f fixture, e object) { e["tier"] = "offline" }},
		{"E7 dimension", "not_compared", E7, func(f fixture, e object) {
			f.in["scope"] = array{"steering", "compare"}
			f.want["scope"] = array{"steering", "compare"}
			f.c("prompt")["condition_a"] = "current"
			f.in["required"] = append(arr(f.in["required"]), required("compare", "comparison", "tool-description-efficacy"))
			for _, v := range arr(f.in["trials"]) {
				obj(v)["arm"] = "B"
			}
			replaceTrials(f, "A", 16, 16)
			o := expectedCheck("compare", "unobserved", K4, "interval_undecided")
			o["d_lo"], o["d_hi"] = "-0.193608", "0.193608"
			f.want["checks"] = append(array{o}, arr(f.want["checks"])...)
			f.gap("tool-description-efficacy", "Steering", D4, nil)
			e["dimension"] = "tool-description-efficacy"
		}},
		{"E8", "regressed", E8, func(f fixture, e object) {
			replaceTrials(f, "", 16, 0)
			setPromptExpected(f, 0, 16, "0.000000", "0.193608", P4, "failed", nil)
		}},
		{"E9", "improved", E9, func(f fixture, e object) { e["result"] = "failed" }},
		{"E9a", "same", E9a, func(f fixture, e object) {
			e["result"] = "failed"
			replaceTrials(f, "", 16, 0)
			setPromptExpected(f, 0, 16, "0.000000", "0.193608", P4, "failed", nil)
		}},
		{"E9b", "not_compared", E9b, func(f fixture, e object) {
			e["result"] = "failed"
			replaceTrials(f, "", 16, 10)
			setPromptExpected(f, 10, 16, "0.386410", "0.815188", P5, "unobserved", "interval_undecided")
		}},
		{"E10", "same", E10, func(f fixture, e object) {}},
		{"E10 P5 remains same", "same", E10, func(f fixture, e object) {
			e["k"] = number("10")
			replaceTrials(f, "", 16, 10)
			setPromptExpected(f, 10, 16, "0.386410", "0.815188", P5, "unobserved", "interval_undecided")
		}},
		{"E11 undecided", "not_compared", E11, func(f fixture, e object) { e["condition"] = "older" }},
		{"E11 improved", "improved", E11, func(f fixture, e object) { e["k"] = number("0") }},
		{"E11 regressed P5", "regressed", E11, func(f fixture, e object) {
			replaceTrials(f, "", 16, 10)
			setPromptExpected(f, 10, 16, "0.386410", "0.815188", P5, "unobserved", "interval_undecided")
		}},
		{"E11 same", "same", E11, func(f fixture, e object) {
			e["condition"] = "older"
			e["k"], e["n"] = number("1000"), number("1000")
			f.c("prompt")["declared_count"] = number("1000")
			replaceTrials(f, "", 1000, 1000)
			setPromptExpected(f, 1000, 1000, "0.996173", "1.000000", P3, "held_crediting", nil)
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := steeringFixture()
			e := baselineEntry("prompt", "prompt", "steering", "held")
			tt.edit(f, e)
			result, rule := "not_regressed", R3
			if tt.class == "missing" || tt.class == "not_compared" {
				result, rule = "not_measured", R2
				f.runVerdict("PROVISIONAL", "gaps", "not_measured", 4)
			}
			if tt.class == "regressed" {
				result, rule = "regressed", R1
				f.runVerdict("FAILED", nil, "red", 1)
			}
			newIDs := array{}
			if str(e["check_id"]) != "prompt" {
				newIDs = append(newIDs, "prompt")
			}
			if len(arr(f.in["required"])) == 2 {
				newIDs = append(array{"compare"}, newIDs...)
			}
			setBaseline(f, array{e}, array{entryClass(str(e["check_id"]), tt.class, tt.rule)}, result, rule, newIDs)
			assertFixture(t, f)
		})
	}
}
func makeAnchorExpected(f fixture, rule Rule, reason any, checks array, entries, replaced array) {
	f.in["request"] = "anchor"
	f.want["own_verdict"] = f.want["verdict"]
	f.want["verdict"] = nil
	f.want["comparison"] = nil
	f.want["outcome"] = "refused"
	f.want["exit_code"] = 5
	var baseline any
	if rule == N7 {
		f.want["outcome"] = "anchored"
		f.want["exit_code"] = 0
		baseline = object{"entries": entries}
	}
	f.want["anchor"] = object{"result": f.want["outcome"], "rule": rule, "reason": reason, "checks": checks, "baseline": baseline, "replaced": replaced}
}
func conformanceEntries(f fixture) array {
	entries := array{}
	for _, v := range arr(f.want["checks"]) {
		o := obj(v)
		if str(o["result"]) == "unobserved" || str(o["result"]) == "out_of_scope" {
			continue
		}
		id := str(o["id"])
		result := "held"
		if str(o["result"]) == "failed_expected" || str(o["result"]) == "failed" {
			result = "failed"
		}
		entries = append(entries, baselineEntry(id, "plain", str(f.c(id)["dimension"]), result))
	}
	return entries
}
func TestAnchorClauses(t *testing.T) {
	t.Run("N1 before N2", func(t *testing.T) {
		f := conformanceFixture()
		f.in["claimed_tier"] = "scripted"
		f.e("execution")["outcome"] = "not_held"
		decision(f.out("execution"), "failed", C6, nil)
		f.red("execution", "Operability")
		makeAnchorExpected(f, N1, "test_run", array{}, nil, array{})
		assertFixture(t, f)
	})
	t.Run("N2 before N3", func(t *testing.T) {
		f := conformanceFixture()
		f.e("execution")["outcome"] = "not_held"
		decision(f.out("execution"), "failed", C6, nil)
		f.in["evidence"] = without(arr(f.in["evidence"]), "check_id", "availability")
		decision(f.out("availability"), "unobserved", C1, "no_evidence")
		f.dim("availability", "HARNESS", D4, nil)
		f.red("execution", "Operability")
		makeAnchorExpected(f, N2, "red", array{}, nil, array{})
		assertFixture(t, f)
	})
	t.Run("N3 before N4", func(t *testing.T) {
		f := conformanceFixture()
		obj(f.in["subject_facts"])["bounding"] = nil
		f.gap("bounding", "Conformance", D3, "placeholder_unfilled")
		f.mark("execution", "execution")
		f.dim("execution", "XPASS", D6, nil)
		f.want["stale_markers"] = array{object{"record": "execution", "dimension": "execution"}}
		f.want["counts"] = object{"xfail": 0, "xpass": 1}
		makeAnchorExpected(f, N3, "not_measured", array{}, nil, array{})
		assertFixture(t, f)
	})
	t.Run("N4", func(t *testing.T) {
		f := conformanceFixture()
		f.mark("execution", "execution")
		f.dim("execution", "XPASS", D6, nil)
		f.want["stale_markers"] = array{object{"record": "execution", "dimension": "execution"}}
		f.want["counts"] = object{"xfail": 0, "xpass": 1}
		makeAnchorExpected(f, N4, "stale_marker", array{}, nil, array{})
		assertFixture(t, f)
	})
	t.Run("N5", func(t *testing.T) {
		f := conformanceFixture()
		f.in["baseline"] = object{"entries": array{baselineEntry("absent", "plain", "execution", "held")}}
		makeAnchorExpected(f, N5, "unretired_check", array{"absent"}, nil, array{})
		assertFixture(t, f)
	})
	for _, accepted := range []bool{false, true} {
		t.Run("N6 or N7 accepted "+map[bool]string{false: "no", true: "yes"}[accepted], func(t *testing.T) {
			f := conformanceFixture()
			old := baselineEntry("execution", "plain", "execution", "held")
			f.in["baseline"] = object{"entries": array{old}}
			f.e("execution")["outcome"] = "not_held"
			f.mark("execution", "execution")
			decision(f.out("execution"), "failed_expected", C5, nil)
			f.dim("execution", "XFAIL", D5, nil)
			f.group("Operability", "KNOWN-DEFECT", G4)
			f.runVerdict("PROVISIONAL", "known-defects", "known_defect", 3)
			f.want["counts"] = object{"xfail": 1, "xpass": 0}
			if accepted {
				f.in["accepted_regressions"] = array{"execution"}
				makeAnchorExpected(f, N7, nil, array{}, conformanceEntries(f), array{old})
			} else {
				makeAnchorExpected(f, N6, "unaccepted_regression", array{"execution"}, nil, array{})
			}
			assertFixture(t, f)
		})
	}
	t.Run("N7 retired absent", func(t *testing.T) {
		f := conformanceFixture()
		old := baselineEntry("absent", "plain", "execution", "held")
		f.in["baseline"] = object{"entries": array{old}}
		f.in["retired"] = array{"absent"}
		makeAnchorExpected(f, N7, nil, array{}, conformanceEntries(f), array{old})
		assertFixture(t, f)
	})
	t.Run("N7 prompt", func(t *testing.T) {
		f := steeringFixture()
		e := baselineEntry("prompt", "prompt", "steering", "held")
		makeAnchorExpected(f, N7, nil, array{}, array{e}, array{})
		assertFixture(t, f)
	})
	t.Run("N7 carry and replace", func(t *testing.T) {
		f := conformanceFixture()
		collision := baselineEntry("execution", "plain", "steering", "held")
		carry := baselineEntry("outside", "plain", "tool-description-efficacy", "failed")
		carry["condition"] = "preserved"
		carry["k"], carry["n"] = number("-0"), number("9007199254740991")
		old := baselineEntry("availability", "plain", "availability", "held")
		f.in["baseline"] = object{"entries": array{carry, collision, old}}
		entries := conformanceEntries(f)
		entries = append(entries, carry)
		sortObjects(entries, "check_id")
		makeAnchorExpected(f, N7, nil, array{}, entries, array{old, collision})
		assertFixture(t, f)
	})
	t.Run("N7 no successor unobserved", func(t *testing.T) {
		f := conformanceFixture()
		obj(f.in["subject_facts"])["bounding"] = "none"
		f.in["evidence"] = without(arr(f.in["evidence"]), "check_id", "bounding")
		decision(f.out("bounding"), "unobserved", C1, "no_evidence")
		f.dim("bounding", "N/A", D2, nil)
		old := baselineEntry("bounding", "plain", "bounding", "held")
		f.in["baseline"] = object{"entries": array{old}}
		makeAnchorExpected(f, N7, nil, array{}, conformanceEntries(f), array{old})
		assertFixture(t, f)
	})
}
func TestAnchorSingleCauses(t *testing.T) {
	for _, rule := range []Rule{N1, N2, N3, N7} {
		t.Run(string(rule), func(t *testing.T) {
			f := conformanceFixture()
			reason := any(nil)
			switch {
			case rule == N1:
				f.in["claimed_tier"] = "scripted"
				reason = "test_run"
			case rule == N2:
				f.e("execution")["outcome"] = "not_held"
				decision(f.out("execution"), "failed", C6, nil)
				f.red("execution", "Operability")
				reason = "red"
			case rule == N3:
				obj(f.in["subject_facts"])["bounding"] = nil
				f.gap("bounding", "Conformance", D3, "placeholder_unfilled")
				reason = "not_measured"
			}
			entries := array{}
			if rule == N7 {
				entries = conformanceEntries(f)
			}
			makeAnchorExpected(f, rule, reason, array{}, entries, array{})
			assertFixture(t, f)
		})
	}
}
func TestBaselineKindChangesAndOutOfScopeResults(t *testing.T) {
	for _, unobserved := range []bool{false, true} {
		t.Run("prompt entry now plain "+map[bool]string{false: "E7", true: "E6"}[unobserved], func(t *testing.T) {
			f := conformanceFixture()
			e := baselineEntry("execution", "prompt", "execution", "held")
			class, rule, result, comparisonRule := "not_compared", E7, "not_measured", R2
			if unobserved {
				f.in["evidence"] = without(arr(f.in["evidence"]), "check_id", "execution")
				decision(f.out("execution"), "unobserved", C1, "no_evidence")
				f.gap("execution", "Operability", D4, nil)
				class, rule = "missing", E6
			}
			f.runVerdict("PROVISIONAL", "gaps", "not_measured", 4)
			setBaseline(f, array{e}, array{entryClass("execution", class, rule)}, result, comparisonRule, expectedNewConformanceExcept("execution"))
			assertFixture(t, f)
		})
	}
	t.Run("plain entry now P5 is E1", func(t *testing.T) {
		f := steeringFixture()
		replaceTrials(f, "", 16, 10)
		setPromptExpected(f, 10, 16, "0.386410", "0.815188", P5, "unobserved", "interval_undecided")
		e := baselineEntry("prompt", "plain", "steering", "held")
		setBaseline(f, array{e}, array{entryClass("prompt", "missing", E1)}, "not_measured", R2, array{})
		assertFixture(t, f)
	})
	t.Run("current out of scope is E2", func(t *testing.T) {
		f := conformanceFixture()
		c := required("outside", "plain", "cli-mcp-parity")
		f.in["required"] = append(arr(f.in["required"]), c)
		o := expectedCheck("outside", "out_of_scope", "", nil)
		o["rule"] = nil
		f.want["checks"] = append(arr(f.want["checks"]), o)
		sortObjects(arr(f.want["checks"]), "id")
		e := baselineEntry("outside", "plain", "execution", "held")
		setBaseline(f, array{e}, array{entryClass("outside", "not_compared", E2)}, "not_measured", R2, expectedNewConformanceExcept("outside"))
		f.runVerdict("PROVISIONAL", "gaps", "not_measured", 4)
		assertFixture(t, f)
	})
}

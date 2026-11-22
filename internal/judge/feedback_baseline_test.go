package judge

import (
	"fmt"
	"testing"
)

// All eight scopes with one Held check per in-scope dimension. The comparison
// uses A=0/16 and B=16/16; its prompt is anchored from B, never A.
func allScopeFixture(mask int) fixture {
	f := conformanceFixture()
	if mask&4 != 0 {
		s := steeringFixture()
		if mask&8 != 0 {
			s = compareFixture()
			s.c("prompt")["condition_a"] = "before"
			replaceTrials(s, "A", 16, 0)
			for _, v := range arr(s.in["trials"]) {
				tr := obj(v)
				if tr["arm"] == "A" {
					tr["condition"] = "before"
				}
			}
			o := find(arr(s.want["checks"]), "id", "compare")
			decision(o, "held_crediting", K4, nil)
			o["detail"] = "improved"
			o["d_lo"], o["d_hi"] = "0.726197", "1.000000"
			s.dim("tool-description-efficacy", "GREEN", D7, nil)
			s.group("Steering", "GREEN", G5)
			s.runVerdict("PASSED", nil, "green", 0)
		}
		if mask&1 == 0 {
			f = s
		} else {
			f.in["required"] = append(arr(f.in["required"]), arr(s.in["required"])...)
			f.in["trials"] = s.in["trials"]
			f.want["checks"] = append(arr(f.want["checks"]), arr(s.want["checks"])...)
			f.dim("steering", "GREEN", D7, nil)
			if mask&8 != 0 {
				f.dim("tool-description-efficacy", "GREEN", D7, nil)
			}
			f.group("Steering", "GREEN", G5)
		}
	}
	if mask&2 != 0 {
		f.in["required"] = append(arr(f.in["required"]), required("destructive-guard", "plain", "destructive-guard"))
		f.in["evidence"] = append(arr(f.in["evidence"]), evidence("destructive-guard"))
		f.want["checks"] = append(arr(f.want["checks"]), expectedCheck("destructive-guard", "held_crediting", C7, nil))
		f.dim("destructive-guard", "GREEN", D7, nil)
	}
	scope := array{}
	for i, s := range []string{"conformance", "destructive", "steering", "compare"} {
		if mask&(1<<i) != 0 {
			scope = append(scope, s)
		}
	}
	f.in["scope"], f.want["scope"] = scope, scope
	sortObjects(arr(f.want["checks"]), "id")
	return f
}
func scopeBaseline(f fixture) array {
	entries := array{}
	for _, v := range arr(f.in["required"]) {
		c := obj(v)
		e := baselineEntry(str(c["id"]), str(c["kind"]), str(c["dimension"]), "held")
		entries = append(entries, e)
	}
	sortObjects(entries, "check_id")
	return entries
}
func TestAnchorContentEveryScope(t *testing.T) {
	for _, mask := range []int{1, 3, 4, 12, 5, 7, 13, 15} {
		t.Run(fmt.Sprint(mask), func(t *testing.T) {
			f := allScopeFixture(mask)
			entries := scopeBaseline(f)
			// Every old in-scope entry is recorded, even when its version is obsolete.
			old := array{}
			for _, v := range entries {
				e := obj(v)
				o := baselineEntry(str(e["check_id"]), str(e["kind"]), str(e["dimension"]), "held")
				o["rulebook_version"] = "old-version"
				old = append(old, o)
			}
			outside := baselineEntry("carry", "comparison", "cli-mcp-parity", "failed")
			outside["rulebook_version"] = "preserve-version"
			outside["tier"] = "replay"
			outside["condition"] = "preserve-condition"
			outside["k"], outside["n"] = number("19"), number("23")
			old = append(old, outside)
			f.in["baseline"] = object{"entries": old}
			entries = append(entries, outside)
			sortObjects(entries, "check_id")
			replaced := array{}
			for _, v := range old {
				if obj(v)["check_id"] != "carry" {
					replaced = append(replaced, v)
				}
			}
			sortObjects(replaced, "check_id")
			makeAnchorExpected(f, N7, nil, array{}, entries, replaced)
			assertFixture(t, f)
		})
	}
	t.Run("plain evidence tier is stored rather than claimed tier", func(t *testing.T) {
		f := conformanceFixture()
		f.e("execution")["tier"] = "mixed"
		entries := conformanceEntries(f)
		find(entries, "check_id", "execution")["tier"] = "mixed"
		makeAnchorExpected(f, N7, nil, array{}, entries, array{})
		assertFixture(t, f)
	})
	t.Run("scripted comparison and prompt store claimed tier", func(t *testing.T) {
		// N1 prevents anchoring scripted runs; E2/E7 still compare their stored tiers.
		f := allScopeFixture(12)
		f.in["claimed_tier"] = "scripted"
		for _, v := range arr(f.in["trials"]) {
			obj(v)["tier"] = "scripted"
		}
		entries := scopeBaseline(f)
		for _, v := range entries {
			obj(v)["tier"] = "scripted"
		}
		setBaseline(f, entries, array{entryClass("compare", "same", E5), entryClass("prompt", "same", E10)}, "not_regressed", R3, array{})
		assertFixture(t, f)
	})
}
func TestBaselineEntryScopeAndVersion(t *testing.T) {
	for _, kind := range []string{"plain", "comparison", "prompt"} {
		for _, field := range []string{"rulebook_version", "tier", "dimension", "kind"} {
			t.Run(kind+"_"+field, func(t *testing.T) {
				f := allScopeFixture(13)
				id, d := "execution", "execution"
				rule := E2
				if kind == "comparison" {
					id, d = "compare", "tool-description-efficacy"
				}
				if kind == "prompt" {
					id, d = "prompt", "steering"
					rule = E7
				}
				e := baselineEntry(id, kind, d, "held")
				switch field {
				case "rulebook_version":
					e[field] = "1.0-other"
				case "tier":
					e[field] = "replay"
				case "dimension":
					if kind == "plain" {
						e[field] = "availability"
					} else {
						e[field] = "execution"
					}
				case "kind":
					if kind == "prompt" {
						e[field] = "plain"
						rule = E2
					} else {
						e[field] = "comparison"
						if kind == "comparison" {
							e[field] = "plain"
						}
					}
				}
				newIDs := array{}
				for _, v := range arr(f.want["checks"]) {
					if obj(v)["id"] != id {
						newIDs = append(newIDs, obj(v)["id"])
					}
				}
				setBaseline(f, array{e}, array{entryClass(id, "not_compared", rule)}, "not_measured", R2, newIDs)
				f.runVerdict("PROVISIONAL", "gaps", "not_measured", 4)
				assertFixture(t, f)
			})
		}
	}
	for _, kind := range []string{"plain", "prompt", "comparison"} {
		t.Run("B2 based on entry dimension_"+kind, func(t *testing.T) {
			f := allScopeFixture(13)
			id := "execution"
			if kind == "prompt" {
				id = "prompt"
			}
			if kind == "comparison" {
				id = "compare"
			}
			e := baselineEntry(id, kind, "cli-mcp-parity", "held")
			e["rulebook_version"] = "obsolete"
			e["tier"] = "replay"
			active := baselineEntry("availability", "plain", "availability", "held")
			newIDs := array{}
			for _, v := range arr(f.want["checks"]) {
				s := str(obj(v)["id"])
				if s != id && s != "availability" {
					newIDs = append(newIDs, s)
				}
			}
			classes := array{entryClass("availability", "same", E5), entryClass(id, "set_aside", B2)}
			sortObjects(classes, "check_id")
			setBaseline(f, array{active, e}, classes, "not_regressed", R3, newIDs)
			assertFixture(t, f)
		})
	}
}
func TestE10UsesCandidatePoolAndAllThreeValues(t *testing.T) {
	for _, field := range []string{"same", "condition", "k", "n"} {
		t.Run(field, func(t *testing.T) {
			f := allScopeFixture(12)
			e := baselineEntry("prompt", "prompt", "steering", "held")
			class, rule, result, rr := "same", E10, "not_regressed", R3
			switch field {
			case "condition":
				e[field] = "before"
			case "k":
				e[field] = number("15")
			case "n":
				e[field] = number("17")
			}
			if field != "same" {
				class, rule, result, rr = "not_compared", E11, "not_measured", R2
				f.runVerdict("PROVISIONAL", "gaps", "not_measured", 4)
			}
			setBaseline(f, array{e}, array{entryClass("prompt", class, rule)}, result, rr, array{"compare"})
			assertFixture(t, f)
		})
	}
}
func TestE11RoundedEdges(t *testing.T) {
	for _, tt := range []struct {
		k      int
		lo, hi string
		same   bool
	}{
		{169, "0.949981", "0.994051", true}, {170, "0.958601", "0.996805", false}, {171, "0.967807", "0.998973", false},
	} {
		for _, swap := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d_swap=%v", tt.k, swap), func(t *testing.T) {
				f := steeringFixture()
				f.c("prompt")["declared_count"] = number("172")
				ka, kb := 161, tt.k
				lo, hi := tt.lo, tt.hi
				if swap {
					ka, kb = kb, ka
					lo, hi = "0.889124", "0.963917"
				}
				replaceTrials(f, "", 172, kb)
				setPromptExpected(f, int64(kb), 172, lo, hi, P3, "held_crediting", nil)
				e := baselineEntry("prompt", "prompt", "steering", "held")
				e["condition"] = "older"
				e["k"], e["n"] = number(fmt.Sprint(ka)), number("172")
				class, result, rule := "not_compared", "not_measured", R2
				if tt.same {
					class, result, rule = "same", "not_regressed", R3
				} else {
					f.runVerdict("PROVISIONAL", "gaps", "not_measured", 4)
				}
				setBaseline(f, array{e}, array{entryClass("prompt", class, E11)}, result, rule, array{})
				assertFixture(t, f)
			})
		}
	}
}
func TestE11OnlyChangedEntriesAreSummed(t *testing.T) {
	f := steeringFixture()
	e := baselineEntry("prompt", "prompt", "steering", "held")
	e["condition"] = "old"
	// A huge unchanged entry would overwhelm the changed set if E10 leaked in.
	c := required("unchanged", "prompt", "steering")
	c["condition"], c["declared_count"] = "current", number("1000")
	f.in["required"] = append(arr(f.in["required"]), c)
	for i := 0; i < 1000; i++ {
		v := trial("unchanged", "", "current", i)
		v["declared_count"] = number("1000")
		f.in["trials"] = append(arr(f.in["trials"]), v)
	}
	o := expectedCheck("unchanged", "held_crediting", P3, nil)
	o["k"], o["n"] = int64(1000), int64(1000)
	o["lower"], o["upper"] = "0.996173", "1.000000"
	f.want["checks"] = append(arr(f.want["checks"]), o)
	u := baselineEntry("unchanged", "prompt", "steering", "held")
	u["k"], u["n"] = number("1000"), number("1000")
	setBaseline(f, array{e, u}, array{entryClass("prompt", "not_compared", E11), entryClass("unchanged", "same", E10)}, "not_measured", R2, array{})
	f.runVerdict("PROVISIONAL", "gaps", "not_measured", 4)
	assertFixture(t, f)
}

func TestE11ImprovementAndRegressionRoundedEdges(t *testing.T) {
	for _, tt := range []struct {
		k       int
		lo, hi  string
		decided bool
	}{
		{206, "0.881319", "0.951862", false}, {207, "0.886641", "0.955357", false}, {208, "0.891998", "0.958816", true},
	} {
		for _, swap := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d_swap=%v", tt.k, swap), func(t *testing.T) {
				f := steeringFixture()
				f.c("prompt")["declared_count"] = number("223")
				ka, kb := 170, tt.k
				lo, hi := tt.lo, tt.hi
				pr, result, reason := P3, "held_crediting", any(nil)
				if swap {
					ka, kb = kb, ka
					lo, hi = "0.702320", "0.813459"
					pr, result, reason = P5, "unobserved", "interval_undecided"
				}
				replaceTrials(f, "", 223, kb)
				setPromptExpected(f, int64(kb), 223, lo, hi, pr, result, reason)
				e := baselineEntry("prompt", "prompt", "steering", "held")
				e["condition"] = "old"
				e["k"], e["n"] = number(fmt.Sprint(ka)), number("223")
				class, comparison, rr := "not_compared", "not_measured", R2
				if tt.decided {
					class, comparison, rr = "improved", "not_regressed", R3
					if swap {
						class, comparison, rr = "regressed", "regressed", R1
						f.runVerdict("FAILED", nil, "red", 1)
					}
				} else {
					f.runVerdict("PROVISIONAL", "gaps", "not_measured", 4)
				}
				setBaseline(f, array{e}, array{entryClass("prompt", class, E11)}, comparison, rr, array{})
				assertFixture(t, f)
			})
		}
	}
}
func TestE11AggregatesChangedPromptsWithUnequalCounts(t *testing.T) {
	f := steeringFixture()
	c := required("second", "prompt", "steering")
	c["declared_count"], c["condition"] = number("16"), "current"
	f.in["required"] = append(arr(f.in["required"]), c)
	for i := 0; i < 16; i++ {
		f.in["trials"] = append(arr(f.in["trials"]), trial("second", "", "current", i))
	}
	o := expectedCheck("second", "held_crediting", P3, nil)
	o["k"], o["n"] = int64(16), int64(16)
	o["lower"], o["upper"] = "0.806392", "1.000000"
	f.want["checks"] = append(arr(f.want["checks"]), o)
	a := baselineEntry("prompt", "prompt", "steering", "held")
	a["k"], a["n"] = number("16"), number("32")
	b := baselineEntry("second", "prompt", "steering", "held")
	b["k"] = number("8")
	// Totals are A=24/48, B=32/32. Both changed entries get the same class.
	setBaseline(f, array{a, b}, array{entryClass("prompt", "improved", E11), entryClass("second", "improved", E11)}, "not_regressed", R3, array{})
	assertFixture(t, f)
}
func TestAnchorNoSuccessorForNowOutOfScopeCheck(t *testing.T) {
	f := conformanceFixture()
	c := required("moved", "plain", "cli-mcp-parity")
	f.in["required"] = append(arr(f.in["required"]), c)
	o := expectedCheck("moved", "out_of_scope", "", nil)
	o["rule"] = nil
	f.want["checks"] = append(arr(f.want["checks"]), o)
	sortObjects(arr(f.want["checks"]), "id")
	e := baselineEntry("moved", "plain", "execution", "held")
	f.in["baseline"] = object{"entries": array{e}}
	makeAnchorExpected(f, N7, nil, array{}, conformanceEntriesWithoutMoved(f), array{e})
	assertFixture(t, f)
}
func conformanceEntriesWithoutMoved(f fixture) array {
	entries := array{}
	for _, v := range arr(f.in["required"]) {
		c := obj(v)
		if c["id"] != "moved" {
			entries = append(entries, baselineEntry(str(c["id"]), "plain", str(c["dimension"]), "held"))
		}
	}
	sortObjects(entries, "check_id")
	return entries
}
func TestAnchorStoresHeldNoncreditingCheck(t *testing.T) {
	f := conformanceFixture()
	c := required("extra", "plain", "execution")
	c["crediting"] = false
	f.in["required"] = append(arr(f.in["required"]), c)
	f.in["evidence"] = append(arr(f.in["evidence"]), evidence("extra"))
	f.want["checks"] = append(arr(f.want["checks"]), expectedCheck("extra", "held_non_crediting", C8, nil))
	sortObjects(arr(f.want["checks"]), "id")
	makeAnchorExpected(f, N7, nil, array{}, conformanceEntries(f), array{})
	assertFixture(t, f)
}

func TestE11PoolsInsteadOfClassifyingEachPrompt(t *testing.T) {
	f := steeringFixture()
	entries := array{}
	classes := array{}
	for _, id := range []string{"prompt", "second", "third"} {
		if id != "prompt" {
			c := required(id, "prompt", "steering")
			c["declared_count"], c["condition"] = number("16"), "current"
			f.in["required"] = append(arr(f.in["required"]), c)
			for i := 0; i < 16; i++ {
				f.in["trials"] = append(arr(f.in["trials"]), trial(id, "", "current", i))
			}
			o := expectedCheck(id, "held_crediting", P3, nil)
			o["k"], o["n"] = int64(16), int64(16)
			o["lower"], o["upper"] = "0.806392", "1.000000"
			f.want["checks"] = append(arr(f.want["checks"]), o)
		}
		e := baselineEntry(id, "prompt", "steering", "held")
		e["condition"] = "old"
		entries = append(entries, e)
		classes = append(classes, entryClass(id, "same", E11))
	}
	// Each 16/16 pair is undecided in isolation. The totals 48/48 are Same.
	setBaseline(f, entries, classes, "not_regressed", R3, array{})
	assertFixture(t, f)
}

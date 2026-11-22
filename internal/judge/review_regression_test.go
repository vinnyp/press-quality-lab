package judge

import (
	"bytes"
	"fmt"
	"testing"
)

func TestReviewTrialIndexOrder(t *testing.T) {
	t.Run("T2_excess_failure_listed_first", func(t *testing.T) {
		f := steeringFixture()
		extra := trial("prompt", "", "current", 16)
		extra["outcome"] = "not_held"
		f.in["trials"] = append(array{extra}, arr(f.in["trials"])...)
		assertFixture(t, f)
	})
	t.Run("T3_reverse_with_errors_at_both_ends", func(t *testing.T) {
		f := steeringFixture()
		f.in["trials"] = array{}
		for i := 19; i >= 0; i-- {
			tr := trial("prompt", "", "current", i)
			tr["errored"] = i < 4 || i > 17
			f.in["trials"] = append(arr(f.in["trials"]), tr)
		}
		setPromptExpected(f, 14, 14, "0.784689", "1.000000", P0, "unobserved", "error_cap")
		assertFixture(t, f)
	})
}

func TestReviewFractionalErrorCap(t *testing.T) {
	for _, row := range []struct {
		count, errors int
		rule          Rule
		lower         string
	}{
		{17, 4, P3, "0.815682"}, {17, 5, P0, "0.815682"}, {18, 5, P0, "0.824121"}, {19, 5, P0, "0.831821"},
	} {
		t.Run(fmt.Sprintf("count_%d_errors_%d", row.count, row.errors), func(t *testing.T) {
			f := steeringFixture()
			f.c("prompt")["declared_count"] = number(fmt.Sprint(row.count))
			f.in["trials"] = array{}
			for i := 0; i < row.count+row.errors; i++ {
				tr := trial("prompt", "", "current", i)
				tr["declared_count"] = number(fmt.Sprint(row.count))
				tr["errored"] = i < row.errors
				f.in["trials"] = append(arr(f.in["trials"]), tr)
			}
			result, reason := "held_crediting", any(nil)
			if row.rule == P0 {
				result, reason = "unobserved", "error_cap"
			}
			setPromptExpected(f, int64(row.count), int64(row.count), row.lower, "1.000000", row.rule, result, reason)
			assertFixture(t, f)
		})
	}
	t.Run("earlier_count_3_one_error", func(t *testing.T) {
		f := steeringFixture()
		for i := 0; i < 4; i++ {
			tr := trial("prompt", "", "old", i)
			tr["declared_count"] = number("3")
			tr["errored"] = i == 0
			f.in["trials"] = append(arr(f.in["trials"]), tr)
			f.want["set_aside"] = append(arr(f.want["set_aside"]), object{"check_id": "prompt", "arm": nil, "condition": "old", "trial_index": int64(i), "reason": "earlier_pool"})
		}
		f.want["earlier_pools"] = array{object{"check_id": "prompt", "arm": nil, "condition": "old", "k": int64(3), "n": int64(3), "rule": P0}}
		assertFixture(t, f)
	})
}

func TestReviewNullRecord(t *testing.T) {
	t.Run("X4_null_does_not_credit_empty", func(t *testing.T) {
		f := conformanceFixture()
		f.c("execution")["record"] = nil
		f.mark("", "execution")
		assertBytes(t, write(f.in), rejectedExpected(X4))
	})
	t.Run("C6_null_failure_beside_empty_credit", func(t *testing.T) {
		f := conformanceFixture()
		f.c("execution")["record"] = ""
		f.mark("", "execution")
		c := required("failure", "plain", "execution")
		c["record"] = nil
		e := evidence("failure")
		e["outcome"] = "not_held"
		f.in["required"] = append(arr(f.in["required"]), c)
		f.in["evidence"] = append(arr(f.in["evidence"]), e)
		f.want["checks"] = append(arr(f.want["checks"]), expectedCheck("failure", "failed", C6, nil))
		sortObjects(arr(f.want["checks"]), "id")
		f.red("execution", "Operability")
		assertFixture(t, f)
	})
}

func TestReviewComparisonNewScope(t *testing.T) {
	f := conformanceFixture()
	c := required("outside", "prompt", "steering")
	f.in["required"] = append(arr(f.in["required"]), c)
	o := expectedCheck("outside", "out_of_scope", "", nil)
	o["rule"] = nil
	f.want["checks"] = append(arr(f.want["checks"]), o)
	sortObjects(arr(f.want["checks"]), "id")
	e := baselineEntry("execution", "plain", "execution", "held")
	setBaseline(f, array{e}, array{entryClass("execution", "same", E5)}, "not_regressed", R3, expectedNewConformanceExcept("execution"))
	assertFixture(t, f)
}

func TestReviewX2SharedComparisonID(t *testing.T) {
	for _, kind := range []string{"plain", "prompt"} {
		for _, observation := range []string{"evidence", "trials"} {
			t.Run(kind+"_"+observation, func(t *testing.T) {
				f := conformanceFixture()
				f.in["required"] = append(arr(f.in["required"]), required("shared", kind, "execution"), required("shared", "comparison", "tool-description-efficacy"))
				if observation == "evidence" {
					f.in["evidence"] = append(arr(f.in["evidence"]), evidence("shared"))
				} else {
					f.in["trials"] = array{trial("shared", "", "current", 0)}
				}
				assertBytes(t, write(f.in), rejectedExpected(X2))
			})
		}
	}
}

func TestReviewEarlierPoolIgnoresW1(t *testing.T) {
	f := steeringFixture()
	for i := 0; i < 16; i++ {
		tr := trial("prompt", "", "old", i)
		f.in["trials"] = append(arr(f.in["trials"]), tr)
		f.want["set_aside"] = append(arr(f.want["set_aside"]), object{"check_id": "prompt", "arm": nil, "condition": "old", "trial_index": int64(i), "reason": "earlier_pool"})
	}
	tr := trial("prompt", "", "old", 0)
	tr["tier"] = "scripted"
	tr["outcome"] = "not_held"
	f.in["trials"] = append(arr(f.in["trials"]), tr)
	f.want["set_aside"] = append(array{object{"check_id": "prompt", "arm": nil, "condition": "old", "trial_index": int64(0), "reason": "wrong_tier"}}, arr(f.want["set_aside"])...)
	f.want["earlier_pools"] = array{object{"check_id": "prompt", "arm": nil, "condition": "old", "k": int64(16), "n": int64(16), "rule": P3}}
	assertFixture(t, f)
}

func TestReviewOutputEscapes(t *testing.T) {
	f := steeringFixture()
	id := "control\x1fdelete\x7f"
	f.c("prompt")["id"] = id
	for _, v := range arr(f.in["trials"]) {
		obj(v)["check_id"] = id
	}
	f.out("prompt")["id"] = id
	assertFixture(t, f)
	output, _, err := Safe(Judge, write(f.in))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte(`control\u001fdelete`)) || !bytes.Contains(output, []byte{0x7f}) {
		t.Fatalf("unexpected escapes: %s", output)
	}
}

func markedFailure(f fixture, id, d, g string) {
	f.e(id)["outcome"] = "not_held"
	f.mark(id, d)
	decision(f.out(id), "failed_expected", C5, nil)
	f.dim(d, "XFAIL", D5, nil)
	f.group(g, "KNOWN-DEFECT", G4)
}

func TestReviewAnchorOrder(t *testing.T) {
	t.Run("N5_multiple_absent", func(t *testing.T) {
		f := conformanceFixture()
		f.in["baseline"] = object{"entries": array{baselineEntry("z-last", "plain", "execution", "held"), baselineEntry("a-first", "plain", "execution", "held")}}
		makeAnchorExpected(f, N5, "unretired_check", array{"a-first", "z-last"}, nil, array{})
		assertFixture(t, f)
	})
	for _, withAbsent := range []bool{false, true} {
		t.Run(fmt.Sprintf("N6_multiple_regressions_absent_%v", withAbsent), func(t *testing.T) {
			f := conformanceFixture()
			markedFailure(f, "execution", "execution", "Operability")
			markedFailure(f, "availability", "availability", "Operability")
			f.want["counts"] = object{"xfail": 2, "xpass": 0}
			f.runVerdict("PROVISIONAL", "known-defects", "known_defect", 3)
			entries := array{baselineEntry("execution", "plain", "execution", "held"), baselineEntry("availability", "plain", "availability", "held")}
			rule, reason, checks := N6, "unaccepted_regression", array{"availability", "execution"}
			if withAbsent {
				entries = append(entries, baselineEntry("absent", "plain", "execution", "held"))
				rule, reason, checks = N5, "unretired_check", array{"absent"}
			}
			f.in["baseline"] = object{"entries": entries}
			makeAnchorExpected(f, rule, reason, checks, nil, array{})
			assertFixture(t, f)
		})
	}
	t.Run("N7_multiple_replaced", func(t *testing.T) {
		f := conformanceFixture()
		entries := array{baselineEntry("robustness", "plain", "robustness", "held"), baselineEntry("execution", "plain", "execution", "held"), baselineEntry("availability", "plain", "availability", "held")}
		f.in["baseline"] = object{"entries": entries}
		replaced := array{entries[2], entries[1], entries[0]}
		makeAnchorExpected(f, N7, nil, array{}, conformanceEntries(f), replaced)
		assertFixture(t, f)
	})
}

func TestReviewCombinedOrdering(t *testing.T) {
	f := conformanceFixture()
	// Two XPASS dimensions, each with two records supplied backwards.
	for _, d := range []string{"robustness", "determinism"} {
		f.c(d)["record"] = "z-record"
		id := d + "-extra"
		c := required(id, "plain", d)
		c["record"] = "a-record"
		f.in["required"] = append(arr(f.in["required"]), c)
		f.in["evidence"] = append(arr(f.in["evidence"]), evidence(id))
		f.want["checks"] = append(arr(f.want["checks"]), expectedCheck(id, "held_crediting", C7, nil))
		f.mark("z-record", d)
		f.mark("a-record", d)
		f.dim(d, "XPASS", D6, nil)
		for _, record := range []string{"a-record", "z-record"} {
			f.want["stale_markers"] = append(arr(f.want["stale_markers"]), object{"dimension": d, "record": record})
		}
	}
	// Markers on XFAIL dimensions are dormant even beside a held crediting check.
	for _, d := range []string{"response-correctness", "completeness-signaling"} {
		markedFailure(f, d, d, "Correctness")
		id := d + "-held"
		f.in["required"] = append(arr(f.in["required"]), required(id, "plain", d))
		f.in["evidence"] = append(arr(f.in["evidence"]), evidence(id))
		f.mark(id, d)
		f.want["checks"] = append(arr(f.want["checks"]), expectedCheck(id, "held_crediting", C7, nil))
	}
	f.in["evidence"] = without(arr(f.in["evidence"]), "check_id", "provenance-honesty")
	decision(f.out("provenance-honesty"), "unobserved", C1, "no_evidence")
	f.gap("provenance-honesty", "Correctness", D4, nil) // G3 must outrank both XFAILs.
	f.want["counts"] = object{"xfail": 2, "xpass": 2}
	for _, id := range []string{"a-prompt", "z-prompt"} {
		f.in["required"] = append(arr(f.in["required"]), required(id, "prompt", "steering"))
		o := expectedCheck(id, "out_of_scope", "", nil)
		o["rule"] = nil
		f.want["checks"] = append(arr(f.want["checks"]), o)
		for _, arm := range []string{"", "A", "B"} {
			for _, condition := range []string{"alpha", "beta"} {
				for i := 0; i < 16; i++ {
					tr := trial(id, arm, condition, i)
					f.in["trials"] = append(arr(f.in["trials"]), tr)
					var armValue any
					if arm != "" {
						armValue = arm
					}
					if i == 0 {
						wrong := trial(id, arm, condition, i)
						wrong["tier"] = "scripted"
						f.in["trials"] = append(arr(f.in["trials"]), wrong)
						f.want["set_aside"] = append(arr(f.want["set_aside"]), object{"check_id": id, "arm": armValue, "condition": condition, "trial_index": int64(i), "reason": "wrong_tier"})
					}
					reason := "other_arm"
					if arm == "" {
						reason = "earlier_pool"
					}
					f.want["set_aside"] = append(arr(f.want["set_aside"]), object{"check_id": id, "arm": armValue, "condition": condition, "trial_index": int64(i), "reason": reason})
				}
				if arm == "" {
					f.want["earlier_pools"] = append(arr(f.want["earlier_pools"]), object{"check_id": id, "arm": nil, "condition": condition, "k": int64(16), "n": int64(16), "rule": P3})
				}
			}
		}
	}
	sortObjects(arr(f.want["checks"]), "id")
	// A deterministic permutation interleaves every ordering key.
	trials := arr(f.in["trials"])
	for i := range trials {
		j := (i*37 + 11) % len(trials)
		trials[i], trials[j] = trials[j], trials[i]
	}
	assertFixture(t, f)
}

// TestReviewAnchorIndependentOrdering tests the internal anchor function directly.
func TestReviewAnchorIndependentOrdering(t *testing.T) {
	f := conformanceFixture()
	f.in["baseline"] = object{"entries": array{baselineEntry("z-entry", "plain", "execution", "held"), baselineEntry("a-entry", "plain", "execution", "held")}}
	in := obj(clone(f.in))
	decodeEnums(in)
	r := newRequest(in)
	own := object{"token": Passed, "sub_label": nil, "plain": Green}
	// Do not call comparison(): anchor must sort its own lists even when the
	// caller hands it a differently ordered baseline and comparison block.
	got := r.anchor(own, map[DimensionStatus]bool{}, array{}, object{"entries": array{}})
	want := object{"result": "refused", "rule": N5, "reason": "unretired_check", "checks": array{"a-entry", "z-entry"}, "baseline": nil, "replaced": array{}}
	if !bytes.Equal(write(got), write(want)) {
		t.Fatalf("N5 order: %s", write(got))
	}
	r.input["retired"] = array{"z-entry", "a-entry"}
	comparison := object{"entries": array{object{"check_id": "z-entry", "class": Regressed}, object{"check_id": "a-entry", "class": Regressed}}}
	got = r.anchor(own, map[DimensionStatus]bool{}, array{}, comparison)
	want["rule"], want["reason"] = N6, "unaccepted_regression"
	if !bytes.Equal(write(got), write(want)) {
		t.Fatalf("N6 order: %s", write(got))
	}
	r.input["accepted_regressions"] = array{"z-entry", "a-entry"}
	got = r.anchor(own, map[DimensionStatus]bool{}, array{}, comparison)
	want = object{"result": "anchored", "rule": N7, "reason": nil, "checks": array{}, "baseline": object{"entries": array{}}, "replaced": array{obj(r.baseline()[1]), obj(r.baseline()[0])}}
	if !bytes.Equal(write(got), write(want)) {
		t.Fatalf("replaced order: %s", write(got))
	}
}

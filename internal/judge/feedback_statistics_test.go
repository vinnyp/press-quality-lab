package judge

import (
	"bytes"
	"fmt"
	"testing"
)

// These fixtures keep the live current pools intact so a set-aside trial has
// exactly one observable effect: its listing (and W3's earlier-pool result).
func TestTrialFilteringMatrix(t *testing.T) {
	for _, compare := range []bool{false, true} {
		for _, tier := range []string{"offline", "replay", "scripted", "live"} {
			for _, arm := range []string{"", "A", "B"} {
				for _, old := range []bool{false, true} {
					t.Run(fmt.Sprintf("compare=%v_tier=%s_arm=%s_old=%v", compare, tier, arm, old), func(t *testing.T) {
						f := steeringFixture()
						if compare {
							f = compareFixture()
						}
						condition := "current"
						if old {
							condition = "old"
						}
						used := (compare && arm != "") || (!compare && arm == "")
						reason := ""
						if tier != "live" {
							reason = "wrong_tier"
						} else if !used {
							reason = "other_arm"
						} else if old {
							reason = "earlier_pool"
						}
						if reason == "" {
							assertFixture(t, f)
							return
						}
						index := 0
						if reason == "wrong_tier" && used && !old {
							index = 16
						}
						v := trial("prompt", arm, condition, index)
						v["tier"] = tier
						// Neither X6's count test nor X9 may inspect set-aside trials.
						v["declared_count"] = number("1")
						v["outcome"] = "not_observed"
						obj(v["evaluator"])["kind"] = "model"
						f.in["trials"] = append(arr(f.in["trials"]), v)
						f.want["set_aside"] = array{object{"check_id": "prompt", "arm": v["arm"], "condition": condition, "trial_index": int64(index), "reason": reason}}
						if reason == "earlier_pool" {
							f.want["earlier_pools"] = array{object{"check_id": "prompt", "arm": v["arm"], "condition": condition, "k": nil, "n": nil, "rule": P1}}
						}
						assertFixture(t, f)
					})
				}
			}
		}
	}
	t.Run("W1 compares to scripted claimed tier", func(t *testing.T) {
		f := steeringFixture()
		f.in["claimed_tier"] = "scripted"
		for _, v := range arr(f.in["trials"]) {
			obj(v)["tier"] = "scripted"
		}
		v := trial("prompt", "A", "old", 0)
		f.in["trials"] = append(arr(f.in["trials"]), v)
		f.want["set_aside"] = array{object{"check_id": "prompt", "arm": "A", "condition": "old", "trial_index": int64(0), "reason": "wrong_tier"}}
		assertFixture(t, f)
	})
	for _, arm := range []string{"", "A", "B"} {
		t.Run("W3 full pool "+arm, func(t *testing.T) {
			f := steeringFixture()
			if arm != "" {
				f = compareFixture()
			}
			aside := array{}
			for i := 0; i < 16; i++ {
				v := trial("prompt", arm, "earlier", i)
				v["outcome"] = "not_held"
				f.in["trials"] = append(arr(f.in["trials"]), v)
				aside = append(aside, object{"check_id": "prompt", "arm": v["arm"], "condition": "earlier", "trial_index": int64(i), "reason": "earlier_pool"})
			}
			f.want["set_aside"] = aside
			var a any
			if arm != "" {
				a = arm
			}
			f.want["earlier_pools"] = array{object{"check_id": "prompt", "arm": a, "condition": "earlier", "k": int64(0), "n": int64(16), "rule": P4}}
			assertFixture(t, f)
		})
	}
}

func secondPrompt(f fixture) {
	c := required("second", "prompt", "steering")
	c["condition"] = "second-current"
	c["condition_a"] = "second-before"
	c["declared_count"] = number("16")
	f.in["required"] = append(arr(f.in["required"]), c)
	for _, arm := range []string{"A", "B"} {
		for i := 0; i < 16; i++ {
			condition := "second-current"
			if arm == "A" {
				condition = "second-before"
			}
			f.in["trials"] = append(arr(f.in["trials"]), trial("second", arm, condition, i))
		}
	}
	o := expectedCheck("second", "held_crediting", P3, nil)
	o["k"], o["n"] = int64(16), int64(16)
	o["lower"], o["upper"] = "0.806392", "1.000000"
	f.want["checks"] = append(arr(f.want["checks"]), o)
	// Both arms total 32/32, rather than averaging per-prompt bounds.
	find(arr(f.want["checks"]), "id", "compare")["d_lo"] = "-0.107179"
	find(arr(f.want["checks"]), "id", "compare")["d_hi"] = "0.107179"
}
func expectIncomparable(f fixture) {
	o := find(arr(f.want["checks"]), "id", "compare")
	decision(o, "unobserved", K2, "arms_not_comparable")
	o["d_lo"], o["d_hi"] = nil, nil
}
func TestComparabilityAcrossEveryPromptAndArm(t *testing.T) {
	for _, field := range []string{"model", "framing", "sampling"} {
		for _, location := range []string{"A", "B", "both arms of second", "one trial of second"} {
			t.Run(field+"_"+location, func(t *testing.T) {
				f := compareFixture()
				secondPrompt(f)
				for _, v := range arr(f.in["trials"]) {
					tr := obj(v)
					change := str(tr["arm"]) == location
					if location == "both arms of second" {
						change = tr["check_id"] == "second"
					}
					if location == "one trial of second" {
						change = tr["check_id"] == "second" && tr["arm"] == "B" && integer(tr["trial_index"]) == 15
					}
					if change {
						if field == "sampling" {
							tr[field] = object{"different": true}
						} else {
							tr[field] = "different"
						}
					}
				}
				expectIncomparable(f)
				assertFixture(t, f)
			})
		}
	}
	for _, arm := range []string{"A", "B"} {
		t.Run("null on "+arm, func(t *testing.T) {
			f := compareFixture()
			for _, v := range arr(f.in["trials"]) {
				tr := obj(v)
				if tr["arm"] == arm {
					tr["sampling"] = nil
				}
			}
			expectIncomparable(f)
			assertFixture(t, f)
		})
	}
	t.Run("unscored trials do not constrain comparability", func(t *testing.T) {
		f := compareFixture()
		for _, v := range arr(f.in["trials"]) {
			tr := obj(v)
			tr["trial_index"] = number(fmt.Sprint(integer(tr["trial_index"]) + 1))
		}
		for _, arm := range []string{"A", "B"} {
			v := trial("prompt", arm, "current", 17)
			v["sampling"] = nil
			v["model"] = "other"
			v["framing"] = "other"
			f.in["trials"] = append(arr(f.in["trials"]), v)
			// An errored trial before the scored span also has no sampling value.
			v = trial("prompt", arm, "current", 0)
			v["errored"] = true
			v["sampling"] = nil
			f.in["trials"] = append(arr(f.in["trials"]), v)
		}
		assertFixture(t, f)
	})
}
func TestSamplingValuesThroughWholeDocuments(t *testing.T) {
	for i, tt := range []struct {
		a, b  string
		equal bool
	}{
		{`{"x":-0.5}`, `{"x":0.5}`, false},
		{`{"x":-1e2}`, `{"x":-100}`, true},
		{`{}`, `{}`, true},
		{`{"x":{"a":0,"b":[1e2,true,"x",null]}}`, `{"x":{"b":[100.0,true,"x",null],"a":-0e5}}`, true},
		{`{"x":0.1}`, `{"x":0.1000000000000000055}`, false},
		{`{"x":12345678901234567890123456789012}`, `{"x":12345678901234567890123456789013}`, false},
		{`{"x":[0,0.0,0e5,-0]}`, `{"x":[-0e9999,0e-9999,0.000,0]}`, true},
		{`{"x":[1e2,0.1]}`, `{"x":[100.0,0.10]}`, true},
		{`{"x":{"a":null}}`, `{"x":{"b":null}}`, false},
		{`{"x":[1,2]}`, `{"x":[2,1]}`, false},
		{`{"x":[1]}`, `{"x":[1,2]}`, false},
		{`{"x":1}`, `{"x":"1"}`, false},
		{`{"x":0}`, `{"x":false}`, false},
		{`{"x":0}`, `{"x":null}`, false},
		{`{"x":false}`, `{"x":null}`, false},
		{`{"x":true}`, `{"x":false}`, false},
		{`{"x":[]}`, `{"x":{}}`, false},
		{`{"x":"a"}`, `{"x":"\u0061"}`, true},
		{`{"x":"é"}`, `{"x":"e\u0301"}`, false},
		{`{"x":""}`, `{"x":" "}`, false},
		{`{"x":"𝄞"}`, `{"x":"\ud834\udd1e"}`, true},
	} {
		t.Run(fmt.Sprintf("sampling_%02d", i), func(t *testing.T) {
			f := compareFixture()
			for _, v := range arr(f.in["trials"]) {
				tr := obj(v)
				tr["sampling"] = "sampling-a"
				if tr["arm"] == "B" {
					tr["sampling"] = "sampling-b"
				}
			}
			if !tt.equal {
				expectIncomparable(f)
			}
			// Preserve the deliberately different wire key order and escaped strings.
			input := bytes.ReplaceAll(write(f.in), []byte(`"sampling-a"`), []byte(tt.a))
			input = bytes.ReplaceAll(input, []byte(`"sampling-b"`), []byte(tt.b))
			assertBytes(t, input, f.want)
		})
	}
}

// Fixed rounded bounds at either side of +0.10. Swapping arms guards -0.10.
func TestNoMaterialChangeRoundedEdges(t *testing.T) {
	for _, tt := range []struct {
		k              int
		lo, hi, pl, pu string
		same           bool
	}{
		{169, "0.003639", "0.094821", "0.949981", "0.994051", true},
		{170, "0.011544", "0.100000", "0.958601", "0.996805", false},
		{171, "0.019764", "0.105306", "0.967807", "0.998973", false},
	} {
		for _, swap := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d_swap=%v", tt.k, swap), func(t *testing.T) {
				f := compareFixture()
				f.c("prompt")["declared_count"] = number("172")
				a, b := 161, tt.k
				lo, hi := tt.lo, tt.hi
				pl, pu := tt.pl, tt.pu
				if swap {
					a, b = b, a
					lo, hi = "-"+tt.hi, "-"+tt.lo
					pl, pu = "0.889124", "0.963917"
				}
				replaceTrials(f, "A", 172, a)
				replaceTrials(f, "B", 172, b)
				setPromptExpected(f, int64(b), 172, pl, pu, P3, "held_crediting", nil)
				o := find(arr(f.want["checks"]), "id", "compare")
				o["d_lo"], o["d_hi"] = lo, hi
				if tt.same {
					decision(o, "held_crediting", K4, nil)
					o["detail"] = "no_material_change"
					f.dim("tool-description-efficacy", "GREEN", D7, nil)
					f.group("Steering", "GREEN", G5)
					f.runVerdict("PASSED", nil, "green", 0)
				}
				assertFixture(t, f)
			})
		}
	}
}

func TestRoundingDoesNotRoundTwice(t *testing.T) {
	// The binary64 value of this literal is just above the decimal halfway point.
	// Scaling by 1e6 first loses that distinction and produces an even tie.
	if got := boundText(0.0000025); got != "0.000003" {
		t.Fatalf("double rounding: got %s want 0.000003", got)
	}
}

func TestRoundedDifferenceDecisions(t *testing.T) {
	// The unrounded upper bound is 0.09999998251511516. Its reported value
	// is 0.100000, so no-material-change must not win. Reverse the arms to
	// test the lower margin with the same rounding distinction.
	for _, swap := range []bool{false, true} {
		t.Run(fmt.Sprint(swap), func(t *testing.T) {
			ka, kb := 347, 372
			pl, pu := "0.959014", "0.989295"
			lo, hi := "0.034330", "0.100000"
			if swap {
				ka, kb = kb, ka
				pl, pu = "0.880550", "0.937496"
				lo, hi = "-0.100000", "-0.034330"
			}
			f := compareFixture()
			f.c("prompt")["declared_count"] = number("380")
			replaceTrials(f, "A", 380, ka)
			replaceTrials(f, "B", 380, kb)
			setPromptExpected(f, int64(kb), 380, pl, pu, P3, "held_crediting", nil)
			o := find(arr(f.want["checks"]), "id", "compare")
			o["d_lo"], o["d_hi"] = lo, hi
			assertFixture(t, f)
		})
	}
}

func TestE11RoundsDifferenceBeforeComparing(t *testing.T) {
	for _, swap := range []bool{false, true} {
		t.Run(fmt.Sprint(swap), func(t *testing.T) {
			ka, kb := 347, 372
			pl, pu := "0.959014", "0.989295"
			if swap {
				ka, kb = kb, ka
				pl, pu = "0.880550", "0.937496"
			}
			f := steeringFixture()
			f.c("prompt")["declared_count"] = number("380")
			replaceTrials(f, "", 380, kb)
			setPromptExpected(f, int64(kb), 380, pl, pu, P3, "held_crediting", nil)
			e := baselineEntry("prompt", "prompt", "steering", "held")
			e["condition"] = "older"
			e["k"], e["n"] = number(fmt.Sprint(ka)), number("380")
			setBaseline(f, array{e}, array{entryClass("prompt", "not_compared", E11)}, "not_measured", R2, array{})
			f.runVerdict("PROVISIONAL", "gaps", "not_measured", 4)
			assertFixture(t, f)
		})
	}
}

func TestEarlierPoolWhenCurrentConditionIsNull(t *testing.T) {
	f := conformanceFixture()
	c := required("outside", "prompt", "steering")
	f.in["required"] = append(arr(f.in["required"]), c)
	o := expectedCheck("outside", "out_of_scope", "", nil)
	o["rule"] = nil
	f.want["checks"] = append(arr(f.want["checks"]), o)
	sortObjects(arr(f.want["checks"]), "id")
	v := trial("outside", "", "", 0)
	f.in["trials"] = array{v}
	f.want["set_aside"] = array{object{"check_id": "outside", "arm": nil, "condition": "", "trial_index": int64(0), "reason": "earlier_pool"}}
	f.want["earlier_pools"] = array{object{"check_id": "outside", "arm": nil, "condition": "", "k": int64(1), "n": int64(1), "rule": P1}}
	assertFixture(t, f)
}

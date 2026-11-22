package judge

import (
	"fmt"
	"testing"
)

func setPromptExpected(f fixture, k, n int64, lo, hi string, rule Rule, result string, reason any) {
	o := f.out("prompt")
	o["k"], o["n"] = k, n
	o["lower"], o["upper"] = lo, hi
	decision(o, CheckResult(result), rule, reason)
	if result == "unobserved" {
		f.gap("steering", "Steering", D4, nil)
	} else if result == "failed" {
		f.red("steering", "Steering")
	}
}
func replaceTrials(f fixture, arm string, n, k int) {
	a := array{}
	for _, v := range arr(f.in["trials"]) {
		if str(obj(v)["arm"]) != arm {
			a = append(a, v)
		}
	}
	for i := 0; i < n; i++ {
		v := trial("prompt", arm, "current", i)
		v["declared_count"] = f.c("prompt")["declared_count"]
		if i >= k {
			v["outcome"] = "not_held"
		}
		a = append(a, v)
	}
	f.in["trials"] = a
}
func TestPromptClausesAndThresholds(t *testing.T) {
	for _, tt := range []struct {
		k, n   int
		lo, hi string
		rule   Rule
		result string
		reason any
	}{
		{16, 16, "0.806392", "1.000000", P3, "held_crediting", nil},
		{9, 16, "0.331786", "0.769013", P4, "failed", nil},
		{10, 16, "0.386410", "0.815188", P5, "unobserved", "interval_undecided"},
		{2337, 2870, "0.799642", "0.828089", P5, "unobserved", "interval_undecided"},
		{2338, 2870, "0.800000", "0.828427", P5, "unobserved", "interval_undecided"},
		{2339, 2870, "0.800358", "0.828765", P3, "held_crediting", nil},
		{2253, 2870, "0.769612", "0.799661", P4, "failed", nil},
		{2254, 2870, "0.769969", "0.800000", P5, "unobserved", "interval_undecided"},
		{2255, 2870, "0.770326", "0.800339", P5, "unobserved", "interval_undecided"},
	} {
		t.Run(fmt.Sprintf("%s_%d_of_%d", tt.rule, tt.k, tt.n), func(t *testing.T) {
			f := steeringFixture()
			f.c("prompt")["declared_count"] = number(fmt.Sprint(tt.n))
			replaceTrials(f, "", tt.n, tt.k)
			setPromptExpected(f, int64(tt.k), int64(tt.n), tt.lo, tt.hi, tt.rule, tt.result, tt.reason)
			assertFixture(t, f)
		})
	}
	t.Run("P1 no default max count", func(t *testing.T) {
		f := steeringFixture()
		f.c("prompt")["declared_count"] = number("9007199254740991")
		f.in["trials"] = array{}
		o := f.out("prompt")
		decision(o, "unobserved", P1, "trials_short")
		o["k"], o["n"] = int64(0), int64(0)
		o["lower"], o["upper"] = nil, nil
		o["to_run"] = maxInteger
		f.gap("steering", "Steering", D4, nil)
		assertFixture(t, f)
	})
	for _, errors := range []int{3, 4, 5} {
		t.Run(fmt.Sprintf("P0 error cap %d", errors), func(t *testing.T) {
			f := steeringFixture()
			a := array{}
			for i := 0; i < 16+errors; i++ {
				v := trial("prompt", "", "current", i)
				v["errored"] = i < errors
				a = append(a, v)
			}
			f.in["trials"] = a
			if errors > 4 {
				setPromptExpected(f, 16, 16, "0.806392", "1.000000", P0, "unobserved", "error_cap")
			}
			assertFixture(t, f)
		})
	}
	t.Run("P0 before P1", func(t *testing.T) {
		f := steeringFixture()
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
		assertFixture(t, f)
	})
	t.Run("T2 T3 later trials ignored", func(t *testing.T) {
		f := steeringFixture()
		for i := 16; i < 30; i++ {
			v := trial("prompt", "", "current", i)
			v["outcome"] = "not_held"
			v["errored"] = i > 16
			f.in["trials"] = append(arr(f.in["trials"]), v)
		}
		assertFixture(t, f)
	})
	t.Run("T3 errors before short span", func(t *testing.T) {
		f := steeringFixture()
		a := arr(f.in["trials"])[:15]
		obj(a[0])["errored"] = true
		f.in["trials"] = a
		o := f.out("prompt")
		decision(o, "unobserved", P1, "trials_short")
		o["k"], o["n"] = int64(14), int64(14)
		o["lower"] = "0.784689"
		o["to_run"] = int64(2)
		f.gap("steering", "Steering", D4, nil)
		assertFixture(t, f)
	})
}
func TestSetAsideRules(t *testing.T) {
	for _, tt := range []struct{ name, arm, condition, tier, reason string }{{"W1 before W2 and W3", "A", "old", "scripted", "wrong_tier"}, {"W2 before W3", "A", "old", "live", "other_arm"}, {"W3", "", "old", "live", "earlier_pool"}} {
		t.Run(tt.name, func(t *testing.T) {
			f := steeringFixture()
			v := trial("prompt", tt.arm, tt.condition, 0)
			v["tier"] = tt.tier
			f.in["trials"] = append(arr(f.in["trials"]), v)
			f.want["set_aside"] = array{object{"check_id": "prompt", "arm": v["arm"], "condition": tt.condition, "trial_index": int64(0), "reason": tt.reason}}
			if tt.reason == "earlier_pool" {
				f.want["earlier_pools"] = array{object{"check_id": "prompt", "arm": nil, "condition": "old", "k": int64(1), "n": int64(1), "rule": P1}}
			}
			assertFixture(t, f)
		})
	}
	for _, tt := range []struct {
		name string
		edit func(object)
	}{{"zero count", func(v object) { v["declared_count"] = number("0") }}, {"gap", func(v object) { v["trial_index"] = number("1") }}, {"outcome", func(v object) { v["outcome"] = "not_observed" }}, {"model evaluator", func(v object) { obj(v["evaluator"])["kind"] = "model" }}} {
		t.Run("W3 malformed "+tt.name, func(t *testing.T) {
			f := steeringFixture()
			v := trial("prompt", "", "old", 0)
			tt.edit(v)
			f.in["trials"] = append(arr(f.in["trials"]), v)
			f.want["set_aside"] = array{object{"check_id": "prompt", "arm": nil, "condition": "old", "trial_index": integer(v["trial_index"]), "reason": "earlier_pool"}}
			f.want["earlier_pools"] = array{object{"check_id": "prompt", "arm": nil, "condition": "old", "k": nil, "n": nil, "rule": P1}}
			assertFixture(t, f)
		})
	}
	for _, mismatch := range []bool{false, true} {
		t.Run(fmt.Sprintf("W3 duplicate or counts %v", mismatch), func(t *testing.T) {
			f := steeringFixture()
			a, b := trial("prompt", "", "old", 0), trial("prompt", "", "old", 0)
			if mismatch {
				b["trial_index"] = number("1")
				b["declared_count"] = number("17")
			}
			f.in["trials"] = append(arr(f.in["trials"]), a, b)
			f.want["set_aside"] = array{object{"check_id": "prompt", "arm": nil, "condition": "old", "trial_index": int64(0), "reason": "earlier_pool"}, object{"check_id": "prompt", "arm": nil, "condition": "old", "trial_index": integer(b["trial_index"]), "reason": "earlier_pool"}}
			f.want["earlier_pools"] = array{object{"check_id": "prompt", "arm": nil, "condition": "old", "k": nil, "n": nil, "rule": P1}}
			assertFixture(t, f)
		})
	}
	t.Run("W1 invalid fields ignored except index", func(t *testing.T) {
		f := steeringFixture()
		v := trial("prompt", "", "current", 16)
		v["tier"] = "scripted"
		v["declared_count"] = number("0")
		v["outcome"] = "not_observed"
		obj(v["evaluator"])["kind"] = "model"
		f.in["trials"] = append(arr(f.in["trials"]), v)
		f.want["set_aside"] = array{object{"check_id": "prompt", "arm": nil, "condition": "current", "trial_index": int64(16), "reason": "wrong_tier"}}
		assertFixture(t, f)
	})
}
func compareFixture() fixture {
	f := steeringFixture()
	f.in["scope"] = array{"steering", "compare"}
	f.want["scope"] = array{"steering", "compare"}
	f.c("prompt")["condition_a"] = "current"
	f.in["required"] = append(arr(f.in["required"]), required("compare", "comparison", "tool-description-efficacy"))
	f.in["trials"] = array{}
	replaceTrials(f, "A", 16, 16)
	replaceTrials(f, "B", 16, 16)
	o := expectedCheck("compare", "unobserved", K4, "interval_undecided")
	o["d_lo"], o["d_hi"] = "-0.193608", "0.193608"
	f.want["checks"] = append(array{o}, arr(f.want["checks"])...)
	f.gap("tool-description-efficacy", "Steering", D4, nil)
	return f
}
func TestComparisonClauses(t *testing.T) {
	t.Run("K4 undecided", func(t *testing.T) { assertFixture(t, compareFixture()) })
	for _, field := range []string{"model", "framing", "sampling", "sampling_null", "decimal"} {
		t.Run("K2 "+field, func(t *testing.T) {
			f := compareFixture()
			v := obj(arr(f.in["trials"])[0])
			switch field {
			case "model", "framing":
				v[field] = "other"
			case "sampling":
				v[field] = object{"x": true}
			case "sampling_null":
				v["sampling"] = nil
			case "decimal":
				for _, v := range arr(f.in["trials"]) {
					obj(v)["sampling"] = object{"value": number("0.1")}
				}
				v["sampling"] = object{"value": number("0.1000000000000000055")}
			}
			o := find(arr(f.want["checks"]), "id", "compare")
			decision(o, "unobserved", K2, "arms_not_comparable")
			o["d_lo"], o["d_hi"] = nil, nil
			assertFixture(t, f)
		})
	}
	t.Run("K1 short before K2", func(t *testing.T) {
		f := compareFixture()
		replaceTrials(f, "A", 0, 0)
		obj(arr(f.in["trials"])[0])["sampling"] = nil
		o := find(arr(f.want["checks"]), "id", "compare")
		decision(o, "unobserved", K1, "trials_short")
		o["d_lo"], o["d_hi"] = nil, nil
		assertFixture(t, f)
	})
	t.Run("K3 cap suppresses K1", func(t *testing.T) {
		f := compareFixture()
		replaceTrials(f, "A", 0, 0)
		for i := 0; i < 5; i++ {
			v := trial("prompt", "A", "current", i)
			v["errored"] = true
			f.in["trials"] = append(arr(f.in["trials"]), v)
		}
		o := find(arr(f.want["checks"]), "id", "compare")
		decision(o, "unobserved", K3, "error_cap")
		o["d_lo"], o["d_hi"] = nil, nil
		assertFixture(t, f)
	})
	t.Run("K2 before K3", func(t *testing.T) {
		f := compareFixture()
		replaceTrials(f, "A", 0, 0)
		for i := 0; i < 5; i++ {
			v := trial("prompt", "A", "current", i)
			v["errored"] = true
			f.in["trials"] = append(arr(f.in["trials"]), v)
		}
		obj(arr(f.in["trials"])[0])["model"] = "other"
		o := find(arr(f.want["checks"]), "id", "compare")
		decision(o, "unobserved", K2, "arms_not_comparable")
		o["d_lo"], o["d_hi"] = nil, nil
		assertFixture(t, f)
	})
	t.Run("K4 regressed", func(t *testing.T) {
		f := compareFixture()
		replaceTrials(f, "B", 16, 0)
		setPromptExpected(f, 0, 16, "0.000000", "0.193608", P4, "failed", nil)
		o := find(arr(f.want["checks"]), "id", "compare")
		decision(o, "failed", K4, nil)
		o["detail"] = "regressed"
		o["d_lo"], o["d_hi"] = "-1.000000", "-0.726197"
		f.red("tool-description-efficacy", "Steering")
		assertFixture(t, f)
	})
	t.Run("K4 improved", func(t *testing.T) {
		f := compareFixture()
		replaceTrials(f, "A", 16, 0)
		o := find(arr(f.want["checks"]), "id", "compare")
		decision(o, "held_crediting", K4, nil)
		o["detail"] = "improved"
		o["d_lo"], o["d_hi"] = "0.726197", "1.000000"
		f.dim("tool-description-efficacy", "GREEN", D7, nil)
		f.group("Steering", "GREEN", G5)
		f.runVerdict("PASSED", nil, "green", 0)
		assertFixture(t, f)
	})
	t.Run("K4 no material change", func(t *testing.T) {
		f := compareFixture()
		f.c("prompt")["declared_count"] = number("1000")
		replaceTrials(f, "A", 1000, 1000)
		replaceTrials(f, "B", 1000, 1000)
		setPromptExpected(f, 1000, 1000, "0.996173", "1.000000", P3, "held_crediting", nil)
		o := find(arr(f.want["checks"]), "id", "compare")
		decision(o, "held_crediting", K4, nil)
		o["detail"] = "no_material_change"
		o["d_lo"], o["d_hi"] = "-0.003827", "0.003827"
		f.dim("tool-description-efficacy", "GREEN", D7, nil)
		f.group("Steering", "GREEN", G5)
		f.runVerdict("PASSED", nil, "green", 0)
		assertFixture(t, f)
	})
}
func TestExactSamplingEquality(t *testing.T) {
	for _, tt := range []struct {
		a, b  string
		equal bool
	}{{`{"a":0,"b":[1e2,true,"x",null]}`, `{"b":[100.0,true,"x",null],"a":-0e5}`, true}, {`{"x":-0.5}`, `{"x":0.5}`, false}, {`{"x":-1e2}`, `{"x":-100}`, true}, {`{"x":0.1}`, `{"x":0.10}`, true}, {`{"x":1e9999}`, `{"x":10e9998}`, true}, {`{"x":0.1}`, `{"x":0.1000000000000000055}`, false}, {`{"x":0}`, `{"x":"0"}`, false}, {`{"x":[1,2]}`, `{"x":[2,1]}`, false}, {`{"x":true}`, `{"x":false}`, false}, {`{"x":null}`, `{"y":null}`, false}} {
		a, oka := readDocument([]byte(tt.a), nil)
		b, okb := readDocument([]byte(tt.b), nil)
		if !oka || !okb || equalJSON(a, b) != tt.equal {
			t.Fatalf("%s / %s", tt.a, tt.b)
		}
	}
}
func TestBoundsAndRounding(t *testing.T) {
	l, u := bounds(16, 16)
	if boundText(l) != "0.806392" || boundText(u) != "1.000000" {
		t.Fatalf("Wilson: %s %s", boundText(l), boundText(u))
	}
	for _, tt := range []struct {
		x    float64
		want string
	}{{0.0078125, "0.007812"}, {0.0234375, "0.023438"}, {-0.00000001, "0.000000"}, {-0.0078125, "-0.007812"}} {
		if s := boundText(tt.x); s != tt.want {
			t.Fatalf("%.15f: %s want %s", tt.x, s, tt.want)
		}
	}
}
func TestComparisonMarginThresholds(t *testing.T) {
	for _, tt := range []struct {
		k              int
		lo, hi, pl, pu string
		decided        bool
	}{{206, "0.094984", "0.227698", "0.881319", "0.951862", false}, {207, "0.100000", "0.231769", "0.886641", "0.955357", false}, {208, "0.105031", "0.235838", "0.891998", "0.958816", true}} {
		t.Run(fmt.Sprint(tt.k), func(t *testing.T) {
			f := compareFixture()
			f.c("prompt")["declared_count"] = number("223")
			replaceTrials(f, "A", 223, 170)
			replaceTrials(f, "B", 223, tt.k)
			setPromptExpected(f, int64(tt.k), 223, tt.pl, tt.pu, P3, "held_crediting", nil)
			o := find(arr(f.want["checks"]), "id", "compare")
			o["d_lo"], o["d_hi"] = tt.lo, tt.hi
			if tt.decided {
				decision(o, "held_crediting", K4, nil)
				o["detail"] = "improved"
				f.dim("tool-description-efficacy", "GREEN", D7, nil)
				f.group("Steering", "GREEN", G5)
				f.runVerdict("PASSED", nil, "green", 0)
			}
			assertFixture(t, f)
			// Swapping arms must reflect the unrounded interval and use the strict negative margin.
			lo, hi, detail := difference(int64(tt.k), 223, 170, 223)
			if boundText(lo) != "-"+tt.hi || boundText(hi) != "-"+tt.lo || tt.decided != (detail == "regressed") {
				t.Fatalf("negative margin: %s %s %s", boundText(lo), boundText(hi), detail)
			}
		})
	}
}
func TestK1SingleCauses(t *testing.T) {
	t.Run("short arm", func(t *testing.T) {
		f := compareFixture()
		replaceTrials(f, "A", 0, 0)
		o := find(arr(f.want["checks"]), "id", "compare")
		decision(o, "unobserved", K1, "trials_short")
		o["d_lo"], o["d_hi"] = nil, nil
		assertFixture(t, f)
	})
	t.Run("no prompts", func(t *testing.T) {
		f := compareFixture()
		f.in["required"] = without(arr(f.in["required"]), "id", "prompt")
		f.in["trials"] = array{}
		f.want["checks"] = without(arr(f.want["checks"]), "id", "prompt")
		o := find(arr(f.want["checks"]), "id", "compare")
		decision(o, "unobserved", K1, "trials_short")
		o["d_lo"], o["d_hi"] = nil, nil
		f.dim("steering", "UNCOVERED", D8, nil)
		assertFixture(t, f)
	})
}

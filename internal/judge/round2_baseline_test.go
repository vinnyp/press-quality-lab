package judge

import (
	"fmt"
	"testing"
)

func TestE2ExactVersionString(t *testing.T) {
	for _, kind := range []string{"plain", "comparison"} {
		for _, version := range []string{"1.0", "1.0-draft", "1.0.0", "1.00", "01.0", "1", "2.0", "", " 1.0", "1.0 ", "1.0\n", "1.0-DRAFT", "１.０"} {
			t.Run(kind+"_"+fmt.Sprintf("%q", version), func(t *testing.T) {
				f := conformanceFixture()
				id, d := "execution", "execution"
				if kind == "comparison" {
					f = allScopeFixture(12)
					id, d = "compare", "tool-description-efficacy"
				}
				e := baselineEntry(id, kind, d, "held")
				e["rulebook_version"] = version
				class, er, result, rr := "not_compared", E2, "not_measured", R2
				if version == "1.0" {
					class, er, result, rr = "same", E5, "not_regressed", R3
				} else {
					f.runVerdict("PROVISIONAL", "gaps", "not_measured", 4)
				}
				newIDs := expectedNewConformanceExcept(id)
				if kind == "comparison" {
					newIDs = array{"prompt"}
				}
				setBaseline(f, array{e}, array{entryClass(id, class, er)}, result, rr, newIDs)
				assertFixture(t, f)
			})
		}
	}
}

// All cases have two changed entries. Fixed literals below come from §9.6's
// Wilson formulas, independently evaluated for each split pool. The K4 boundary
// applies to integer totals, not each entry's interval or averaged bounds.
func TestE11SummedBoundaryOutcomes(t *testing.T) {
	for _, tt := range []struct {
		name           string
		n, a, b        int
		al, au, bl, bu [2]string
		aUndecided     bool
		class          string
	}{
		{"outer_below", 223, 170, 206, [2]string{"0.678908", "0.671979"}, [2]string{"0.834843", "0.828705"}, [2]string{"0.864197", "0.854304"}, [2]string{"0.963030", "0.957149"}, true, "not_compared"},
		// Total d_lo=0.099999906124294824 rounds to 0.100000.
		{"outer_equal_from_below", 223, 170, 207, [2]string{"0.678908", "0.671979"}, [2]string{"0.834843", "0.828705"}, [2]string{"0.864197", "0.865355"}, [2]string{"0.963030", "0.963364"}, true, "not_compared"},
		{"outer_above", 223, 170, 208, [2]string{"0.678908", "0.671979"}, [2]string{"0.834843", "0.828705"}, [2]string{"0.875523", "0.865355"}, [2]string{"0.969119", "0.963364"}, true, "improved"},
		// Total d_lo=0.10000007603366377 must still be undecided after rounding.
		{"outer_equal_from_above", 472, 384, 450, [2]string{"0.758993", "0.758993"}, [2]string{"0.858081", "0.858081"}, [2]string{"0.918479", "0.918479"}, [2]string{"0.973777", "0.973777"}, true, "not_compared"},
		{"inner_below", 172, 161, 169, [2]string{"0.856039", "0.871022"}, [2]string{"0.967635", "0.974913"}, [2]string{"0.919120", "0.937036"}, [2]string{"0.993599", "0.997944"}, false, "same"},
		// Total d_hi=0.10000025559024484 rounds down to 0.100000.
		{"inner_equal_from_above", 172, 161, 170, [2]string{"0.856039", "0.871022"}, [2]string{"0.967635", "0.974913"}, [2]string{"0.937036", "0.937036"}, [2]string{"0.997944", "0.997944"}, false, "not_compared"},
		{"inner_above", 172, 161, 171, [2]string{"0.856039", "0.871022"}, [2]string{"0.967635", "0.974913"}, [2]string{"0.937036", "0.957242"}, [2]string{"0.997944", "1.000000"}, false, "not_compared"},
		// Total d_hi=0.09999998251511516 rounds up to 0.100000.
		{"inner_equal_from_below", 380, 347, 372, [2]string{"0.861395", "0.867597"}, [2]string{"0.943387", "0.947502"}, [2]string{"0.947128", "0.947128"}, [2]string{"0.991783", "0.991783"}, false, "not_compared"},
	} {
		for _, swap := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s_swap=%v", tt.name, swap), func(t *testing.T) {
				f := steeringFixture()
				f.in["required"], f.in["trials"], f.want["checks"] = array{}, array{}, array{}
				ka, kb := tt.a, tt.b
				lows, highs := tt.bl, tt.bu
				undecided := false
				class := tt.class
				if swap {
					ka, kb = kb, ka
					lows, highs = tt.al, tt.au
					undecided = tt.aUndecided
					if class == "improved" {
						class = "regressed"
					}
				}
				entries, classes := array{}, array{}
				ns := [2]int{tt.n / 2, tt.n - tt.n/2}
				aks := [2]int{ka / 2, ka - ka/2}
				bks := [2]int{kb / 2, kb - kb/2}
				for i, id := range []string{"first", "second"} {
					n, k := ns[i], bks[i]
					c := required(id, "prompt", "steering")
					c["condition"], c["declared_count"] = "current", number(fmt.Sprint(n))
					f.in["required"] = append(arr(f.in["required"]), c)
					for j := 0; j < n; j++ {
						tr := trial(id, "", "current", j)
						tr["declared_count"] = number(fmt.Sprint(n))
						if j >= k {
							tr["outcome"] = "not_held"
						}
						f.in["trials"] = append(arr(f.in["trials"]), tr)
					}
					o := expectedCheck(id, "held_crediting", P3, nil)
					if undecided {
						decision(o, "unobserved", P5, "interval_undecided")
					}
					o["k"], o["n"], o["lower"], o["upper"] = int64(k), int64(n), lows[i], highs[i]
					f.want["checks"] = append(arr(f.want["checks"]), o)
					e := baselineEntry(id, "prompt", "steering", "held")
					e["condition"], e["k"], e["n"] = "old", number(fmt.Sprint(aks[i])), number(fmt.Sprint(n))
					entries = append(entries, e)
					classes = append(classes, entryClass(id, class, E11))
				}
				if undecided {
					f.gap("steering", "Steering", D4, nil)
				}
				result, rr := "not_regressed", R3
				if class == "not_compared" {
					result, rr = "not_measured", R2
					f.runVerdict("PROVISIONAL", "gaps", "not_measured", 4)
				}
				if class == "regressed" {
					result, rr = "regressed", R1
					f.runVerdict("FAILED", nil, "red", 1)
				}
				setBaseline(f, entries, classes, result, rr, array{})
				assertFixture(t, f)
			})
		}
	}
}

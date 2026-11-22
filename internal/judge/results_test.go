package judge

import "testing"

func without(a array, key, id string) array {
	b := array{}
	for _, v := range a {
		if str(obj(v)[key]) != id {
			b = append(b, v)
		}
	}
	return b
}
func TestPlainClauses(t *testing.T) {
	for _, tt := range []struct {
		name   string
		rule   Rule
		result string
		reason any
		edit   func(fixture)
	}{
		{"C1", C1, "unobserved", "no_evidence", func(f fixture) { f.in["evidence"] = without(arr(f.in["evidence"]), "check_id", "execution") }},
		{"C3 zero", C3, "unobserved", "empty_target", func(f fixture) {
			f.c("execution")["empty_satisfiable"] = true
			f.e("execution")["target_size"] = number("0")
		}},
		{"C3 absent", C3, "unobserved", "empty_target", func(f fixture) { f.c("execution")["empty_satisfiable"] = true; f.e("execution")["target_size"] = nil }},
		{"C4 false", C4, "unobserved", "precondition_unmet", func(f fixture) {
			f.c("execution")["asserts_emptiness"] = true
			f.c("execution")["precondition"] = ""
			f.e("execution")["precondition_observed"] = false
		}},
		{"C4 absent", C4, "unobserved", "precondition_unmet", func(f fixture) {
			f.c("execution")["asserts_emptiness"] = true
			f.c("execution")["precondition"] = "sample"
			f.e("execution")["precondition_observed"] = nil
		}},
		{"C6", C6, "failed", nil, func(f fixture) { f.e("execution")["outcome"] = "not_held" }},
		{"C7", C7, "held_crediting", nil, func(f fixture) {}},
		{"C8", C8, "held_non_crediting", nil, func(f fixture) { f.c("execution")["crediting"] = false }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := conformanceFixture()
			tt.edit(f)
			decision(f.out("execution"), CheckResult(tt.result), tt.rule, tt.reason)
			if tt.result == "unobserved" {
				f.gap("execution", "Operability", D4, nil)
			} else if tt.result == "failed" {
				f.red("execution", "Operability")
			} else if tt.rule == C8 {
				f.dim("execution", "UNCOVERED", D8, nil)
				f.group("Operability", "GAPPED", G3)
				f.runVerdict("PROVISIONAL", "gaps", "not_measured", 4)
			}
			assertFixture(t, f)
		})
	}
	for _, reason := range []string{"harness_fault", "could_not_start", "input_missing", "precondition_unmet", "empty_target", "placeholder_unfilled", "computation_failed", "earlier_step_failed", "network_attempted", "network_unverifiable", "live_not_run", "judgement_unverified"} {
		t.Run("C2 "+reason, func(t *testing.T) {
			f := conformanceFixture()
			f.e("execution")["outcome"] = "not_observed"
			f.e("execution")["reason"] = reason
			decision(f.out("execution"), "unobserved", C2, reason)
			f.gap("execution", "Operability", D4, nil)
			assertFixture(t, f)
		})
	}
	t.Run("C5 D5 G4 V3", func(t *testing.T) {
		f := conformanceFixture()
		f.e("execution")["outcome"] = "not_held"
		f.mark("execution", "execution")
		decision(f.out("execution"), "failed_expected", C5, nil)
		f.dim("execution", "XFAIL", D5, nil)
		f.group("Operability", "KNOWN-DEFECT", G4)
		f.runVerdict("PROVISIONAL", "known-defects", "known_defect", 3)
		f.want["counts"] = object{"xfail": 1, "xpass": 0}
		assertFixture(t, f)
	})
	t.Run("D6 stale deduplicated", func(t *testing.T) {
		f := conformanceFixture()
		f.mark("execution", "execution")
		f.mark("execution", "execution")
		f.dim("execution", "XPASS", D6, nil)
		f.want["counts"] = object{"xfail": 0, "xpass": 1}
		f.want["stale_markers"] = array{object{"record": "execution", "dimension": "execution"}}
		assertFixture(t, f)
	})
	t.Run("D8 no check", func(t *testing.T) {
		f := conformanceFixture()
		f.in["required"] = without(arr(f.in["required"]), "id", "execution")
		f.in["evidence"] = without(arr(f.in["evidence"]), "check_id", "execution")
		f.want["checks"] = without(arr(f.want["checks"]), "id", "execution")
		f.dim("execution", "UNCOVERED", D8, nil)
		f.group("Operability", "GAPPED", G3)
		f.runVerdict("PROVISIONAL", "gaps", "not_measured", 4)
		assertFixture(t, f)
	})
}
func TestStructuralFacts(t *testing.T) {
	for _, tt := range []struct {
		d, f, g  string
		excluded any
	}{{"bounding", "bounding", "Conformance", "none"}, {"idempotency", "read_only", "Safety", true}, {"side-effect-integrity", "store_backed", "Safety", false}, {"flag-conformance", "has_cli", "Conformance", false}, {"destructive-guard", "has_destructive_ops", "Safety", false}} {
		setup := func() fixture {
			f := conformanceFixture()
			if tt.d == "destructive-guard" {
				f.in["scope"] = array{"conformance", "destructive"}
				f.want["scope"] = array{"conformance", "destructive"}
				f.in["required"] = append(arr(f.in["required"]), required(tt.d, "plain", tt.d))
				f.in["evidence"] = append(arr(f.in["evidence"]), evidence(tt.d))
				f.want["checks"] = append(arr(f.want["checks"]), expectedCheck(tt.d, "held_crediting", C7, nil))
				sortObjects(arr(f.want["checks"]), "id")
			}
			return f
		}
		t.Run("D2 "+tt.d, func(t *testing.T) {
			f := setup()
			obj(f.in["subject_facts"])[tt.f] = tt.excluded
			f.dim(tt.d, "N/A", D2, nil)
			assertFixture(t, f)
		})
		t.Run("D3 "+tt.d, func(t *testing.T) {
			f := setup()
			obj(f.in["subject_facts"])[tt.f] = nil
			f.gap(tt.d, tt.g, D3, "placeholder_unfilled")
			assertFixture(t, f)
		})
		t.Run("D1 before D2 "+tt.d, func(t *testing.T) {
			f := setup()
			obj(f.in["subject_facts"])[tt.f] = tt.excluded
			f.e(tt.d)["outcome"] = "not_held"
			decision(f.out(tt.d), "failed", C6, nil)
			f.red(tt.d, tt.g)
			assertFixture(t, f)
		})
		t.Run("D2 before D4 "+tt.d, func(t *testing.T) {
			f := setup()
			obj(f.in["subject_facts"])[tt.f] = tt.excluded
			f.in["evidence"] = without(arr(f.in["evidence"]), "check_id", tt.d)
			decision(f.out(tt.d), "unobserved", C1, "no_evidence")
			f.dim(tt.d, "N/A", D2, nil)
			assertFixture(t, f)
		})
	}
}
func TestResultOrderAndMarkers(t *testing.T) {
	t.Run("C2 before C3 and C4", func(t *testing.T) {
		for _, empty := range []bool{true, false} {
			f := conformanceFixture()
			f.c("execution")["empty_satisfiable"] = empty
			f.c("execution")["asserts_emptiness"] = !empty
			f.c("execution")["precondition"] = "sample"
			f.e("execution")["outcome"] = "not_observed"
			f.e("execution")["reason"] = "harness_fault"
			f.e("execution")["target_size"] = number("0")
			f.e("execution")["precondition_observed"] = false
			decision(f.out("execution"), "unobserved", C2, "harness_fault")
			f.gap("execution", "Operability", D4, nil)
			assertFixture(t, f)
		}
	})
	t.Run("failed empty target is C6", func(t *testing.T) {
		f := conformanceFixture()
		f.c("execution")["empty_satisfiable"] = true
		f.e("execution")["outcome"] = "not_held"
		f.e("execution")["target_size"] = number("0")
		decision(f.out("execution"), "failed", C6, nil)
		f.red("execution", "Operability")
		assertFixture(t, f)
	})
	t.Run("D3 before D4", func(t *testing.T) {
		f := conformanceFixture()
		obj(f.in["subject_facts"])["bounding"] = nil
		f.in["evidence"] = without(arr(f.in["evidence"]), "check_id", "bounding")
		decision(f.out("bounding"), "unobserved", C1, "no_evidence")
		f.gap("bounding", "Conformance", D3, "placeholder_unfilled")
		assertFixture(t, f)
	})
	for _, mode := range []string{"dormant", "harness", "red", "excluded"} {
		t.Run("marker "+mode, func(t *testing.T) {
			f := conformanceFixture()
			d, g := "execution", "Operability"
			if mode == "excluded" {
				d, g = "bounding", "Conformance"
				obj(f.in["subject_facts"])["bounding"] = "none"
			}
			f.mark(d, d)
			f.e(d)["outcome"] = "not_held"
			decision(f.out(d), "failed_expected", C5, nil)
			f.dim(d, "XFAIL", D5, nil)
			f.group(g, "KNOWN-DEFECT", G4)
			f.runVerdict("PROVISIONAL", "known-defects", "known_defect", 3)
			f.want["counts"] = object{"xfail": 1, "xpass": 0}
			c := required("additional", "plain", d)
			f.in["required"] = append(arr(f.in["required"]), c)
			e := evidence("additional")
			out := expectedCheck("additional", "held_crediting", C7, nil)
			if mode == "harness" || mode == "excluded" {
				e["outcome"] = "not_observed"
				e["reason"] = "harness_fault"
				decision(out, "unobserved", C2, "harness_fault")
				f.gap(d, g, D4, nil)
				f.want["counts"] = object{"xfail": 0, "xpass": 0}
			} else if mode == "red" {
				e["outcome"] = "not_held"
				decision(out, "failed", C6, nil)
				f.red(d, g)
				f.want["counts"] = object{"xfail": 0, "xpass": 0}
			} else {
				f.mark("additional", d)
			}
			f.in["evidence"] = append(arr(f.in["evidence"]), e)
			f.want["checks"] = append(arr(f.want["checks"]), out)
			sortObjects(arr(f.want["checks"]), "id")
			assertFixture(t, f)
		})
	}
}
func TestUnreachableGroupAndVerdictClauses(t *testing.T) {
	s, r := groupState(map[DimensionStatus]bool{"N/A": true})
	if s != "N/A" || r != G6 {
		t.Fatalf("G6: %s %s", s, r)
	}
	v, code := verdict(map[DimensionStatus]bool{"N/A": true}, "none")
	want := object{"token": "PROVISIONAL", "sub_label": "gaps", "plain": "not_measured"}
	if string(write(v)) != string(write(want)) || code != 4 {
		t.Fatalf("V2 middle: %s %d", write(v), code)
	}
	for _, tt := range []struct {
		states            map[DimensionStatus]bool
		comparison, token string
		code              int
	}{{map[DimensionStatus]bool{"RED": true, "HARNESS": true, "XFAIL": true}, "not_measured", "FAILED", 1}, {map[DimensionStatus]bool{"GREEN": true, "HARNESS": true, "XFAIL": true}, "none", "PROVISIONAL", 4}, {map[DimensionStatus]bool{"GREEN": true, "XFAIL": true}, "none", "PROVISIONAL", 3}} {
		v, code := verdict(tt.states, ComparisonResult(tt.comparison))
		if str(v["token"]) != tt.token || code != tt.code {
			t.Fatalf("verdict precedence: %s %d", write(v), code)
		}
	}
}

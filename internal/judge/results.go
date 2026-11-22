package judge

import "sort"

func checkOutput(id string) object {
	return object{"id": id, "result": OutOfScope, "rule": nil, "reason": nil, "detail": nil, "k": nil, "n": nil, "to_run": nil, "lower": nil, "upper": nil, "d_lo": nil, "d_hi": nil}
}
func decision(o object, result CheckResult, rule Rule, reason any) {
	o["result"] = result
	o["rule"] = rule
	o["reason"] = reason
}
func (r *request) plain(c object) object {
	o := checkOutput(str(c["id"]))
	e := r.evidence[str(c["id"])]
	if e == nil {
		decision(o, Unobserved, C1, NoEvidence)
		return o
	}
	switch enum[EvidenceOutcome](e["outcome"]) {
	case NotObserved:
		decision(o, Unobserved, C2, enum[Reason](e["reason"]))
	case ObservedNotHeld:
		if c["record"] != nil && r.markers[markerKey{str(c["record"]), enum[Dimension](c["dimension"])}] {
			decision(o, FailedExpected, C5, nil)
		} else {
			decision(o, Failed, C6, nil)
		}
	case ObservedHeld:
		if boolean(c["empty_satisfiable"]) && (e["target_size"] == nil || integer(e["target_size"]) == 0) {
			decision(o, Unobserved, C3, EmptyTarget)
		} else if boolean(c["asserts_emptiness"]) && !boolean(e["precondition_observed"]) {
			decision(o, Unobserved, C4, PreconditionUnmet)
		} else if boolean(c["crediting"]) {
			decision(o, HeldCrediting, C7, nil)
		} else {
			decision(o, HeldNonCrediting, C8, nil)
		}
	}
	return o
}
func dimensionState(inScope, excluded, unstated, marked bool, results map[CheckResult]bool) (DimensionStatus, Rule, any) {
	if !inScope {
		return DimensionOutOfScope, D0, ScopeNotRequested
	}
	if results[Failed] {
		return DimensionRed, D1, nil
	}
	if excluded && !results[FailedExpected] {
		return DimensionNA, D2, nil
	}
	if unstated {
		return DimensionHarness, D3, PlaceholderUnfilled
	}
	if results[Unobserved] {
		return DimensionHarness, D4, nil
	}
	if results[FailedExpected] {
		return DimensionXFail, D5, nil
	}
	if results[HeldCrediting] && marked {
		return DimensionXPass, D6, nil
	}
	if results[HeldCrediting] {
		return DimensionGreen, D7, nil
	}
	return DimensionUncovered, D8, nil
}
func groupState(states map[DimensionStatus]bool) (GroupStatus, Rule) {
	if len(states) == 0 {
		return GroupOutOfScope, G1
	}
	red, gap, known, green := false, false, false, false
	for state, present := range states {
		if !present {
			continue
		}
		switch state {
		case DimensionRed:
			red = true
		case DimensionHarness, DimensionUncovered:
			gap = true
		case DimensionXFail:
			known = true
		case DimensionGreen, DimensionXPass:
			green = true
		case DimensionOutOfScope, DimensionNA:
		}
	}
	if red {
		return GroupRed, G2
	}
	if gap {
		return GroupGapped, G3
	}
	if known {
		return GroupKnownDefect, G4
	}
	if green {
		return GroupGreen, G5
	}
	return GroupNA, G6
}
func verdict(states map[DimensionStatus]bool, comparison ComparisonResult) (object, int) {
	regressed, notMeasured := false, false
	switch comparison {
	case ComparisonRegressed:
		regressed = true
	case ComparisonNotMeasured:
		notMeasured = true
	case NoComparison, NotRegressed:
	}
	if states[DimensionRed] || regressed {

		return object{"token": RunFailed, "sub_label": nil, "plain": Red}, 1
	}
	if states[DimensionHarness] || states[DimensionUncovered] || !states[DimensionGreen] && !states[DimensionXPass] && !states[DimensionXFail] || notMeasured {
		return object{"token": Provisional, "sub_label": Gaps, "plain": NotMeasured}, 4
	}
	if states[DimensionXFail] {
		return object{"token": Provisional, "sub_label": KnownDefects, "plain": KnownDefect}, 3
	}
	return object{"token": Passed, "sub_label": nil, "plain": Green}, 0
}
func (r *request) dimensions(checks array) (array, array, array, object, map[DimensionStatus]bool) {
	results := map[Dimension]map[CheckResult]bool{}
	marked := map[Dimension]bool{}
	for m := range r.markers {
		marked[m.dimension] = true
	}
	for _, v := range checks {
		o := obj(v)
		d := enum[Dimension](r.byID[str(o["id"])]["dimension"])
		if results[d] == nil {
			results[d] = map[CheckResult]bool{}
		}
		results[d][enum[CheckResult](o["result"])] = true
	}
	facts := obj(r.input["subject_facts"])
	dims := array{}
	groups := array{}
	stale := array{}
	counts := object{"xfail": 0, "xpass": 0}
	states := map[DimensionStatus]bool{}
	groupStates := map[Group]map[DimensionStatus]bool{}
	for _, i := range []Group{Operability, Correctness, Conformance, Usability, Safety, SteeringGroup} {
		groupStates[i] = map[DimensionStatus]bool{}
	}
	for _, d := range dimensions {
		fact := ""
		var excluded bool
		switch d {
		case Bounding:
			fact = "bounding"
			switch enum[BoundingFact](facts[fact]) {
			case NoBounding:
				excluded = true
			case DeclaredBounding, "":
			}

		case Idempotency:
			fact = "read_only"
			excluded = boolean(facts[fact])
		case DestructiveGuard:
			fact = "has_destructive_ops"
			excluded = facts[fact] != nil && !boolean(facts[fact])
		case SideEffectIntegrity:
			fact = "store_backed"
			excluded = facts[fact] != nil && !boolean(facts[fact])
		case FlagConformance:
			fact = "has_cli"
			excluded = facts[fact] != nil && !boolean(facts[fact])
		case Availability, Execution, GracefulFailure, Robustness, ResponseCorrectness, Determinism, CLIMCPParity, ProvenanceHonesty, CompletenessSignaling, OutputConformance, Discoverability, ErrorMessageQuality, TokenEconomy, PIISecretHygiene, Steering, ToolDescriptionEfficacy:
		}
		state, rule, reason := dimensionState(r.inScope[d], excluded, fact != "" && facts[fact] == nil, marked[d], results[d])
		if rule == D0 && dimensionGroup(d) == SteeringGroup {
			reason = LiveNotRun
		}
		dims = append(dims, object{"dimension": d, "state": state, "rule": rule, "reason": reason})
		if state != DimensionOutOfScope {
			states[state] = true
			groupStates[dimensionGroup(d)][state] = true
		}
		if state == DimensionXFail {
			n, ok := counts["xfail"].(int)
			if !ok {
				panic("invalid count")
			}
			counts["xfail"] = n + 1
		}
		if state == DimensionXPass {
			n, ok := counts["xpass"].(int)
			if !ok {
				panic("invalid count")
			}
			counts["xpass"] = n + 1
			records := []string{}
			for m := range r.markers {
				if m.dimension == d {
					records = append(records, m.record)
				}
			}
			sort.Strings(records)
			for _, record := range records {
				stale = append(stale, object{"record": record, "dimension": d})
			}
		}
	}
	for _, g := range []Group{Operability, Correctness, Conformance, Usability, Safety, SteeringGroup} {
		s, rule := groupState(groupStates[g])
		groups = append(groups, object{"group": g, "state": s, "rule": rule})
	}
	return dims, groups, stale, counts, states
}
func (r *request) evaluate() (object, int) {
	o := emptyOutput()
	scope := array{}
	for _, s := range []Scope{ConformanceScope, DestructiveScope, SteeringScope, CompareScope} {
		if r.scopes[s] {
			scope = append(scope, s)
		}
	}
	o["scope"] = scope
	scores := r.scores()
	checks := array{}
	sort.Slice(r.checks, func(i, j int) bool { return str(r.checks[i]["id"]) < str(r.checks[j]["id"]) })
	for _, c := range r.checks {
		v := checkOutput(str(c["id"]))
		if r.inScope[enum[Dimension](c["dimension"])] {
			switch enum[Kind](c["kind"]) {
			case Plain:
				v = r.plain(c)
			case Prompt:
				arm := NoArm
				if r.scopes[CompareScope] {
					arm = ArmB
				}
				v = promptOutput(str(c["id"]), scores[poolKey{str(c["id"]), arm, str(c["condition"])}], integer(c["declared_count"]))
			case Comparison:
				v = r.comparisonCheck(c, scores)
			}

		}
		checks = append(checks, v)
	}
	o["checks"] = checks
	o["set_aside"], o["earlier_pools"] = r.setAside()
	dims, groups, stale, counts, states := r.dimensions(checks)
	o["dimensions"] = dims
	o["groups"] = groups
	o["stale_markers"] = stale
	o["counts"] = counts
	comparison := r.comparison(checks)
	code := 0
	switch enum[Request](r.input["request"]) {
	case Grade:
		v, c := verdict(states, enum[ComparisonResult](comparison["result"]))
		code = c
		o["outcome"] = VerdictOutcome
		o["verdict"] = v
		o["comparison"] = comparison
	case Anchor:
		own, _ := verdict(states, NoComparison)
		o["own_verdict"] = own
		a := r.anchor(own, states, checks, comparison)
		o["anchor"] = a
		o["outcome"] = a["result"]
		switch enum[Outcome](a["result"]) {
		case Refused:
			code = 5
		case Anchored:
		case VerdictOutcome, Rejected:
			panic("invalid anchor outcome")
		}

	}
	o["exit_code"] = code
	return o, code
}

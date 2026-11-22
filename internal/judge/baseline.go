package judge

import "sort"

func failed(result CheckResult) bool {
	switch result {
	case Failed, FailedExpected:
		return true
	case Unobserved, HeldCrediting, HeldNonCrediting, OutOfScope:
		return false
	}
	panic("unknown check result")
}
func held(result CheckResult) bool {
	switch result {
	case HeldCrediting, HeldNonCrediting:
		return true
	case Unobserved, Failed, FailedExpected, OutOfScope:
		return false
	}
	panic("unknown check result")
}
func (r *request) storedTier(c object) any {
	if !r.inScope[enum[Dimension](c["dimension"])] {
		return nil
	}
	if enum[Kind](c["kind"]) == Plain {
		e := r.evidence[str(c["id"])]
		if e == nil {
			return nil
		}
		return e["tier"]
	}
	return r.input["claimed_tier"]
}
func (r *request) classify(e, c, o object) (EntryClass, Rule) {
	prompt := false
	switch enum[Kind](e["kind"]) {
	case Prompt:
		prompt = true
	case Plain, Comparison:
	}

	result := CheckResult("")
	if o != nil {
		result = enum[CheckResult](o["result"])
	}
	if !prompt && (c == nil || result == Unobserved) {
		return Missing, E1
	}
	if prompt && (c == nil || enum[Kind](c["kind"]) == Prompt && (o["rule"] == P0 || o["rule"] == P1) || enum[Kind](c["kind"]) != Prompt && result == Unobserved) {
		return Missing, E6
	}
	if str(e["rulebook_version"]) != "1.0" || enum[Tier](e["tier"]) != enum[Tier](r.storedTier(c)) || enum[Kind](e["kind"]) != enum[Kind](c["kind"]) || enum[Dimension](e["dimension"]) != enum[Dimension](c["dimension"]) {
		if prompt {
			return NotCompared, E7
		}
		return NotCompared, E2
	}
	baselineResult := enum[BaselineResult](e["result"])
	if !prompt {
		switch baselineResult {
		case BaselineHeld:
			if failed(result) {
				return Regressed, E3
			}
		case BaselineFailed:
			if held(result) {
				return Improved, E4
			}
		}
		return Same, E5
	}
	switch baselineResult {
	case BaselineHeld:
		if o["rule"] == P4 {
			return Regressed, E8
		}
	case BaselineFailed:
		if o["rule"] == P3 {
			return Improved, E9
		}
		if o["rule"] == P4 {
			return Same, E9a
		}
		return NotCompared, E9b
	}
	if e["condition"] == c["condition"] && integer(e["k"]) == outputInt(o["k"]) && integer(e["n"]) == outputInt(o["n"]) {
		return Same, E10
	}
	return NotCompared, E11
}
func outputInt(v any) int64 {
	n, ok := v.(int64)
	if !ok {
		panic("invalid output integer")
	}
	return n
}
func (r *request) comparison(checks array) object {
	outputs := map[string]object{}
	for _, v := range checks {
		o := obj(v)
		outputs[str(o["id"])] = o
	}
	entries := append(array{}, r.baseline()...)
	sort.Slice(entries, func(i, j int) bool { return str(obj(entries[i])["check_id"]) < str(obj(entries[j])["check_id"]) })
	active := false
	for _, v := range entries {
		if r.inScope[enum[Dimension](obj(v)["dimension"])] {
			active = true
		}
	}
	comparison := object{"result": NoComparison, "rule": B1, "next_action": nil, "entries": array{}, "new": array{}}
	classes := array{}
	changed := []object{}
	var ka, na, kb, nb int64
	ids := map[string]bool{}
	for _, v := range entries {
		e := obj(v)
		id := str(e["check_id"])
		ids[id] = true
		o := object{"check_id": id, "class": SetAside, "rule": B2}
		if active && r.inScope[enum[Dimension](e["dimension"])] {
			class, rule := r.classify(e, r.byID[id], outputs[id])
			o["class"] = class
			o["rule"] = rule
			if rule == E11 {
				changed = append(changed, o)
				ka += integer(e["k"])
				na += integer(e["n"])
				kb += outputInt(outputs[id]["k"])
				nb += outputInt(outputs[id]["n"])
			}
		}
		classes = append(classes, o)
	}
	comparison["entries"] = classes
	if !active {
		return comparison
	}
	if len(changed) > 0 {
		_, _, detail := difference(ka, na, kb, nb)
		class := NotCompared
		switch detail {
		case DetailRegressed:
			class = Regressed
		case DetailImproved:
			class = Improved
		case NoMaterialChange:
			class = Same
		case Undecided:
		}
		for _, o := range changed {
			o["class"] = class
		}
	}
	newIDs := array{}
	for _, c := range r.checks {
		if r.inScope[enum[Dimension](c["dimension"])] && !ids[str(c["id"])] {
			newIDs = append(newIDs, c["id"])
		}
	}
	comparison["new"] = newIDs
	regressed, missing := false, false
	for _, v := range classes {
		class := enum[EntryClass](obj(v)["class"])
		switch class {
		case Regressed:
			regressed = true
		case Missing, NotCompared:
			missing = true
		case SetAside, Improved, Same:
		}
	}
	if regressed {
		comparison["result"] = ComparisonRegressed
		comparison["rule"] = R1
	} else if missing {
		comparison["result"] = ComparisonNotMeasured
		comparison["rule"] = R2
		comparison["next_action"] = ReAnchor
	} else {
		comparison["result"] = NotRegressed
		comparison["rule"] = R3
	}
	return comparison
}
func (r *request) anchor(own object, states map[DimensionStatus]bool, checks array, comparison object) object {
	a := object{"result": Refused, "rule": N1, "reason": TestRun, "checks": array{}, "baseline": nil, "replaced": array{}}
	if enum[Tier](r.input["claimed_tier"]) == Scripted {
		return a
	}
	switch enum[VerdictToken](own["token"]) {
	case RunFailed:
		a["rule"] = N2
		a["reason"] = RedReason
		return a
	case Passed, Provisional:
	}
	switch enum[SubLabel](own["sub_label"]) {
	case Gaps:
		a["rule"] = N3
		a["reason"] = NotMeasuredReason
		return a
	case KnownDefects, "":
	}
	if states[DimensionXPass] {
		a["rule"] = N4
		a["reason"] = StaleMarker
		return a
	}
	retired, accepted := map[string]bool{}, map[string]bool{}
	for _, v := range arr(r.input["retired"]) {
		retired[str(v)] = true
	}
	for _, v := range arr(r.input["accepted_regressions"]) {
		accepted[str(v)] = true
	}
	absent := array{}
	for _, v := range r.baseline() {
		e := obj(v)
		id := str(e["check_id"])
		if r.inScope[enum[Dimension](e["dimension"])] && r.byID[id] == nil && !retired[id] {
			absent = append(absent, id)
		}
	}
	if len(absent) > 0 {
		a["rule"] = N5
		a["reason"] = UnretiredCheck
		sort.Slice(absent, func(i, j int) bool { return str(absent[i]) < str(absent[j]) })
		a["checks"] = absent
		return a
	}
	regressions := array{}
	for _, v := range arr(comparison["entries"]) {
		e := obj(v)
		if enum[EntryClass](e["class"]) == Regressed && !accepted[str(e["check_id"])] {
			regressions = append(regressions, e["check_id"])
		}
	}
	if len(regressions) > 0 {
		a["rule"] = N6
		a["reason"] = UnacceptedRegression
		sort.Slice(regressions, func(i, j int) bool { return str(regressions[i]) < str(regressions[j]) })
		a["checks"] = regressions
		return a
	}
	entries := array{}
	newIDs := map[string]bool{}
	for _, v := range checks {
		o := obj(v)
		result := enum[CheckResult](o["result"])
		if !held(result) && !failed(result) {
			continue
		}
		c := r.byID[str(o["id"])]
		stored := BaselineHeld
		if failed(result) {
			stored = BaselineFailed
		}
		e := object{"check_id": c["id"], "kind": c["kind"], "dimension": c["dimension"], "result": stored, "rulebook_version": "1.0", "tier": r.storedTier(c), "condition": nil, "k": nil, "n": nil}
		if enum[Kind](c["kind"]) == Prompt {
			e["condition"] = c["condition"]
			e["k"] = o["k"]
			e["n"] = o["n"]
		}
		entries = append(entries, e)
		newIDs[str(c["id"])] = true
	}
	replaced := array{}
	for _, v := range r.baseline() {
		e := obj(v)
		if r.inScope[enum[Dimension](e["dimension"])] || newIDs[str(e["check_id"])] {
			replaced = append(replaced, e)
		} else {
			entries = append(entries, e)
		}
	}
	sort.Slice(entries, func(i, j int) bool { return str(obj(entries[i])["check_id"]) < str(obj(entries[j])["check_id"]) })
	a["result"] = Anchored
	a["rule"] = N7
	a["reason"] = nil
	a["baseline"] = object{"entries": entries}
	sort.Slice(replaced, func(i, j int) bool { return str(obj(replaced[i])["check_id"]) < str(obj(replaced[j])["check_id"]) })
	a["replaced"] = replaced
	return a
}

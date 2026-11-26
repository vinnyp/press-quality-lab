package judge

import (
	"sort"

	"github.com/vinnyp/press-quality-lab/internal/rulebook"
)

type poolKey struct {
	id        string
	arm       Arm
	condition string
}
type pool struct {
	trials         []object
	indices        map[int64]bool
	indexCount     int
	maxIndex       int64
	duplicate      bool
	badOutcome     bool
	counts         map[int64]bool
	scored         []object
	scorePositions []int
	errors         int64
}
type request struct {
	input    object
	checks   []object
	byID     map[string]object
	evidence map[string]object
	scopes   map[Scope]bool
	inScope  map[Dimension]bool
	markers  map[markerKey]bool
	pools    map[poolKey]*pool
}
type markerKey struct {
	record    string
	dimension Dimension
}

func key(t object) poolKey {
	return poolKey{str(t["check_id"]), enum[Arm](t["arm"]), str(t["condition"])}
}
func (r *request) current(c object, arm Arm) *pool {
	field := "condition"
	if arm == ArmA {
		field = "condition_a"
	}
	if c[field] == nil {
		return nil
	}
	return r.pools[poolKey{str(c["id"]), arm, str(c[field])}]
}
func (r *request) arms() []Arm {
	if r.scopes[CompareScope] {
		return []Arm{ArmA, ArmB}
	}
	return []Arm{NoArm}
}
func (r *request) usedArm(arm Arm) bool {
	switch arm {
	case NoArm:
		return !r.scopes[CompareScope]
	case ArmA, ArmB:
		return r.scopes[CompareScope]
	}
	panic("unknown arm")
}

func newRequest(in object) *request {
	r := &request{input: in, byID: map[string]object{}, evidence: map[string]object{}, scopes: map[Scope]bool{}, inScope: map[Dimension]bool{}, markers: map[markerKey]bool{}, pools: map[poolKey]*pool{}}
	for _, s := range arr(in["scope"]) {
		r.scopes[enum[Scope](s)] = true
	}
	for _, d := range dimensions {
		for scope := range r.scopes {
			// Unknown input scopes are diagnosed by X1 after request construction.
			switch scope {
			case ConformanceScope, DestructiveScope, SteeringScope, CompareScope:
				r.inScope[d] = r.inScope[d] || ScopeIncludes(scope, d)
			default:
			}
		}
	}
	for _, v := range arr(in["required"]) {
		c := obj(v)
		r.checks = append(r.checks, c)
		r.byID[str(c["id"])] = c
	}
	for _, v := range arr(in["evidence"]) {
		e := obj(v)
		r.evidence[str(e["check_id"])] = e
	}
	for _, v := range arr(in["markers"]) {
		m := obj(v)
		r.markers[markerKey{str(m["record"]), enum[Dimension](m["dimension"])}] = true
	}
	return r
}
func (r *request) earlyRejection() Rule {
	if len(r.scopes) != len(arr(r.input["scope"])) || len(r.scopes) == 0 || r.scopes[DestructiveScope] && !r.scopes[ConformanceScope] || r.scopes[CompareScope] && !r.scopes[SteeringScope] {
		return X1
	}
	for s := range r.scopes {
		switch s {
		case ConformanceScope, DestructiveScope, SteeringScope, CompareScope:
		default:
			return X1
		}
	}

	kinds := map[string]map[Kind]bool{}
	for _, c := range r.checks {
		id := str(c["id"])
		if kinds[id] == nil {
			kinds[id] = map[Kind]bool{}
		}
		kinds[id][enum[Kind](c["kind"])] = true
	}
	for _, v := range arr(r.input["evidence"]) {
		k := kinds[str(obj(v)["check_id"])]
		if !k[Plain] || k[Prompt] || k[Comparison] {
			return X2
		}
	}
	for _, v := range arr(r.input["trials"]) {
		k := kinds[str(obj(v)["check_id"])]
		if !k[Prompt] || k[Plain] || k[Comparison] {
			return X2
		}
	}
	seen := map[string]bool{}
	for _, v := range arr(r.input["evidence"]) {
		e := obj(v)
		id := str(e["check_id"])
		if seen[id] || (enum[EvidenceOutcome](e["outcome"]) == NotObserved) != (e["reason"] != nil) {
			return X3
		}
		seen[id] = true
	}
	credit := map[markerKey]bool{}
	for _, c := range r.checks {
		if boolean(c["crediting"]) && c["record"] != nil {
			credit[markerKey{str(c["record"]), enum[Dimension](c["dimension"])}] = true
		}
	}
	for m := range r.markers {
		if !credit[m] || !markerGroup(dimensionGroup(m.dimension)) || m.dimension == CLIMCPParity {
			return X4
		}
	}
	for _, c := range r.checks {
		kind := enum[Kind](c["kind"])
		if boolean(c["empty_satisfiable"]) && boolean(c["asserts_emptiness"]) || boolean(c["asserts_emptiness"]) && c["precondition"] == nil || kind != Plain && !boolean(c["crediting"]) || kind != Prompt && (c["declared_count"] != nil || c["condition"] != nil || c["condition_a"] != nil) {
			return X5
		}
	}
	return ""
}
func (r *request) collect() {
	for _, v := range arr(r.input["trials"]) {
		t := obj(v)
		k := key(t)
		wrong := wrongTier(enum[Tier](t["tier"]), enum[Tier](r.input["claimed_tier"]))
		if !r.usedArm(k.arm) {
			continue
		}
		p := r.pools[k]
		if p == nil {
			p = &pool{indices: map[int64]bool{}, maxIndex: -1, counts: map[int64]bool{}}
			r.pools[k] = p
		}
		idx := integer(t["trial_index"])
		if p.indices[idx] {
			p.duplicate = true
		}
		p.indices[idx] = true
		p.indexCount++
		p.maxIndex = max(p.maxIndex, idx)
		if wrong {
			continue
		}
		p.trials = append(p.trials, t)
		p.counts[integer(t["declared_count"])] = true
		if enum[EvidenceOutcome](t["outcome"]) == NotObserved || modelEvaluator(enum[EvaluatorKind](obj(t["evaluator"])["kind"])) {
			p.badOutcome = true
		}
	}
	for _, p := range r.pools {
		sort.Slice(p.trials, func(i, j int) bool { return integer(p.trials[i]["trial_index"]) < integer(p.trials[j]["trial_index"]) })
		for i, t := range p.trials {
			if !boolean(t["errored"]) {
				p.scored = append(p.scored, t)
				p.scorePositions = append(p.scorePositions, i)
			} else {
				p.errors++
			}
		}
	}
}
func (r *request) lateRejection() Rule {
	arms := r.arms()
	for _, c := range r.checks {
		if enum[Kind](c["kind"]) != Prompt {
			continue
		}
		for _, a := range arms {
			p := r.current(c, a)
			if p == nil {
				continue
			}
			if p.duplicate || p.maxIndex != int64(p.indexCount)-1 {
				return X6
			}
			if c["declared_count"] != nil {
				n := integer(c["declared_count"])
				if len(p.counts) > 1 || len(p.counts) == 1 && !p.counts[n] {
					return X6
				}
			}
		}
	}
	for _, c := range r.checks {
		if enum[Kind](c["kind"]) == Prompt && c["declared_count"] != nil && integer(c["declared_count"]) < 16 {
			return X7
		}
	}
	for _, c := range r.checks {
		if enum[Kind](c["kind"]) == Prompt && r.inScope[enum[Dimension](c["dimension"])] && (c["declared_count"] == nil || c["condition"] == nil || r.scopes[CompareScope] && c["condition_a"] == nil) {
			return X8
		}
	}
	for _, c := range r.checks {
		if enum[Kind](c["kind"]) == Prompt {
			for _, a := range arms {
				if p := r.current(c, a); p != nil && p.badOutcome {
					return X9
				}
			}
		}
	}
	if len(r.byID) != len(r.checks) {
		return X10
	}
	comparisons := 0
	for _, c := range r.checks {
		kind, d := enum[Kind](c["kind"]), enum[Dimension](c["dimension"])
		if kind == Comparison {
			comparisons++
		}
		if kind == Prompt && d != Steering || kind == Comparison && d != ToolDescriptionEfficacy || kind == Plain && dimensionGroup(d) == SteeringGroup {
			return X10
		}
	}
	if r.scopes[CompareScope] && comparisons != 1 || !r.scopes[CompareScope] && comparisons != 0 {
		return X10
	}
	for _, v := range arr(r.input["retired"]) {
		if r.byID[str(v)] != nil {
			return X11
		}
	}
	if enum[Request](r.input["request"]) == Grade && (len(arr(r.input["retired"])) > 0 || len(arr(r.input["accepted_regressions"])) > 0) {
		return X11
	}
	ids := map[string]bool{}
	var total int64
	for _, v := range r.baseline() {
		e := obj(v)
		id := str(e["check_id"])
		if ids[id] {
			return X11
		}
		ids[id] = true
		if enum[Kind](e["kind"]) == Prompt {
			if e["condition"] == nil || e["k"] == nil || e["n"] == nil {
				return X11
			}
			k, n := integer(e["k"]), integer(e["n"])
			if n == 0 || k > n || n > maxInteger-total {
				return X11
			}
			total += n
		}
	}
	return ""
}
func (r *request) baseline() array {
	if r.input["baseline"] == nil {
		return array{}
	}
	return arr(obj(r.input["baseline"])["entries"])
}
func dimensionGroup(d Dimension) Group {
	switch d {
	case Availability, Execution, GracefulFailure, Robustness:
		return Operability
	case ResponseCorrectness, Determinism, CLIMCPParity, ProvenanceHonesty, CompletenessSignaling:
		return Correctness
	case OutputConformance, FlagConformance, Bounding:
		return Conformance
	case Discoverability, ErrorMessageQuality, TokenEconomy:
		return Usability
	case SideEffectIntegrity, Idempotency, DestructiveGuard, PIISecretHygiene:
		return Safety
	case Steering, ToolDescriptionEfficacy:
		return SteeringGroup
	}
	panic("unknown dimension")
}

func markerGroup(g Group) bool {
	switch g {
	case Operability, Correctness, Conformance, Usability:
		return true
	case Safety, SteeringGroup:
		return false
	}
	panic("unknown group")
}

func wrongTier(actual, claimed Tier) bool {
	switch actual {
	case Offline, Replay, Mixed:
		return true
	case Live:
		return claimed != Live
	case Scripted:
		return claimed != Scripted
	}
	panic("unknown tier")
}
func modelEvaluator(kind EvaluatorKind) bool {
	switch kind {
	case ModelEvaluator:
		return true
	case DeterministicEvaluator:
		return false
	}
	panic("unknown evaluator")
}

// ScopeIncludes reports whether a scope part includes a dimension (rulebook §3).
func ScopeIncludes(scope Scope, d Dimension) bool {
	return rulebook.ScopeIncludes(scope, d)
}

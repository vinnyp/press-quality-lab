package judge

import (
	"math"
	"sort"
	"strconv"

	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func bounds(k, n int64) (float64, float64) {
	z := float64(1.959964)
	nf := float64(n)
	p := float64(k) / nf
	zz := float64(z * z)
	centre := (p + zz/(float64(2)*nf)) / (1 + zz/nf)
	half := z * math.Sqrt(float64(p*(1-p))/nf+zz/(float64(4)*nf*nf)) / (1 + zz/nf)
	return centre - half, centre + half
}
func boundText(x float64) string {
	s := strconv.FormatFloat(x, 'f', 6, 64)
	if s == "-0.000000" {
		return "0.000000"
	}
	return s
}
func rounded(x float64) float64 {
	v, e := strconv.ParseFloat(boundText(x), 64)
	if e != nil {
		panic(e)
	}
	return v
}
func difference(ka, na, kb, nb int64) (float64, float64, ComparisonDetail) {
	la, ua := bounds(ka, na)
	lb, ub := bounds(kb, nb)
	pa, pb := float64(ka)/float64(na), float64(kb)/float64(nb)
	d := pb - pa
	x, y := pb-lb, ua-pa
	lo := d - math.Sqrt(float64(x*x)+float64(y*y))
	x, y = ub-pb, pa-la
	hi := d + math.Sqrt(float64(x*x)+float64(y*y))
	l, h := rounded(lo), rounded(hi)
	if h < -0.10 {
		return lo, hi, DetailRegressed
	}
	if l > 0.10 {
		return lo, hi, DetailImproved
	}
	if l > -0.10 && h < 0.10 {
		return lo, hi, NoMaterialChange
	}
	return lo, hi, Undecided
}

type score struct {
	k, n, errors int64
	rule         Rule
	scored       []object
}

func scorePool(p *pool, count int64) score {
	s := score{rule: P1}
	if p != nil {
		s.n = min(count, int64(len(p.scored)))
		s.scored = p.scored[:int(s.n)]
		for _, t := range s.scored {
			if enum[EvidenceOutcome](t["outcome"]) == ObservedHeld {
				s.k++
			}
		}
		s.errors = p.errors
		if s.n == count && count > 0 {
			s.errors = int64(p.scorePositions[int(s.n)-1]+1) - s.n
		}
	}
	if s.errors > count/4 {
		s.rule = P0
	} else if s.n < count {
		s.rule = P1
	} else if s.n > 0 {
		lo, hi := bounds(s.k, s.n)
		if rounded(lo) > 0.80 {
			s.rule = P3
		} else if rounded(hi) < 0.80 {
			s.rule = P4
		} else {
			s.rule = P5
		}
	}
	return s
}
func promptOutput(id string, s score, count int64) object {
	o := checkOutput(id)
	o["k"] = s.k
	o["n"] = s.n
	if s.n > 0 {
		lo, hi := bounds(s.k, s.n)
		o["lower"] = boundText(lo)
		o["upper"] = boundText(hi)
	}
	switch s.rule {
	case P0:
		decision(o, Unobserved, P0, ErrorCap)
	case P1:
		decision(o, Unobserved, P1, TrialsShort)
		o["to_run"] = count - s.n
	case P3:
		decision(o, HeldCrediting, P3, nil)
	case P4:
		decision(o, Failed, P4, nil)
	case P5:
		decision(o, Unobserved, P5, IntervalUndecided)
	case X1, X2, X3, X4, X5, X6, X7, X8, X9, X10, X11, X12, C1, C2, C3, C4, C5, C6, C7, C8, D0, D1, D2, D3, D4, D5, D6, D7, D8, G1, G2, G3, G4, G5, G6, K1, K2, K3, K4, B1, B2, E1, E2, E3, E4, E5, E6, E7, E8, E9, E9a, E9b, E10, E11, R1, R2, R3, N1, N2, N3, N4, N5, N6, N7:
		panic("non-prompt rule in pool score")
	}

	return o
}
func (r *request) scores() map[poolKey]score {
	scores := map[poolKey]score{}
	for _, c := range r.checks {
		if enum[Kind](c["kind"]) != Prompt || !r.inScope[enum[Dimension](c["dimension"])] {
			continue
		}
		for _, a := range r.arms() {
			field := "condition"
			if a == ArmA {
				field = "condition_a"
			}
			k := poolKey{str(c["id"]), a, str(c[field])}
			scores[k] = scorePool(r.current(c, a), integer(c["declared_count"]))
		}
	}
	return scores
}
func (r *request) comparisonCheck(c object, scores map[poolKey]score) object {
	o := checkOutput(str(c["id"]))
	short, cap := len(scores) == 0, false
	for _, s := range scores {
		short = short || s.rule == P1
		cap = cap || s.rule == P0
	}
	if len(scores) == 0 || short && !cap {
		decision(o, Unobserved, K1, TrialsShort)
		return o
	}
	var first object
	comparable := true
	var ka, na, kb, nb int64
	for k, s := range scores {
		if k.arm == ArmA {
			ka += s.k
			na += s.n
		} else {
			kb += s.k
			nb += s.n
		}
		for _, t := range s.scored {
			if t["sampling"] == nil {
				comparable = false
			}
			if first == nil {
				first = t
			} else if first["model"] != t["model"] || first["framing"] != t["framing"] || !equalJSON(first["sampling"], t["sampling"]) {
				comparable = false
			}
		}
	}
	if !comparable {
		decision(o, Unobserved, K2, ArmsNotComparable)
		return o
	}
	if cap {
		decision(o, Unobserved, K3, ErrorCap)
		return o
	}
	lo, hi, detail := difference(ka, na, kb, nb)
	o["d_lo"] = boundText(lo)
	o["d_hi"] = boundText(hi)
	switch detail {
	case Undecided:
		decision(o, Unobserved, K4, IntervalUndecided)
	case DetailRegressed:
		o["detail"] = detail
		decision(o, Failed, K4, nil)
	case DetailImproved, NoMaterialChange:
		o["detail"] = detail
		decision(o, HeldCrediting, K4, nil)
	}
	return o
}

var asideRank = map[Reason]int{"wrong_tier": 0, "other_arm": 1, "earlier_pool": 2}

func (r *request) setAside() (array, array) {
	aside := array{}
	earlier := map[poolKey]bool{}
	for _, v := range arr(r.input["trials"]) {
		t := obj(v)
		c := r.byID[str(t["check_id"])]
		reason := Reason("")
		if wrongTier(enum[Tier](t["tier"]), enum[Tier](r.input["claimed_tier"])) {
			reason = WrongTier
		} else if !r.usedArm(enum[Arm](t["arm"])) {
			reason = OtherArm
		} else {
			field := "condition"
			if enum[Arm](t["arm"]) == ArmA {
				field = "condition_a"
			}
			if c[field] == nil || t["condition"] != c[field] {
				reason = EarlierPool
				earlier[key(t)] = true
			}
		}
		if reason != "" {
			aside = append(aside, object{"check_id": t["check_id"], "arm": t["arm"], "condition": t["condition"], "trial_index": integer(t["trial_index"]), "reason": reason})
		}
	}
	sort.Slice(aside, func(i, j int) bool {
		a, b := obj(aside[i]), obj(aside[j])
		if str(a["check_id"]) != str(b["check_id"]) {
			return str(a["check_id"]) < str(b["check_id"])
		}
		if enum[Arm](a["arm"]) != enum[Arm](b["arm"]) {
			return enum[Arm](a["arm"]) < enum[Arm](b["arm"])
		}
		if str(a["condition"]) != str(b["condition"]) {
			return str(a["condition"]) < str(b["condition"])
		}
		ia, oka := a["trial_index"].(int64)
		ib, okb := b["trial_index"].(int64)
		if !oka || !okb {
			panic("invalid index")
		}
		if ia != ib {
			return ia < ib
		}
		return asideRank[enum[Reason](a["reason"])] < asideRank[enum[Reason](b["reason"])]
	})
	keys := make([]poolKey, 0, len(earlier))
	for k := range earlier {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.id != b.id {
			return a.id < b.id
		}
		if a.arm != b.arm {
			return a.arm < b.arm
		}
		return a.condition < b.condition
	})
	pools := array{}
	for _, k := range keys {
		p := r.pools[k]
		malformed := len(p.counts) != 1 || p.counts[0] || p.badOutcome
		for i, t := range p.trials {
			if integer(t["trial_index"]) != int64(i) {
				malformed = true
			}
		}
		var count int64
		for n := range p.counts {
			count = n
		}
		o := object{"check_id": k.id, "arm": nil, "condition": k.condition, "k": nil, "n": nil, "rule": P1}
		if k.arm != NoArm {
			o["arm"] = k.arm
		}
		if !malformed {
			s := scorePool(p, count)
			o["k"] = s.k
			o["n"] = s.n
			o["rule"] = s.rule
		}
		pools = append(pools, o)
	}
	return aside, pools
}

func equalJSON(a, b any) bool { return strictjson.EqualJSON(a, b) }

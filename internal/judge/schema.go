package judge

import (
	"strconv"
	"strings"
)

type shape struct {
	kinds    string
	values   []string
	nullable bool
	fields   map[string]*shape
	item     *shape
	min      int
}

func scalar(k string) *shape            { return &shape{kinds: k} }
func nullable(s *shape) *shape          { c := *s; c.nullable = true; return &c }
func enumeration(values string) *shape  { return &shape{kinds: "s", values: strings.Fields(values)} }
func list(s *shape) *shape              { return &shape{kinds: "a", item: s} }
func fields(f map[string]*shape) *shape { return &shape{kinds: "o", fields: f} }

var dimensions = []Dimension{Availability, Execution, GracefulFailure, Robustness, ResponseCorrectness, Determinism, CLIMCPParity, ProvenanceHonesty, CompletenessSignaling, OutputConformance, FlagConformance, Bounding, Discoverability, ErrorMessageQuality, TokenEconomy, SideEffectIntegrity, Idempotency, DestructiveGuard, PIISecretHygiene, Steering, ToolDescriptionEfficacy}
var dimensionNames = func() []string {
	names := make([]string, len(dimensions))
	for i, d := range dimensions {
		names[i] = string(d)
	}
	return names
}()

var inputShape = makeInputShape()

func makeInputShape() *shape {
	s := scalar("s")
	ns := nullable(s)
	i := scalar("i")
	ni := nullable(i)
	b := scalar("b")
	nb := nullable(b)
	dim := &shape{kinds: "s", values: dimensionNames}
	kind := enumShape(Plain, Prompt, Comparison)
	tier := enumShape(Offline, Live, Replay, Scripted, Mixed)
	trialTier := enumShape(Offline, Live, Replay, Scripted)
	outcome := enumShape(ObservedHeld, ObservedNotHeld, NotObserved)
	evaluator := fields(map[string]*shape{"kind": enumShape(DeterministicEvaluator, ModelEvaluator), "model": ns})
	target := fields(map[string]*shape{"kind": enumeration("invocation artifact"), "id": s, "tier": trialTier})
	targets := list(target)
	targets.min = 1
	entry := fields(map[string]*shape{"check_id": s, "kind": kind, "dimension": dim, "result": enumShape(BaselineHeld, BaselineFailed), "rulebook_version": s, "tier": tier, "condition": ns, "k": ni, "n": ni})
	return fields(map[string]*shape{
		"request": enumShape(Grade, Anchor), "scope": list(s), "claimed_tier": enumShape(Live, Scripted),
		"subject_facts": fields(map[string]*shape{"read_only": nb, "has_destructive_ops": nb, "store_backed": nb, "has_cli": nb, "bounding": nullable(enumShape(NoBounding, DeclaredBounding))}),
		"required":      list(fields(map[string]*shape{"id": s, "kind": kind, "dimension": dim, "record": ns, "crediting": b, "empty_satisfiable": b, "asserts_emptiness": b, "precondition": ns, "declared_count": ni, "condition": ns, "condition_a": ns})),
		"evidence":      list(fields(map[string]*shape{"check_id": s, "outcome": outcome, "reason": nullable(enumShape(HarnessFault, CouldNotStart, InputMissing, PreconditionUnmet, EmptyTarget, PlaceholderUnfilled, ComputationFailed, EarlierStepFailed, NetworkAttempted, NetworkUnverifiable, LiveNotRun, JudgementUnverified)), "failure_kind": nullable(enumeration("predicate wrong_exit crashed over_budget")), "target_size": ni, "precondition_observed": nb, "tier": tier, "expected": scalar("*"), "observed": scalar("*"), "targets": targets, "evaluator": evaluator})),
		"trials":        list(fields(map[string]*shape{"check_id": s, "arm": nullable(enumShape(ArmA, ArmB)), "condition": s, "trial_index": i, "declared_count": i, "outcome": outcome, "errored": b, "tier": trialTier, "model": s, "sampling": nullable(scalar("o")), "framing": s, "evaluator": evaluator, "grade": nullable(enumeration("full partial none"))})),
		"markers":       list(fields(map[string]*shape{"record": s, "dimension": dim})), "baseline": nullable(fields(map[string]*shape{"entries": list(entry)})), "retired": list(s), "accepted_regressions": list(s),
	})
}
func valid(v any, s *shape) bool {
	if s.kinds == "*" {
		return true
	}
	if v == nil {
		return s.nullable
	}
	switch x := v.(type) {
	case string:
		if s.kinds != "s" {
			return false
		}
		if s.values != nil {
			for _, e := range s.values {
				if e == x {
					return true
				}
			}
			return false
		}
		return true
	case bool:
		return s.kinds == "b"
	case number:
		if s.kinds != "i" || strings.ContainsAny(string(x), ".eE") {
			return false
		}
		n, e := strconv.ParseInt(string(x), 10, 64)
		return e == nil && n >= 0 && n <= maxInteger
	case array:
		if s.kinds != "a" || len(x) < s.min {
			return false
		}
		for _, e := range x {
			if !valid(e, s.item) {
				return false
			}
		}
		return true
	case object:
		if s.kinds != "o" {
			return false
		}
		if s.fields == nil {
			return true
		}
		if len(x) != len(s.fields) {
			return false
		}
		for k, f := range s.fields {
			e, ok := x[k]
			if !ok || !valid(e, f) {
				return false
			}
		}
		return true
	default:
		panic("unknown JSON node")
	}
}

func enumShape[T ~string](values ...T) *shape {
	names := make([]string, len(values))
	for i, v := range values {
		names[i] = string(v)
	}
	return &shape{kinds: "s", values: names}
}

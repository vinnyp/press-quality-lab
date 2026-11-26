package judge

import "github.com/vinnyp/press-quality-lab/internal/rulebook"

// Dimension is a routing destination in the fixed dimension table.
type Dimension = rulebook.Dimension

const (
	Availability            = rulebook.Availability
	Execution               = rulebook.Execution
	GracefulFailure         = rulebook.GracefulFailure
	Robustness              = rulebook.Robustness
	ResponseCorrectness     = rulebook.ResponseCorrectness
	Determinism             = rulebook.Determinism
	CLIMCPParity            = rulebook.CLIMCPParity
	ProvenanceHonesty       = rulebook.ProvenanceHonesty
	CompletenessSignaling   = rulebook.CompletenessSignaling
	OutputConformance       = rulebook.OutputConformance
	FlagConformance         = rulebook.FlagConformance
	Bounding                = rulebook.Bounding
	Discoverability         = rulebook.Discoverability
	ErrorMessageQuality     = rulebook.ErrorMessageQuality
	TokenEconomy            = rulebook.TokenEconomy
	SideEffectIntegrity     = rulebook.SideEffectIntegrity
	Idempotency             = rulebook.Idempotency
	DestructiveGuard        = rulebook.DestructiveGuard
	PIISecretHygiene        = rulebook.PIISecretHygiene
	Steering                = rulebook.Steering
	ToolDescriptionEfficacy = rulebook.ToolDescriptionEfficacy
)

// Group names a fixed group of dimensions.
type Group string

const (
	Operability   Group = "Operability"
	Correctness   Group = "Correctness"
	Conformance   Group = "Conformance"
	Usability     Group = "Usability"
	Safety        Group = "Safety"
	SteeringGroup Group = "Steering"
)

// Scope names a requested part.
type Scope = rulebook.Scope

const (
	ConformanceScope = rulebook.ConformanceScope
	DestructiveScope = rulebook.DestructiveScope
	SteeringScope    = rulebook.SteeringScope
	CompareScope     = rulebook.CompareScope
)

// Kind identifies a required check's evaluation method.
type Kind string

const (
	Plain      Kind = "plain"
	Prompt     Kind = "prompt"
	Comparison Kind = "comparison"
)

// Tier identifies how evidence was collected.
type Tier string

const (
	Offline  Tier = "offline"
	Live     Tier = "live"
	Replay   Tier = "replay"
	Scripted Tier = "scripted"
	Mixed    Tier = "mixed"
)

// Request identifies the requested operation.
type Request string

const (
	Grade  Request = "grade"
	Anchor Request = "anchor"
)

// Arm identifies a trial arm, with NoArm represented by null on the wire.
type Arm string

const (
	NoArm Arm = ""
	ArmA  Arm = "A"
	ArmB  Arm = "B"
)

// EvidenceOutcome is the reported observation.
type EvidenceOutcome string

const (
	ObservedHeld    EvidenceOutcome = "held"
	ObservedNotHeld EvidenceOutcome = "not_held"
	NotObserved     EvidenceOutcome = "not_observed"
)

// CheckResult is the judge's result for a check.
type CheckResult string

const (
	Unobserved       CheckResult = "unobserved"
	Failed           CheckResult = "failed"
	FailedExpected   CheckResult = "failed_expected"
	HeldCrediting    CheckResult = "held_crediting"
	HeldNonCrediting CheckResult = "held_non_crediting"
	OutOfScope       CheckResult = "out_of_scope"
)

// DimensionStatus is the state of one dimension.
type DimensionStatus string

const (
	DimensionOutOfScope DimensionStatus = "OUT-OF-SCOPE"
	DimensionRed        DimensionStatus = "RED"
	DimensionNA         DimensionStatus = "N/A"
	DimensionHarness    DimensionStatus = "HARNESS"
	DimensionXFail      DimensionStatus = "XFAIL"
	DimensionXPass      DimensionStatus = "XPASS"
	DimensionGreen      DimensionStatus = "GREEN"
	DimensionUncovered  DimensionStatus = "UNCOVERED"
)

// GroupStatus is the state of one group.
type GroupStatus string

const (
	GroupOutOfScope  GroupStatus = "OUT-OF-SCOPE"
	GroupRed         GroupStatus = "RED"
	GroupGapped      GroupStatus = "GAPPED"
	GroupKnownDefect GroupStatus = "KNOWN-DEFECT"
	GroupGreen       GroupStatus = "GREEN"
	GroupNA          GroupStatus = "N/A"
)

// VerdictToken is the top-level verdict token.
type VerdictToken string

const (
	Passed      VerdictToken = "PASSED"
	RunFailed   VerdictToken = "FAILED"
	Provisional VerdictToken = "PROVISIONAL"
)

// SubLabel qualifies a provisional verdict.
type SubLabel string

const (
	KnownDefects SubLabel = "known-defects"
	Gaps         SubLabel = "gaps"
)

// PlainVerdict is the plain word attached to a verdict.
type PlainVerdict string

const (
	Green       PlainVerdict = "green"
	Red         PlainVerdict = "red"
	KnownDefect PlainVerdict = "known_defect"
	NotMeasured PlainVerdict = "not_measured"
)

// Outcome identifies the output document's operation result.
type Outcome string

const (
	VerdictOutcome Outcome = "verdict"
	Rejected       Outcome = "rejected"
	Anchored       Outcome = "anchored"
	Refused        Outcome = "refused"
)

// EntryClass is a baseline entry's comparison class.
type EntryClass string

const (
	SetAside    EntryClass = "set_aside"
	Missing     EntryClass = "missing"
	NotCompared EntryClass = "not_compared"
	Regressed   EntryClass = "regressed"
	Improved    EntryClass = "improved"
	Same        EntryClass = "same"
)

// ComparisonResult describes the baseline comparison.
type ComparisonResult string

const (
	NoComparison          ComparisonResult = "none"
	ComparisonRegressed   ComparisonResult = "regressed"
	ComparisonNotMeasured ComparisonResult = "not_measured"
	NotRegressed          ComparisonResult = "not_regressed"
)

// ComparisonDetail describes an interval decision.
type ComparisonDetail string

const (
	Undecided        ComparisonDetail = ""
	DetailRegressed  ComparisonDetail = "regressed"
	DetailImproved   ComparisonDetail = "improved"
	NoMaterialChange ComparisonDetail = "no_material_change"
)

// BaselineResult is the stored result of a measured check.
type BaselineResult string

const (
	BaselineHeld   BaselineResult = "held"
	BaselineFailed BaselineResult = "failed"
)

// Reason names a closed-list observation or judge reason.
type Reason string

const (
	HarnessFault         Reason = "harness_fault"
	CouldNotStart        Reason = "could_not_start"
	InputMissing         Reason = "input_missing"
	PreconditionUnmet    Reason = "precondition_unmet"
	EmptyTarget          Reason = "empty_target"
	PlaceholderUnfilled  Reason = "placeholder_unfilled"
	ComputationFailed    Reason = "computation_failed"
	EarlierStepFailed    Reason = "earlier_step_failed"
	NetworkAttempted     Reason = "network_attempted"
	NetworkUnverifiable  Reason = "network_unverifiable"
	LiveNotRun           Reason = "live_not_run"
	JudgementUnverified  Reason = "judgement_unverified"
	NoEvidence           Reason = "no_evidence"
	TrialsShort          Reason = "trials_short"
	ErrorCap             Reason = "error_cap"
	IntervalUndecided    Reason = "interval_undecided"
	ArmsNotComparable    Reason = "arms_not_comparable"
	ScopeNotRequested    Reason = "scope_not_requested"
	WrongTier            Reason = "wrong_tier"
	OtherArm             Reason = "other_arm"
	EarlierPool          Reason = "earlier_pool"
	TestRun              Reason = "test_run"
	RedReason            Reason = "red"
	NotMeasuredReason    Reason = "not_measured"
	StaleMarker          Reason = "stale_marker"
	UnretiredCheck       Reason = "unretired_check"
	UnacceptedRegression Reason = "unaccepted_regression"
)

// EvaluatorKind describes the evidence evaluator.
type EvaluatorKind string

const (
	DeterministicEvaluator EvaluatorKind = "deterministic"
	ModelEvaluator         EvaluatorKind = "model"
)

// BoundingFact identifies the declared bounding capability.
type BoundingFact string

const (
	NoBounding       BoundingFact = "none"
	DeclaredBounding BoundingFact = "declared"
)

// NextAction names the action requested by an incomplete comparison.
type NextAction string

const ReAnchor NextAction = "re_anchor"

func enum[T ~string](v any) T {
	if v == nil {
		return T("")
	}
	x, ok := v.(T)
	if !ok {
		panic("expected validated enum")
	}
	return x
}
func decodeEnums(in object) {
	in["request"] = Request(str(in["request"]))
	in["claimed_tier"] = Tier(str(in["claimed_tier"]))
	for i, v := range arr(in["scope"]) {
		arr(in["scope"])[i] = Scope(str(v))
	}
	f := obj(in["subject_facts"])
	if f["bounding"] != nil {
		f["bounding"] = BoundingFact(str(f["bounding"]))
	}
	for _, field := range []string{"required", "evidence", "trials", "markers"} {
		for _, v := range arr(in[field]) {
			o := obj(v)
			if o["dimension"] != nil {
				o["dimension"] = Dimension(str(o["dimension"]))
			}
			if o["tier"] != nil {
				o["tier"] = Tier(str(o["tier"]))
			}
			if o["outcome"] != nil {
				o["outcome"] = EvidenceOutcome(str(o["outcome"]))
			}
			if o["reason"] != nil {
				o["reason"] = reportedReason(Reason(str(o["reason"])))
			}
			switch field {
			case "required":
				o["kind"] = Kind(str(o["kind"]))
			case "trials":
				if o["arm"] != nil {
					o["arm"] = Arm(str(o["arm"]))
				}
				e := obj(o["evaluator"])
				e["kind"] = EvaluatorKind(str(e["kind"]))
			}
		}
	}
	if in["baseline"] != nil {
		for _, v := range arr(obj(in["baseline"])["entries"]) {
			o := obj(v)
			o["kind"] = Kind(str(o["kind"]))
			o["dimension"] = Dimension(str(o["dimension"]))
			o["tier"] = Tier(str(o["tier"]))
			o["result"] = BaselineResult(str(o["result"]))
		}
	}
}

// reportedReason distinguishes the family's closed list from reasons produced
// only by the judge. The schema guarantees this invariant at the boundary.
func reportedReason(reason Reason) Reason {
	switch reason {
	case HarnessFault, CouldNotStart, InputMissing, PreconditionUnmet, EmptyTarget,
		PlaceholderUnfilled, ComputationFailed, EarlierStepFailed, NetworkAttempted,
		NetworkUnverifiable, LiveNotRun, JudgementUnverified:
		return reason
	case NoEvidence, TrialsShort, ErrorCap, IntervalUndecided, ArmsNotComparable,
		ScopeNotRequested, WrongTier, OtherArm, EarlierPool, TestRun, RedReason,
		NotMeasuredReason, StaleMarker, UnretiredCheck, UnacceptedRegression:
		panic("judge-only reason reported as evidence")
	}
	panic("unknown reason")
}

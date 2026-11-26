// Package rulebook holds shared scope and structural applicability rules.
package rulebook

// Dimension is a routing destination in the fixed dimension table.
type Dimension string

const (
	Availability            Dimension = "availability"
	Execution               Dimension = "execution"
	GracefulFailure         Dimension = "graceful-failure"
	Robustness              Dimension = "robustness"
	ResponseCorrectness     Dimension = "response-correctness"
	Determinism             Dimension = "determinism"
	CLIMCPParity            Dimension = "cli-mcp-parity"
	ProvenanceHonesty       Dimension = "provenance-honesty"
	CompletenessSignaling   Dimension = "completeness-signaling"
	OutputConformance       Dimension = "output-conformance"
	FlagConformance         Dimension = "flag-conformance"
	Bounding                Dimension = "bounding"
	Discoverability         Dimension = "discoverability"
	ErrorMessageQuality     Dimension = "error-message-quality"
	TokenEconomy            Dimension = "token-economy"
	SideEffectIntegrity     Dimension = "side-effect-integrity"
	Idempotency             Dimension = "idempotency"
	DestructiveGuard        Dimension = "destructive-guard"
	PIISecretHygiene        Dimension = "pii-secret-hygiene"
	Steering                Dimension = "steering"
	ToolDescriptionEfficacy Dimension = "tool-description-efficacy"
)

// Scope names a requested part.
type Scope string

const (
	ConformanceScope Scope = "conformance"
	DestructiveScope Scope = "destructive"
	SteeringScope    Scope = "steering"
	CompareScope     Scope = "compare"
)

// ScopeIncludes reports whether a scope part includes a dimension (rulebook §3).
func ScopeIncludes(scope Scope, d Dimension) bool {
	switch scope {
	case ConformanceScope, DestructiveScope, SteeringScope, CompareScope:
	default:
		panic("unknown scope")
	}
	switch d {
	case CLIMCPParity:
		return false
	case DestructiveGuard:
		return scope == DestructiveScope
	case Steering:
		return scope == SteeringScope
	case ToolDescriptionEfficacy:
		return scope == CompareScope
	case Availability, Execution, GracefulFailure, Robustness, ResponseCorrectness, Determinism, ProvenanceHonesty, CompletenessSignaling, OutputConformance, FlagConformance, Bounding, Discoverability, ErrorMessageQuality, TokenEconomy, SideEffectIntegrity, Idempotency, PIISecretHygiene:
		return scope == ConformanceScope
	}
	panic("unknown dimension")
}

// AlwaysApplicable reports whether a dimension has no structural deciding fact (§6).
func AlwaysApplicable(d Dimension) bool {
	switch d {
	case Bounding, Idempotency, DestructiveGuard, SideEffectIntegrity, FlagConformance:
		return false
	case Availability, Execution, GracefulFailure, Robustness, ResponseCorrectness, Determinism, CLIMCPParity, ProvenanceHonesty, CompletenessSignaling, OutputConformance, Discoverability, ErrorMessageQuality, TokenEconomy, PIISecretHygiene, Steering, ToolDescriptionEfficacy:
		return true
	}
	panic("unknown dimension")
}

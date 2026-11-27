package decide

// Reasons returns QE13's closed collector reason set as a fresh slice.
func Reasons() []Reason {
	return []Reason{
		"network_unverifiable",
		"could_not_start",
		"input_missing",
		"precondition_unmet",
		"empty_target",
		"placeholder_unfilled",
		"computation_failed",
		"earlier_step_failed",
		"harness_fault",
	}
}

// ValidReason reports whether a collector can emit the reason.
func ValidReason(reason Reason) bool {
	for _, allowed := range Reasons() {
		if reason == allowed {
			return true
		}
	}
	return false
}

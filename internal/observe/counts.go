package observe

import "fmt"

// MaxCount is the judge input's largest exact integer count.
const MaxCount int64 = 1<<53 - 1

// ValidateCounts refuses facts that cannot be carried to the judge exactly.
func ValidateCounts(r Run, sum bool) error {
	if r.StdoutSeen < 0 || r.StderrSeen < 0 || r.StdoutSeen > MaxCount || r.StderrSeen > MaxCount {
		return fmt.Errorf("QE10: byte count outside judge range")
	}
	if sum && r.StdoutSeen > MaxCount-r.StderrSeen {
		return fmt.Errorf("QE8: byte bound sum outside judge range")
	}
	return nil
}

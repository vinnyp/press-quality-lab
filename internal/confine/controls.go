package confine

import (
	"bytes"
	"time"

	"github.com/vinnyp/press-quality-lab/internal/observe"
)

// ControlRun includes the complete launcher lifetime, not just program time.
type ControlRun struct {
	Run     observe.Run
	Elapsed time.Duration
}

// ControlFacts is the complete raw result of U29's grade-wide harness.
type ControlFacts struct {
	// Error invalidates all suite facts, even those collected before the failure.
	Error                error
	Subject, Computation []observe.Run
	Fast                 []ControlRun
	CalibrationRead      observe.Run
	Before, After        time.Time
}

func (f ControlFacts) Apply(c observe.Controls) observe.Controls {
	if f.Error != nil {
		f = ControlFacts{}
	}
	return ApplyControlResults(c, f.Subject, f.Computation, f.Fast, f.Before, f.After, CalibrationReadPassed(f.CalibrationRead))
}

func CalibrationReadPassed(r observe.Run) bool {
	return cleanEnd(r) && len(r.Stderr) == 0 && bytes.Equal(r.Stdout, []byte("calibration_value\n"))
}

// KnownFastWithin implements QH24. Missing, invalid or incomplete runs cannot
// establish a host's bound, even when the measured durations are small.
func KnownFastWithin(runs []ControlRun) bool {
	if len(runs) != 5 {
		return false
	}
	for _, r := range runs {
		if !cleanEnd(r.Run) || len(r.Run.Stdout) != 0 || len(r.Run.Stderr) != 0 || r.Elapsed < 0 || r.Elapsed > 2*time.Second {
			return false
		}
	}
	return true
}

func cleanEnd(r observe.Run) bool {
	return r.Markers.Confinement && !r.Markers.CouldNotStart && !r.CopyChanged && !r.End.EngineKilled && !r.End.BudgetExpired && !r.OutputCutAfterExit && !r.StdoutCut && !r.StderrCut && r.End.Signal == 0 && r.End.Code == 0
}

// CalibrationMoved compares the observed access times, at native precision.
// A successful read is also required; metadata alone does not prove a read.
func CalibrationMoved(before, after time.Time, readOK bool) bool {
	return readOK && !before.IsZero() && after.After(before)
}

// SelfTestProgram describes one QH9 witness. Target arguments are supplied by
// the decoy harness; this list describes operations, never live subject data.
type SelfTestProgram struct {
	Name   string
	Killed bool
	Output string
}

// SelfTestPrograms returns a fresh list so callers cannot change the oracle.
func SelfTestPrograms(computation bool) []SelfTestProgram {
	var programs []SelfTestProgram
	for _, name := range []string{
		"tcp",
		"unix",
		"udp",
		"bind",
		"listen",
		"read",
		"map",
		"write",
		"create",
		"service",
		"sysctl",
		"arguments",
		"environment",
		"process",
		"signal",
		"codesign",
		"priority",
		"resource",
	} {
		programs = append(programs, SelfTestProgram{Name: name, Killed: true})
	}
	if computation {
		programs = append(programs, SelfTestProgram{Name: "setsid", Killed: true}, SelfTestProgram{Name: "setpgid", Killed: true}, SelfTestProgram{Name: "child", Output: "attempt_child\nchild_ok\n"})
	} else {
		programs = append(programs, SelfTestProgram{Name: "child", Killed: true})
	}
	return append(programs, SelfTestProgram{Name: "clean", Output: "clean_value\n"})
}

// SelfTestPassed requires every expected witness, in order, for both profiles.
// Its caller retains the raw runs for diagnostics, including pre-attempt kills.
func SelfTestPassed(subject, computation []observe.Run) bool {
	for i, runs := range [][]observe.Run{subject, computation} {
		programs := SelfTestPrograms(i == 1)
		if len(runs) != len(programs) {
			return false
		}
		for j, program := range programs {
			if !SelfTestProgramPassed(program, runs[j]) {
				return false
			}
		}
	}
	return true
}

// SelfTestProgramPassed checks the attempt line as well as the raw end. A kill
// before the attempted operation, or a budget kill, proves no refusal.
func SelfTestProgramPassed(p SelfTestProgram, r observe.Run) bool {
	if !p.Killed {
		return cleanEnd(r) && len(r.Stderr) == 0 && bytes.Equal(r.Stdout, []byte(p.Output))
	}
	return r.Markers.Confinement && !r.Markers.CouldNotStart && !r.CopyChanged && !r.End.EngineKilled && !r.End.BudgetExpired && !r.End.MemoryKilled && !r.End.CodeSigningKilled && r.End.ExitDetail&0x70000 == 0 && !r.OutputCutAfterExit && !r.StdoutCut && !r.StderrCut && r.End.Signal == 9 && len(r.Stderr) == 0 && bytes.Equal(r.Stdout, []byte("attempt_"+p.Name+"\n"))
}

// ApplyControlResults updates only the three facts U29 collects. QO1 and QO4
// remain the responsibility of the existing untagged decision path.
func ApplyControlResults(c observe.Controls, subject, computation []observe.Run, fast []ControlRun, before, after time.Time, readOK bool) observe.Controls {
	c.SelfTestPassed = SelfTestPassed(subject, computation)
	c.KnownFastWithin = KnownFastWithin(fast)
	c.ReadCalibrationMoved = CalibrationMoved(before, after, readOK)
	return c
}

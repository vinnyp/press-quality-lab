package observe

import (
	"reflect"
	"testing"
)

func TestHelpWrittenBytesAndCut(t *testing.T) {
	// QD2, QD5, E-32: retained emptiness never selects a different stream.
	for _, row := range []struct {
		name    string
		run     Run
		want    []string
		heading bool
	}{
		{"CutStdout", Run{StdoutSeen: 1 << 21, StdoutCut: true, Stderr: []byte("Commands:\n  err  desc"), StderrSeen: 21}, nil, false},
		{"CutStderr", Run{Stdout: []byte("Commands:\n  out  desc"), StdoutSeen: 21, StderrCut: true}, []string{"out"}, true},
		{"WrittenStdoutEmptyRetained", Run{StdoutSeen: 10, Stderr: []byte("Commands:\n  err  desc"), StderrSeen: 21}, nil, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			got, h := HelpCommands(row.run)
			if !reflect.DeepEqual(got, row.want) || h != row.heading {
				t.Fatal(got, h)
			}
		})
	}
}

func TestProjectionAttributionOrder(t *testing.T) {
	// QH21, QO4, QT5: engine budget kills take precedence over memory details.
	for _, memory := range []bool{false, true} {
		e := End{Signal: 9, EngineKilled: true, BudgetExpired: true, RunningAtBudget: true, MemoryKilled: memory}
		want := map[string]any{"kind": "signal", "signal": "SIGKILL", "by": "budget"}
		if got, err := ProjectEnd(e, Markers{Confinement: true}); err != nil || !reflect.DeepEqual(got, want) {
			t.Fatal(got)
		}
	}
}

func TestExitRacingEngineKill(t *testing.T) {
	// QO5, QH21: a sent group kill cannot replace the leader's observed normal exit.
	for _, code := range []int{0, 2} {
		a, err := Attribute(Run{Markers: Markers{Confinement: true}, End: End{Code: code, EngineKilled: true, BudgetExpired: true, RunningAtBudget: true}})
		if err != nil || a != Exited {
			t.Fatal(a, err)
		}
	}
}

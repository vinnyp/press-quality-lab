package observe

import "testing"

func TestDroppedRunProjection(t *testing.T) {
	// QT5, QE10: an intentionally discarded capture cannot masquerade as empty bytes.
	ProjectRun(Run{})
	defer func() {
		if recover() == nil {
			t.Fatal("dropped bytes projected")
		}
	}()
	ProjectRun(Run{BytesDropped: true})
}

func TestUnrepresentableKillProjection(t *testing.T) {
	// QT5, QO4, QO5: no QT5 by value expresses an engine nonbudget kill or an unassigned budget-time kill.
	for _, row := range []struct {
		name string
		end  End
		bad  bool
	}{
		{"Confinement", End{Signal: 9}, false},
		{"Engine", End{Signal: 9, EngineKilled: true}, true},
		{"EngineExpiredNotRunning", End{Signal: 9, EngineKilled: true, BudgetExpired: true}, true},
		{"EngineRunningNoBudget", End{Signal: 9, EngineKilled: true, RunningAtBudget: true}, true},
		{"Expired", End{Signal: 9, BudgetExpired: true}, true},
		{"ExpiredRunning", End{Signal: 9, BudgetExpired: true, RunningAtBudget: true}, true},
		{"Budget", End{Signal: 9, EngineKilled: true, BudgetExpired: true, RunningAtBudget: true}, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			got, err := ProjectEnd(row.end, Markers{Confinement: true})
			if (err != nil) != row.bad || row.bad && got != nil {
				t.Fatal(got, err)
			}
		})
	}
}

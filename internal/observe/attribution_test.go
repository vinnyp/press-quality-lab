package observe

import (
	"fmt"
	"testing"
)

func TestAttributionRawTable(t *testing.T) {
	// QH6, QH7, E-28, QH21, QO2, QO3, QO4, QO5: the complete raw-fact product.
	// Each row carries the clause supplying its literal oracle, independently of Attribute.
	for _, marker := range []bool{false, true} {
		for _, start := range []bool{false, true} {
			for _, signal := range []int{0, 9, 11} {
				for _, engine := range []bool{false, true} {
					for _, budget := range []bool{false, true} {
						for _, running := range []bool{false, true} {
							for _, detail := range []uint32{0, 0x10000, 0x20000, 0x40000, 0x60000} {
								want, clause, fault := Exited, "QO5", false
								switch {
								case !marker:
									want, clause = Unconfined, "QH6_E28"
								case running && !budget, signal == 0 && detail != 0, signal == 11 && (engine || detail&0x60000 != 0):
									fault, clause = true, "QH21"
								case start:
									want, clause = CouldNotStart, "QH7_QO2"
								case signal == 0:
									want, clause = Exited, "QO5"
								case signal == 11:
									want, clause = Crashed, "QO5"
								case engine && budget:
									want, clause = BudgetKill, "QO4"
								case engine:
									want, clause = EngineKill, "QO5"
								case detail&0x60000 != 0:
									want, clause = Crashed, "QO5"
								case !budget:
									want, clause = ConfinementKill, "QH21_QO3"
								default:
									want, clause = Crashed, "QO5"
								}
								name := fmt.Sprintf("%s_M%t_S%t_Sig%d_E%t_B%t_R%t_D%d", clause, marker, start, signal, engine, budget, running, detail)
								t.Run(name, func(t *testing.T) {
									for _, code := range []int{0, 2} {
										r := Run{Markers: Markers{Confinement: marker, CouldNotStart: start}, End: End{Signal: signal, Code: code, EngineKilled: engine, BudgetExpired: budget, RunningAtBudget: running, ExitDetail: detail}}
										got, err := Attribute(r)
										if (err != nil) != fault || !fault && got != want {
											t.Fatalf("%s: got %v err %v want %v fault %t", clause, got, err, want, fault)
										}
									}
								})
							}
						}
					}
				}
			}
		}
	}
}

func TestExitDetailDecoder(t *testing.T) {
	// QH21, QO5: Darwin exit detail distinguishes memory and code-signing kills.
	for _, row := range []struct {
		name            string
		raw             uint32
		memory, signing bool
	}{
		{"None", 0, false, false}, {"Memory", 0x20000, true, false}, {"CodeSigning", 0x40000, false, true}, {"Both", 0x60000, true, true}, {"Other", 0x10000, false, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			m, s := DecodeExitDetail(row.raw)
			if m != row.memory || s != row.signing {
				t.Fatalf("%t %t", m, s)
			}
		})
	}
}

func TestSignalNames(t *testing.T) {
	// QH21, QO5: either source's signal representation identifies SIGKILL; conflicting facts are a fault.
	for _, row := range []struct {
		name  string
		end   End
		want  Attribution
		fault bool
	}{
		{"NamedKill", End{SignalName: "SIGKILL"}, ConfinementKill, false},
		{"NumberedKill", End{Signal: 9}, ConfinementKill, false},
		{"BothKill", End{Signal: 9, SignalName: "SIGKILL"}, ConfinementKill, false},
		{"OtherNamed", End{SignalName: "SIG999"}, Crashed, false},
		{"OtherBoth", End{Signal: -1, SignalName: "SIG999"}, Crashed, false},
		{"ContradictoryNumber", End{Signal: 11, SignalName: "SIGKILL"}, 0, true},
		{"ContradictoryName", End{Signal: 9, SignalName: "SIGTERM"}, 0, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			got, err := Attribute(Run{Markers: Markers{Confinement: true}, End: row.end})
			if (err != nil) != row.fault || !row.fault && got != row.want {
				t.Fatalf("got %v error %v", got, err)
			}
		})
	}
}

package decide

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"testing"
)

func TestPlantedCombination(t *testing.T) {
	// QO7, QP11, QP12, QP13: unsuccessful exits and crashes prove no read.
	ch := plan.Check{Preimage: plan.Object{"probe": "planted_secret"}, Keys: []plan.Object{{}}}
	for _, row := range []struct {
		name string
		run  observe.Run
		want Result
	}{
		{"Crash", observe.Run{Markers: observe.Markers{Confinement: true}, End: observe.End{Signal: 11}}, Result{Level: 8, Outcome: NotObserved, Reason: "input_missing"}},
		{"Leak", observe.Run{Markers: observe.Markers{Confinement: true}, Details: observe.RunDetails{LeakScan: observe.LeakScan{Leaks: []observe.Leak{{Place: "stderr"}}}}}, Result{Level: 8, Outcome: "not_held", FailureKind: "predicate"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			e := Evidence{Invocations: []observe.Invocation{{Run: row.run}}}
			got := level7(ch, passing(), e)
			if got.Level == 0 {
				got = level8(ch, passing(), e)
			}
			if got != row.want {
				t.Fatalf("got %+v want %+v", got, row.want)
			}
		})
	}
}

func TestStoreProbeRules(t *testing.T) {
	// QP6, QP7, QP8, QP9: exclusions are narrow; empty target and control precede changes; E-10 faults cannot hold.
	yes := true
	no := false
	empty := emptyDigest()
	nonempty := "nonempty"
	base := plan.Check{Preimage: plan.Object{"probe": "store_unchanged", "store": "data", "bookkeeping": plan.Array{"cache/journal"}}, Keys: []plan.Object{{}}, Seed: []plan.Entry{{Path: "data", Data: []byte("seed")}}}
	for _, row := range []struct {
		name, path, kind, change string
		digest                   *string
		control                  *bool
		unreadable               bool
		out                      Outcome
		reason                   Reason
	}{
		{"Unchanged", "", "", "", nil, &yes, false, "held", ""},
		{"Changed", "data", "file", "modified", &nonempty, &yes, false, "not_held", ""},
		{"Bookkeeping", "cache/journal/a", "file", "created", &nonempty, &yes, false, "held", ""},
		{"ParentCreated", "cache", "directory", "created", nil, &yes, false, "held", ""},
		{"Sibling", "cache/other", "file", "created", &nonempty, &yes, false, "not_held", ""},
		{"Temp", ".pql-tmp/a", "file", "created", &nonempty, &yes, false, "held", ""},
		{"LinkExcluded", "cache/journal/link", "link", "created", nil, &yes, false, "not_held", ""},
		{"SpecialExcluded", ".pql-tmp/fifo", "special", "created", nil, &yes, false, "not_held", ""},
		{"Shm", "data-shm", "file", "created", &nonempty, &yes, false, "held", ""},
		{"EmptyWal", "data-wal", "file", "created", &empty, &yes, false, "held", ""},
		{"EmptyJournal", "data-journal", "file", "modified", &empty, &yes, false, "held", ""},
		{"RemovedWal", "data-wal", "file", "removed", nil, &yes, false, "held", ""},
		{"WalBytes", "data-wal", "file", "created", &nonempty, &yes, false, "not_held", ""},
		{"Unreadable", "", "", "", nil, &yes, true, NotObserved, "harness_fault"},
		{"ChangeAndUnreadable", "data", "file", "modified", &nonempty, &yes, true, "not_held", ""},
		{"ControlBeforeChange", "data", "file", "modified", &nonempty, &no, false, NotObserved, "harness_fault"},
	} {
		t.Run(row.name, func(t *testing.T) {
			home := observe.HomeChanges{ControlDetected: row.control, Unreadable: row.unreadable}
			if row.path != "" {
				home.Changes = []observe.HomeChange{{Path: row.path, Kind: row.kind, Change: row.change, Digest: row.digest}}
			}
			got := storeUnchanged(base, Evidence{Invocations: []observe.Invocation{{Run: observe.Run{Details: observe.RunDetails{Home: home}}}}})
			failure := ""
			if row.out == "not_held" {
				failure = "predicate"
			}
			want := Result{Level: 8, Outcome: row.out, Reason: row.reason, FailureKind: failure}
			if got != want {
				t.Fatalf("got %+v want %+v", got, want)
			}
		})
	}
	for _, row := range []struct {
		name  string
		seed  []plan.Entry
		store any
	}{{"NoStore", base.Seed, nil}, {"EmptySeed", nil, "data"}, {"ZeroFile", []plan.Entry{{Path: "data"}}, "data"}, {"ShmOnly", []plan.Entry{{Path: "data-shm", Data: []byte("index")}}, "data"}} {
		t.Run(row.name, func(t *testing.T) {
			ch := base
			ch.Seed = row.seed
			ch.Preimage = plan.Object{"probe": "store_unchanged", "store": row.store}
			got := storeUnchanged(ch, Evidence{})
			if got != (Result{Level: 8, Outcome: NotObserved, Reason: "empty_target"}) {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestPlantedReadConditions(t *testing.T) {
	// QP11, QO7: four independent read conditions, E-37 attribution and E-38 calibration.
	ch := plan.Check{Preimage: plan.Object{"probe": "planted_secret"}, Keys: []plan.Object{{}}, Subject: plan.Object{"commands": plan.Array{plan.Object{"words": plan.Array{"read"}, "exit_codes": plan.Array{strictjson.Number("3")}}}}}
	clean := observe.Run{Markers: observe.Markers{Confinement: true}, End: observe.End{Code: 3}, Details: observe.RunDetails{Planted: &observe.Planted{AtimeMoved: true, SameFile: true, ControlEnd: &observe.InvocationProtocol{End: observe.End{Signal: 9}, Markers: observe.Markers{Confinement: true}}}}}
	for _, row := range []struct {
		name   string
		change func(*observe.Run, *observe.Controls)
		reason Reason
	}{
		{"Proved", func(*observe.Run, *observe.Controls) {}, ""},
		{"Unsuccessful", func(r *observe.Run, _ *observe.Controls) { r.End.Code = 0 }, "input_missing"},
		{"Crash", func(r *observe.Run, _ *observe.Controls) { r.End = observe.End{Signal: 11} }, "input_missing"},
		{"Atime", func(r *observe.Run, _ *observe.Controls) { r.Details.Planted.AtimeMoved = false }, "input_missing"},
		{"Identity", func(r *observe.Run, _ *observe.Controls) { r.Details.Planted.SameFile = false }, "input_missing"},
		{"ControlExit", func(r *observe.Run, _ *observe.Controls) { r.Details.Planted.ControlEnd.End = observe.End{} }, "harness_fault"},
		{"ControlBeforeMarker", func(r *observe.Run, _ *observe.Controls) { r.Details.Planted.ControlEnd.Markers = observe.Markers{} }, "harness_fault"},
		{"ControlMemory", func(r *observe.Run, _ *observe.Controls) { r.Details.Planted.ControlEnd.End.MemoryKilled = true }, "harness_fault"},
		{"ControlSigning", func(r *observe.Run, _ *observe.Controls) { r.Details.Planted.ControlEnd.End.CodeSigningKilled = true }, "harness_fault"},
		{"ControlEngine", func(r *observe.Run, _ *observe.Controls) { r.Details.Planted.ControlEnd.End.EngineKilled = true }, "harness_fault"},
		{"Calibration", func(r *observe.Run, c *observe.Controls) {
			c.ReadCalibrationMoved = false
			r.Details.Planted.ControlEnd.End = observe.End{}
		}, "input_missing"},
	} {
		t.Run(row.name, func(t *testing.T) {
			r := clean
			p := *clean.Details.Planted
			end := *p.ControlEnd
			p.ControlEnd = &end
			r.Details.Planted = &p
			c := passing()
			c.ReadCalibrationMoved = true
			row.change(&r, &c)
			got := plantedSecret(ch, c, Evidence{Invocations: []observe.Invocation{{Key: observe.Key{"words": plan.Array{"read"}}, Run: r}}})
			outcome := NotObserved
			if row.reason == "" {
				outcome = "held"
			}
			want := Result{Level: 8, Outcome: outcome, Reason: row.reason}
			if got != want {
				t.Fatalf("got %+v want %+v", got, want)
			}
		})
	}
}

func TestPlantedPrecedence(t *testing.T) {
	// QO7, QP11, QP12, QP13: test every pair in both listing orders, including E-35 and E-36.
	ch := plan.Check{Preimage: plan.Object{"probe": "planted_secret"}, Keys: []plan.Object{{}}}
	c := passing()
	c.ReadCalibrationMoved = true
	c.KnownFastWithin = true
	normal := func() observe.Run { return observe.Run{Markers: observe.Markers{Confinement: true}} }
	type step struct {
		name string
		run  observe.Run
		want Result
	}
	unconfined := normal()
	unconfined.Markers.Confinement = false
	leak := normal()
	leak.Details.LeakScan.Leaks = []observe.Leak{{Place: "stdout"}}
	engine := normal()
	engine.End = observe.End{Signal: 9, EngineKilled: true}
	control := normal()
	control.Details.Planted = &observe.Planted{AtimeMoved: true, SameFile: true, ControlEnd: &observe.InvocationProtocol{Markers: observe.Markers{Confinement: true}}}
	start := normal()
	start.Markers.CouldNotStart = true
	bound := normal()
	bound.Details.LeakScan.BoundReached = true
	read := normal()
	read.Details.Planted = &observe.Planted{AtimeMoved: true, SameFile: true, ControlEnd: &observe.InvocationProtocol{End: observe.End{Signal: 9}, Markers: observe.Markers{Confinement: true}}}
	steps := []step{
		{"Unconfined", unconfined, Result{Level: 1, Outcome: NotObserved, Reason: NetworkUnverifiable}},
		{"Leak", leak, Result{Level: 8, Outcome: "not_held", FailureKind: "predicate"}},
		{"Engine", engine, Result{Level: 7, Outcome: NotObserved, Reason: "harness_fault"}},
		{"Control", control, Result{Level: 8, Outcome: NotObserved, Reason: "harness_fault"}},
		{"Start", start, Result{Level: 7, Outcome: NotObserved, Reason: "could_not_start"}},
		{"Bound", bound, Result{Level: 8, Outcome: NotObserved, Reason: "harness_fault"}},
		{"Read", read, Result{Level: 8, Outcome: "held"}},
		{"Missing", normal(), Result{Level: 8, Outcome: NotObserved, Reason: "input_missing"}},
	}
	for i, a := range steps {
		for _, b := range steps[i+1:] {
			for _, reverse := range []bool{false, true} {
				name := a.name + "_" + b.name
				if reverse {
					name += "_Reverse"
				}
				t.Run(name, func(t *testing.T) {
					invs := []observe.Invocation{{Run: a.run}, {Run: b.run}}
					if reverse {
						invs[0], invs[1] = invs[1], invs[0]
					}
					e := Evidence{Invocations: invs}
					got := level1(ch, c, e)
					if got.Level == 0 {
						got = level7(ch, c, e)
					}
					if got.Level == 0 {
						got = level8(ch, c, e)
					}
					if got != a.want {
						t.Fatalf("got %+v want %+v", got, a.want)
					}
				})
			}
		}
	}
	// QO7's QO2-QO4 step uses listing order, rather than severity.
	killed := normal()
	killed.End = observe.End{Signal: 9}
	budget := normal()
	budget.End = observe.End{Signal: 9, EngineKilled: true, BudgetExpired: true, RunningAtBudget: true}
	for _, runs := range [][]observe.Run{{start, killed, budget}, {killed, budget, start}, {budget, start, killed}} {
		invs := []observe.Invocation{}
		for _, r := range runs {
			invs = append(invs, observe.Invocation{Run: r})
		}
		got := plantedSecret(ch, c, Evidence{Invocations: invs})
		want := DiscoveryInvocation(ch, c, runs[0])
		if got != want {
			t.Fatalf("listing order got %+v want %+v", got, want)
		}
	}
}

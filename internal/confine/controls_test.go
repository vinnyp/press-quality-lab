package confine_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/vinnyp/press-quality-lab/internal/confine"
	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
)

func fastFixture() []confine.ControlRun {
	runs := make([]confine.ControlRun, 5)
	for i := range runs {
		runs[i] = confine.ControlRun{Run: observe.Run{Markers: observe.Markers{Confinement: true}}, Elapsed: time.Millisecond}
	}
	return runs
}

func TestKnownFastWithin(t *testing.T) {
	// QH24: exactly five marker-only successful runs; the slowest is <= 2 s.
	for _, row := range []struct {
		name  string
		count int
		slow  time.Duration
		want  bool
	}{
		{"Within", 5, time.Second, true},
		{"Boundary", 5, 2 * time.Second, true},
		{"Over", 5, 2*time.Second + time.Nanosecond, false},
		{"Negative", 5, -time.Nanosecond, false},
		{"Missing", 4, time.Second, false},
		{"Extra", 6, time.Second, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			runs := fastFixture()
			if row.count == 6 {
				runs = append(runs, runs[0])
			} else {
				runs = runs[:row.count]
			}
			// Put the slow run first: the last run and the mean are no oracle.
			runs[0].Elapsed = row.slow
			if got := confine.KnownFastWithin(runs); got != row.want {
				t.Fatalf("got %v want %v", got, row.want)
			}
		})
	}
	for _, row := range []struct {
		name     string
		breakRun func(*observe.Run)
	}{
		{"Marker", func(r *observe.Run) { r.Markers.Confinement = false }},
		{"StartFailure", func(r *observe.Run) { r.Markers.CouldNotStart = true }},
		{"Exit", func(r *observe.Run) { r.End.Code = 1 }},
		{"Signal", func(r *observe.Run) { r.End.Signal = 9 }},
		{"EngineKill", func(r *observe.Run) { r.End.EngineKilled = true }},
		{"Budget", func(r *observe.Run) { r.End.BudgetExpired = true }},
		{"Copy", func(r *observe.Run) { r.CopyChanged = true }},
		{"Output", func(r *observe.Run) { r.Stdout = []byte("unexpected") }},
		{"Stderr", func(r *observe.Run) { r.Stderr = []byte("unexpected") }},
		{"StderrCut", func(r *observe.Run) { r.StderrCut = true }},
		{"Cut", func(r *observe.Run) { r.StdoutCut = true }},
		{"Drain", func(r *observe.Run) { r.OutputCutAfterExit = true }},
	} {
		t.Run(row.name, func(t *testing.T) {
			runs := fastFixture()
			if !confine.KnownFastWithin(runs) {
				t.Fatal("positive control")
			}
			row.breakRun(&runs[2].Run)
			if confine.KnownFastWithin(runs) {
				t.Fatal("invalid run passed")
			}
		})
	}
}

func TestCalibrationMoved(t *testing.T) {
	// QH26: a read must succeed and advance valid access times, including nanos.
	if confine.CalibrationMoved(time.Time{}, time.Unix(100, 0), true) {
		t.Fatal("zero before accepted")
	}
	before := time.Unix(100, 123)
	for _, row := range []struct {
		name         string
		after        time.Time
		readOK, want bool
	}{
		{"Moved", before.Add(time.Nanosecond), true, true},
		{"Unchanged", before, true, false},
		{"Backwards", before.Add(-time.Nanosecond), true, false},
		{"ZeroAfter", time.Time{}, true, false},
		{"NoRead", before.Add(time.Hour), false, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			if got := confine.CalibrationMoved(before, row.after, row.readOK); got != row.want {
				t.Fatalf("got %v want %v", got, row.want)
			}
		})
	}
}

func TestCalibrationReadPassed(t *testing.T) {
	// QH26: metadata movement cannot substitute for a successful decoy read.
	r := observe.Run{Markers: observe.Markers{Confinement: true}, Stdout: []byte("calibration_value\n")}
	if !confine.CalibrationReadPassed(r) {
		t.Fatal("positive read control failed")
	}
	for _, row := range []struct {
		name   string
		change func(*observe.Run)
	}{
		{"WrongValue", func(r *observe.Run) { r.Stdout = []byte("wrong_value\n") }},
		{"NoMarker", func(r *observe.Run) { r.Markers.Confinement = false }},
		{"Exit", func(r *observe.Run) { r.End.Code = 1 }},
		{"Killed", func(r *observe.Run) { r.End.Signal = 9 }},
		{"ErrorOutput", func(r *observe.Run) { r.Stderr = []byte("failed") }},
	} {
		t.Run(row.name, func(t *testing.T) {
			bad := r
			row.change(&bad)
			if confine.CalibrationReadPassed(bad) {
				t.Fatal("unproved read passed")
			}
		})
	}
}

func TestControlFactsApply(t *testing.T) {
	// QH9, QH24, QH26: the Darwin harness's raw result uses the untagged
	// derivations, including refusal of an unproved calibration read.
	f := confine.ControlFacts{Subject: selfTestFixture(false), Computation: selfTestFixture(true), Fast: fastFixture(), Before: time.Unix(1, 0), After: time.Unix(2, 0), CalibrationRead: observe.Run{Markers: observe.Markers{Confinement: true}, Stdout: []byte("calibration_value\n")}}
	want := observe.Controls{SelfTestPassed: true, KnownFastWithin: true, ReadCalibrationMoved: true}
	if got := f.Apply(observe.Controls{}); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
	f.CalibrationRead.Stdout = []byte("wrong_value\n")
	want.ReadCalibrationMoved = false
	if got := f.Apply(observe.Controls{}); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func selfTestFixture(computation bool) []observe.Run {
	// Literal expectations from QH9, independent of SelfTestPrograms.
	names := []string{"tcp", "unix", "udp", "bind", "listen", "read", "map", "write", "create", "service", "sysctl", "arguments", "environment", "process", "signal", "codesign", "priority", "resource"}
	if computation {
		names = append(names, "setsid", "setpgid")
	} else {
		names = append(names, "child")
	}
	var runs []observe.Run
	for _, name := range names {
		runs = append(runs, observe.Run{Markers: observe.Markers{Confinement: true}, End: observe.End{Signal: 9}, Stdout: []byte("attempt_" + name + "\n")})
	}
	if computation {
		runs = append(runs, observe.Run{Markers: observe.Markers{Confinement: true}, Stdout: []byte("attempt_child\nchild_ok\n")})
	}
	return append(runs, observe.Run{Markers: observe.Markers{Confinement: true}, Stdout: []byte("clean_value\n")})
}

func TestSelfTestResults(t *testing.T) {
	// QH9: every named program under each profile must prove its operation.
	subject, computation := selfTestFixture(false), selfTestFixture(true)
	if !confine.SelfTestPassed(subject, computation) {
		t.Fatal("literal positive control failed")
	}
	for profile, runs := range [][]observe.Run{subject, computation} {
		for i := range runs {
			changed := append([]observe.Run(nil), runs...)
			changed[i].Stdout = []byte("wrong_output\n")
			s, c := subject, computation
			if profile == 0 {
				s = changed
			} else {
				c = changed
			}
			if confine.SelfTestPassed(s, c) {
				t.Fatalf("profile %d program %d wrong output passed", profile, i)
			}
		}
	}
	if confine.SelfTestPassed(subject[:len(subject)-1], computation) || confine.SelfTestPassed(subject, append(computation, computation[0])) {
		t.Fatal("incomplete or extra witness passed")
	}
	for _, row := range []struct {
		name   string
		change func(*observe.Run)
	}{
		{"Stderr", func(r *observe.Run) { r.Stderr = []byte("error") }},
		{"CouldNotStart", func(r *observe.Run) { r.Markers.CouldNotStart = true }},
		{"CopyChanged", func(r *observe.Run) { r.CopyChanged = true }},
		{"OutputCutAfterExit", func(r *observe.Run) { r.OutputCutAfterExit = true }},
		{"StdoutCut", func(r *observe.Run) { r.StdoutCut = true }},
		{"StderrCut", func(r *observe.Run) { r.StderrCut = true }},
		{"NoAttempt", func(r *observe.Run) { r.Stdout = nil }},
		{"Survived", func(r *observe.Run) { r.End.Signal = 0 }},
		{"Budget", func(r *observe.Run) { r.End.BudgetExpired = true }},
		{"Engine", func(r *observe.Run) { r.End.EngineKilled = true }},
		{"Memory", func(r *observe.Run) { r.End.MemoryKilled = true }},
		{"Codesigning", func(r *observe.Run) { r.End.CodeSigningKilled = true }},
		{"Detail", func(r *observe.Run) { r.End.ExitDetail = 0x10000 }},
		{"NoMarker", func(r *observe.Run) { r.Markers.Confinement = false }},
	} {
		t.Run(row.name, func(t *testing.T) {
			changed := append([]observe.Run(nil), subject...)
			row.change(&changed[0])
			if confine.SelfTestPassed(changed, computation) {
				t.Fatal("invalid kill passed")
			}
		})
	}
	for _, row := range []struct {
		name   string
		change func(*observe.Run)
	}{
		{"CleanStderr", func(r *observe.Run) { r.Stderr = []byte("error") }},
		{"CleanStderrCut", func(r *observe.Run) { r.StderrCut = true }},
	} {
		t.Run(row.name, func(t *testing.T) {
			changed := append([]observe.Run(nil), subject...)
			row.change(&changed[len(changed)-1])
			if confine.SelfTestPassed(changed, computation) {
				t.Fatal("invalid clean run passed")
			}
		})
	}
}

func TestControlResultsMapping(t *testing.T) {
	// QH9, QH24, QH26: map facts onto Controls without losing unrelated facts.
	before := time.Unix(100, 0)
	base := observe.Controls{SystemNamed: true, ConfinementApplied: true, PrivateCopyUnchanged: true, StoreControlDetected: true}
	want := base
	want.SelfTestPassed = true
	want.KnownFastWithin = true
	want.ReadCalibrationMoved = true
	got := confine.ApplyControlResults(base, selfTestFixture(false), selfTestFixture(true), fastFixture(), before, before.Add(time.Hour), true)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
	got = confine.ApplyControlResults(got, nil, selfTestFixture(true), nil, before, before, true)
	if !reflect.DeepEqual(got, base) {
		t.Fatalf("failure mapping got %+v want %+v", got, base)
	}
	// QO1, erratum E-15: failed self-test reaches the existing level-1 path,
	// including checks whose preimage contains no command.
	for _, probe := range []string{"store_unchanged", "byte_bound", "planted_secret"} {
		r := decide.Resolve(plan.Check{Preimage: plan.Object{"probe": probe}, Required: plan.Object{}}, got, decide.Evidence{})
		if r != (decide.Result{Outcome: "not_observed", Reason: "network_unverifiable", Level: 1}) {
			t.Fatalf("%s: %+v", probe, r)
		}
	}
}

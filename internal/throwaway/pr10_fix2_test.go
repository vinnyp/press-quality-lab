package throwaway

import (
	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"os"
	"testing"
	"time"
)

func TestFix2IncompleteComparison(t *testing.T) {
	// QP6, E-46, E-47: absent coverage is not a created or removed file.
	home := t.TempDir()
	putHome(t, home, "data", "clean")
	putHome(t, home, "other", "clean")
	before, err := SnapshotHome(home)
	if err != nil || before.Unreadable {
		t.Fatalf("control %+v %v", before, err)
	}
	for _, row := range []string{"Bound", "RootChmod"} {
		t.Run(row, func(t *testing.T) {
			h := defaultHomeHooks(openHomeFile)
			if row == "Bound" {
				h.entryLimit = 1
			} else {
				h.chmod = func(*os.Root, string, os.FileMode) error { return os.ErrPermission }
			}
			after, err := snapshotHomeWith(home, h)
			if err != nil {
				t.Fatal(err)
			}
			changes, fault := CompareHomes(before, after)
			assertFix2StoreDecision(t, changes, fault)
			if len(changes) != 0 || !fault {
				t.Fatalf("changes %+v fault %v", changes, fault)
			}
			changes, fault = CompareHomes(after, before)
			if len(changes) != 0 || !fault {
				t.Fatalf("reverse %+v fault %v", changes, fault)
			}
		})
	}
}

func TestFix2DirectoryOpen(t *testing.T) {
	// E-46, E-47, QH25: failure opening the root directory cannot invent removals.
	home := t.TempDir()
	putHome(t, home, "data", "clean")
	before, err := SnapshotHome(home)
	if err != nil || before.Unreadable {
		t.Fatalf("control %+v %v", before, err)
	}
	h := defaultHomeHooks(openHomeFile)
	h.openDirectory = func(*os.Root) (*os.File, error) { return nil, os.ErrPermission }
	after, err := snapshotHomeWith(home, h)
	if err != nil {
		t.Fatal(err)
	}
	changes, fault := CompareHomes(before, after)
	assertFix2StoreDecision(t, changes, fault)
	if len(changes) != 0 || !fault {
		t.Fatalf("changes %+v fault %v", changes, fault)
	}
	scan, err := scanHomeWith(home, "", []ScanForm{{"plain", "abcdefgh"}}, nil, nil, false, h, 100, 100)
	if err != nil || !scan.Unreadable || scan.BoundReached || len(scan.Leaks) != 0 {
		t.Fatalf("scan %+v %v", scan, err)
	}
}
func TestFix2CoveredChange(t *testing.T) {
	// QP6, E-47: an inspected change survives a directory gap beside it.
	home := t.TempDir()
	putHome(t, home, "a", "before")
	putHome(t, home, "z/data", "clean")
	before, err := SnapshotHome(home)
	if err != nil || before.Unreadable {
		t.Fatalf("control %+v %v", before, err)
	}
	putHome(t, home, "a", "after")
	h := defaultHomeHooks(openHomeFile)
	opens := 0
	h.openDirectory = func(r *os.Root) (*os.File, error) {
		opens++
		if opens == 2 {
			return nil, os.ErrPermission
		}
		return r.Open(".")
	}
	after, err := snapshotHomeWith(home, h)
	if err != nil {
		t.Fatal(err)
	}
	changes, fault := CompareHomes(before, after)
	assertFix2StoreDecision(t, changes, fault)
	if !fault || len(changes) != 1 || changes[0].Path != "a" || changes[0].Change != "modified" {
		t.Fatalf("changes %+v fault %v", changes, fault)
	}
}

func TestFix2BetweenFilesDeadline(t *testing.T) {
	// QH25, E-46, E-47: files beyond the read deadline cannot acquire empty digests and appear changed.
	home := t.TempDir()
	putHome(t, home, "a", "clean")
	putHome(t, home, "b", "clean")
	before, err := SnapshotHome(home)
	if err != nil || before.Unreadable {
		t.Fatalf("control %+v %v", before, err)
	}
	h := defaultHomeHooks(openHomeFile)
	opened := false
	openCount := 0
	calls := 0
	now := time.Now()
	h.open = func(r *os.Root, n string) (homeFile, error) { opened = true; openCount++; return openHomeFile(r, n) }
	h.now = func() time.Time {
		if opened {
			calls++
			if calls >= 3 {
				return now.Add(2 * MaxWalkDuration)
			}
		}
		return now
	}
	after, err := snapshotHomeWith(home, h)
	if err != nil {
		t.Fatal(err)
	}
	changes, fault := CompareHomes(before, after)
	if openCount != 1 {
		t.Fatalf("opened %d files past read deadline", openCount)
	}
	if len(changes) != 0 || !fault || !after.Entries[1].Unreadable {
		t.Fatalf("changes %+v fault %v snapshot %+v", changes, fault, after)
	}
}

func assertFix2StoreDecision(t *testing.T, changes []HomeDelta, unreadable bool) {
	t.Helper()
	yes := true
	home := observe.HomeChanges{Unreadable: unreadable, ControlDetected: &yes}
	for _, c := range changes {
		d := c.Digest
		home.Changes = append(home.Changes, observe.HomeChange{Path: c.Path, Kind: c.Kind, Change: c.Change, Digest: &d})
	}
	run := observe.Run{Markers: observe.Markers{Confinement: true}, Details: observe.RunDetails{Home: home}}
	got := fix2Decision("store_unchanged", run)
	want := decide.Result{Level: 8, Outcome: decide.NotObserved, Reason: "harness_fault"}
	if len(changes) > 0 {
		want = decide.Result{Level: 8, Outcome: "not_held", FailureKind: "predicate"}
	}
	if got != want {
		t.Fatalf("decision %+v want %+v", got, want)
	}
}
func fix2Decision(probe string, run observe.Run) decide.Result {
	ch := plan.Check{Subject: plan.Object{}, Preimage: plan.Object{"probe": probe, "store": "data", "earlier": plan.Array{}, "expected": nil}, Keys: []plan.Object{{}}, Seed: []plan.Entry{{Path: "data", Data: []byte("clean")}}}
	controls := observe.Controls{ConfinementApplied: true, SelfTestPassed: true, SystemNamed: true, PrivateCopyUnchanged: true}
	return decide.Resolve(ch, controls, decide.Evidence{Discovery: &decide.Discovery{}, Invocations: []observe.Invocation{{Run: run}}})
}

func assertFix2LeakDecision(t *testing.T, scan ScanResult) {
	t.Helper()
	run := observe.Run{Markers: observe.Markers{Confinement: true}}
	for _, l := range scan.Leaks {
		run.Details.LeakScan.Leaks = append(run.Details.LeakScan.Leaks, observe.Leak{Place: l.Place, Encoding: l.Encoding})
	}
	if got := fix2Decision("planted_secret", run); got != (decide.Result{Level: 8, Outcome: "not_held", FailureKind: "predicate"}) {
		t.Fatalf("leak decision %+v", got)
	}
}

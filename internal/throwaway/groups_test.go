package throwaway

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSweepRecordedGroupIdentity(t *testing.T) {
	// QE20, QH23, KTD5: prove identity before signalling, and end before removal.
	for _, row := range []struct {
		name, start string
		gone, end   bool
	}{
		{"Matching", "group", false, true},
		{"Reused", "different", false, false},
		{"LeaderGone", "", true, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			parent := t.TempDir()
			if err := os.Chmod(parent, 0700); err != nil {
				t.Fatal(err)
			}
			w, err := Create(parent)
			if err != nil {
				t.Fatal(err)
			}
			if err := w.Publish(Process{PID: 41, StartTime: "engine"}); err != nil {
				t.Fatal(err)
			}
			if err := w.RecordGroup(Process{PID: 42, StartTime: "group"}); err != nil {
				t.Fatal(err)
			}
			var report bytes.Buffer
			ended := false
			lookup := func(pid int) (string, error) {
				if pid == 41 || row.gone {
					return "", os.ErrNotExist
				}
				return row.start, nil
			}
			err = Sweep(parent, lookup, SweepOptions{Report: &report, End: func(pid int) error {
				if pid != 42 {
					t.Fatalf("group %d", pid)
				}
				if _, err := os.Stat(w.Path()); err != nil {
					t.Fatal("removed before group ended", err)
				}
				ended = true
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
			if ended != row.end {
				t.Fatalf("ended=%v want %v", ended, row.end)
			}
			want := ""
			if !row.end {
				want = "cannot verify recorded process group 42; not signalled\n"
			}
			if report.String() != want {
				t.Fatalf("report %q want %q", report.String(), want)
			}
			if _, err := os.Stat(w.Path()); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("workspace remains: %v", err)
			}
		})
	}
}

func TestSweepGroupFailureRetries(t *testing.T) {
	// QE20, QH23, E-11: failure to end a verified group retains its home for retry.
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	w, err := Create(parent)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Publish(Process{PID: 41, StartTime: "engine"}); err != nil {
		t.Fatal(err)
	}
	if err := w.RecordGroup(Process{PID: 42, StartTime: "group"}); err != nil {
		t.Fatal(err)
	}
	lookup := func(pid int) (string, error) {
		if pid == 41 {
			return "", os.ErrNotExist
		}
		return "group", nil
	}
	failed := errors.New("group did not end")
	if err := Sweep(parent, lookup, SweepOptions{End: func(int) error { return failed }}); err != nil {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(filepath.Join(w.Path(), "process.json")); err != nil {
		t.Fatal(err)
	}
	if err := Sweep(parent, lookup, SweepOptions{End: func(int) error { return nil }}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(w.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("retry: %v", err)
	}
}

func TestEndRecordedAttemptsEveryGroup(t *testing.T) {
	// QH23, KTD17: one group's failure cannot leave later groups unattempted.
	groups := []Process{{PID: 41, StartTime: "one"}, {PID: 42, StartTime: "two"}}
	lookup := func(pid int) (string, error) {
		if pid == 41 {
			return "one", nil
		}
		return "two", nil
	}
	for _, failed := range []bool{false, true} {
		var got []int
		wantErr := errors.New("first group failed")
		err := endRecorded(groups, lookup, SweepOptions{End: func(pid int) error {
			got = append(got, pid)
			if failed && pid == 41 {
				return wantErr
			}
			return nil
		}})
		if len(got) != 2 || got[0] != 41 || got[1] != 42 {
			t.Fatalf("attempts %v", got)
		}
		if failed {
			if !errors.Is(err, wantErr) {
				t.Fatalf("failure lost: %v", err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
	}
}

func TestRecordAndForgetGroup(t *testing.T) {
	// KTD5, QE20: live groups are atomically persisted and removed after ending.
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	w, err := Create(parent)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Publish(Process{PID: 41, StartTime: "engine"}); err != nil {
		t.Fatal(err)
	}
	if err := w.RecordGroup(Process{PID: 42, StartTime: "group"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(w.Path(), "process.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"groups\":[{\"pid\":42,\"start_time\":\"group\"}],\"pid\":41,\"start_time\":\"engine\"}\n"
	if string(data) != want {
		t.Fatalf("group publication %s", data)
	}
	for _, row := range []struct {
		name string
		p    Process
	}{
		{"Duplicate", Process{PID: 42, StartTime: "group"}},
		{"ZeroPID", Process{PID: 0, StartTime: "group"}},
		{"MissingStart", Process{PID: 43}},
	} {
		t.Run(row.name, func(t *testing.T) {
			if err := w.RecordGroup(row.p); err == nil {
				t.Fatal("invalid group accepted")
			}
			data, err := os.ReadFile(filepath.Join(w.Path(), "process.json"))
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != want {
				t.Fatalf("publication changed on refusal: %s", data)
			}
		})
	}
	if err := w.ForgetGroup(42); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(w.Path(), "process.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{\"pid\":41,\"start_time\":\"engine\"}\n" {
		t.Fatalf("forget publication %s", data)
	}
}

func TestEndGroupsClosesRegistration(t *testing.T) {
	// KTD17, QH23: a collector cannot hand out a new live group during cleanup.
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	w, err := Create(parent)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Publish(Process{PID: 41, StartTime: "engine"}); err != nil {
		t.Fatal(err)
	}
	p := Process{PID: 42, StartTime: "group"}
	if err := w.RecordGroup(p); err != nil {
		t.Fatal("positive control", err)
	}
	if err := w.ForgetGroup(p.PID); err != nil {
		t.Fatal(err)
	}
	var report bytes.Buffer
	if err := w.EndGroups(&report); err != nil {
		t.Fatal(err)
	}
	if err := w.RecordGroup(p); err == nil {
		t.Fatal("group registered during shutdown")
	}
	data, err := os.ReadFile(filepath.Join(w.Path(), "process.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{\"pid\":41,\"start_time\":\"engine\"}\n" {
		t.Fatalf("shutdown publication %s", data)
	}
}

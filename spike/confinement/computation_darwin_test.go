//go:build confinementspike && darwin

package confinement

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func computationFixture(t *testing.T) fixture {
	t.Helper()
	f := newFixture(t)
	for _, name := range []string{"declared", "seed"} {
		writeDecoy(t, filepath.Join(f.root, name))
	}
	scratch := filepath.Join(f.root, "scratch")
	if os.Mkdir(scratch, 0700) != nil {
		t.Fatal("create scratch")
	}
	f.params = []string{"HOME=" + scratch}
	f.env = []string{"HOME=" + scratch, "TMPDIR=" + scratch, "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TZ=UTC"}
	return f
}

func TestComputationValues(t *testing.T) {
	f := computationFixture(t)
	for _, mode := range []string{"pipeline", "system-value"} {
		runScenario(t, mode, func(t *testing.T) {
			repeat(t, func() bool {
				probe := f
				want := "value=7\n"
				if mode == "pipeline" {
					probe.args = []string{"/bin/sh", "/bin/cat", "/usr/bin/wc", filepath.Join(f.root, "declared")}
					want = "5\n"
				}
				p := profile(true)
				r := probe.invoke(p, mode, false)
				if !correct(r, want) {
					t.Logf("status=%d marked=%v output=%q", r.status, r.marked, r.output)
				}
				return correct(r, want)
			})
		})
	}
}

func TestComputationFileIsolation(t *testing.T) {
	f := computationFixture(t)
	subject := filepath.Join(f.root, "subject")
	if r := run("/usr/bin/clang", []string{"-Wall", "-Wextra", "-Werror", "-o", subject, "testdata/programs/subject.c"}, f.env, ""); r.err != nil {
		t.Fatal("build synthetic subject")
	}
	used := filepath.Join(f.root, "used-home")
	if os.Mkdir(used, 0700) != nil {
		t.Fatal("create used-home decoy")
	}
	usedFile := filepath.Join(used, "target")
	writeDecoy(t, usedFile)
	for _, scenario := range []struct{ name, mode, target string }{
		{"used-home", "file-read", usedFile},
		{"seed-write", "file-write", filepath.Join(f.root, "seed")},
		{"outside-write", "file-write", usedFile},
		{"subject-read", "file-open", subject},
		{"subject-execute", "execute", subject},
	} {
		for _, form := range []string{"confined", "twin", "positive", "without-kill"} {
			runScenario(t, scenario.name+"/"+form, func(t *testing.T) {
				repeat(t, func() bool {
					probe := f
					good := filepath.Join(f.root, "declared")
					if scenario.mode == "file-write" {
						good = filepath.Join(f.root, "scratch", "target")
						writeDecoy(t, good)
					}
					if scenario.mode == "execute" {
						good = f.probe
					}
					writeTarget := scenario.mode == "file-write"
					if writeTarget {
						writeDecoy(t, scenario.target)
					}
					probe.args = []string{scenario.target, good, filepath.Join(f.root, "scratch", "unused")}
					p := profile(true)
					if form == "positive" {
						p = ""
					}
					if form == "without-kill" {
						p = strings.ReplaceAll(p, "(with send-signal SIGKILL)", "")
					}
					before := decoySnapshot(scenario.target)
					r := probe.invoke(p, scenario.mode, form == "twin")
					if form == "positive" {
						return correct(r, "performed\nfallback\n")
					}
					unchanged := before == decoySnapshot(scenario.target)
					if form == "confined" {
						return r.killed() && unchanged
					}
					if form == "without-kill" {
						return r.marked && !r.timedOut && !r.killed() && unchanged
					}
					want := "performed\nfallback\n"
					if scenario.mode == "execute" {
						want = "clean\n"
					}
					return correct(r, want) && unchanged
				})
			})
		}
	}
}

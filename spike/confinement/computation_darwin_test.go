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
	if os.Mkdir(filepath.Join(scratch, "tmp"), 0700) != nil {
		t.Fatal("create private computation temporary directory")
	}
	f.env = []string{"HOME=" + scratch, "TMPDIR=" + filepath.Join(scratch, "tmp"), "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TZ=UTC", coreFoundationEncoding}
	return f
}

func TestComputationValues(t *testing.T) {
	t.Parallel()
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
				r := probe.invokeOn(t, p, mode, false)
				if !correct(r, want) {
					t.Logf("status=%d marked=%v output=%q", r.status, r.marked, r.output)
				}
				return correct(r, want)
			})
		})
	}
}

func TestComputationFileIsolationSmoke(t *testing.T) {
	t.Log("smoke test: these kills do not isolate the operation rule")
	t.Parallel()
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
					r := probe.invokeOn(t, p, scenario.mode, form == "twin")
					if form == "positive" {
						return correct(r, "performed\nfallback\n")
					}
					unchanged := before == decoySnapshot(scenario.target)
					if form == "confined" {
						return r.killed() && unchanged
					}
					if form == "without-kill" {
						return correct(r, "fallback\n") && unchanged
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

func TestSubjectInToolDirectory(t *testing.T) {
	t.Parallel()
	f := computationFixture(t)
	tools := filepath.Join(f.root, "tools")
	copyDir := filepath.Join(f.root, "subject-image")
	if os.Mkdir(tools, 0700) != nil || os.Mkdir(copyDir, 0700) != nil {
		t.Fatal("create subject directories")
	}
	subject := filepath.Join(tools, "subject")
	copied := filepath.Join(copyDir, "image")
	if r := run("/usr/bin/clang", []string{"-Wall", "-Wextra", "-Werror", "-o", subject, "testdata/programs/subject.c"}, f.env, ""); r.err != nil {
		t.Fatal("compile subject")
	}
	data, err := os.ReadFile(subject)
	if err != nil || os.WriteFile(copied, data, 0500) != nil {
		t.Fatal("copy subject")
	}
	for _, target := range []struct{ name, path string }{{"original", subject}, {"copy", copied}} {
		for _, mode := range []string{"file-open", "execute"} {
			for _, form := range []string{"confined", "twin", "positive", "remove-denial", "remove-kill"} {
				runScenario(t, target.name+"/"+mode+"/"+form, func(t *testing.T) {
					repeat(t, func() bool {
						probe := f
						probe.params = append(append([]string{}, f.params...), "BIN_DIR="+filepath.Dir(target.path), "SUBJECT="+subject, "SUBJECT_COPY="+copied)
						good := filepath.Join(f.root, "declared")
						if mode == "execute" {
							good = f.probe
						}
						probe.args = []string{target.path, good, filepath.Join(f.home(), "unused")}
						p := profile(true)
						line := `(deny file-read-data process-exec* (literal (param "SUBJECT")) (literal (param "SUBJECT_COPY")) (with send-signal SIGKILL))`
						if form == "positive" {
							p = ""
						}
						if form == "remove-denial" {
							p = strings.Replace(p, line, "", 1)
						}
						if form == "remove-kill" {
							p = strings.Replace(p, line, strings.Replace(line, " (with send-signal SIGKILL)", "", 1), 1)
						}
						before := decoySnapshot(target.path)
						r := probe.invokeOn(t, p, mode, form == "twin")
						unchanged := before == decoySnapshot(target.path)
						if form == "confined" {
							return r.killed() && unchanged
						}
						if form == "remove-kill" {
							return correct(r, "fallback\n") && unchanged
						}
						want := "performed\nfallback\n"
						if form == "twin" && mode == "execute" {
							want = "clean\n"
						}
						return correct(r, want) && unchanged
					})
				})
			}
		}
	}
}

func TestComputationPrivilegedImage(t *testing.T) {
	f := computationFixture(t)
	entries, err := os.ReadDir("/usr/bin")
	if err != nil {
		t.Fatal("list granted tools")
	}
	target := ""
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			t.Fatal("stat granted tool")
		}
		if info.Mode().IsRegular() && info.Mode()&(os.ModeSetuid|os.ModeSetgid) != 0 {
			target = filepath.Join("/usr/bin", entry.Name())
			break
		}
	}
	if target == "" {
		t.Fatal("no privileged image available for confined test")
	}
	f.args = []string{target}
	runScenario(t, "platform-refusal", func(t *testing.T) {
		repeat(t, func() bool {
			r := f.invokeOn(t, profile(true), "setid-execute", false)
			return correct(r, "privileged image refused\n") && !r.harnessKilled && !r.status.Signaled()
		})
	})
	t.Log("open limit: a setuid or setgid image in the granted system directory returns EPERM without a kill; a third silent refusal")
}

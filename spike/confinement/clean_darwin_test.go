//go:build confinementspike && darwin

package confinement

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func buildGoClean(t *testing.T, f fixture) fixture {
	t.Helper()
	compiler, err := exec.LookPath("go")
	if err != nil {
		t.Fatal("Go toolchain missing")
	}
	f.probe = filepath.Join(f.root, "go-clean")
	buildEnv := append(append([]string{}, f.env...), "GOCACHE="+filepath.Join(f.root, "cache"), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "CGO_ENABLED=1")
	r := runBudget(compiler, []string{"build", "-o", f.probe, "testdata/programs/clean.go"}, buildEnv, "", 2*time.Minute)
	if r.err != nil {
		t.Fatal("build Go clean program")
	}
	return f
}

func TestGoCleanPrograms(t *testing.T) {
	t.Parallel()
	f := buildGoClean(t, newFixture(t))
	for _, computation := range []bool{false, true} {
		for _, mode := range []string{"help", "user", "exit"} {
			runScenario(t, profileName(computation)+"/"+mode, func(t *testing.T) {
				repeat(t, func() bool {
					r := f.invoke(profile(computation), mode, false)
					if mode == "exit" {
						return r.marked && !r.timedOut && r.status.Exited() && r.status.ExitStatus() == 3 && r.output == "exit three\n"
					}
					want := "usage: synthetic [help|user|exit]\n"
					if mode == "user" {
						want = "user ok\n"
					}
					return correct(r, want)
				})
			})
		}
	}
}

func TestDatabaseCleanPrograms(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.probe = filepath.Join(f.root, "database")
	if r := run("/usr/bin/clang", []string{"-Wall", "-Wextra", "-Werror", "-o", f.probe, "testdata/programs/database.c", "-lsqlite3"}, f.env, ""); r.err != nil {
		t.Fatal("build database clean program")
	}
	for _, computation := range []bool{false, true} {
		runScenario(t, profileName(computation), func(t *testing.T) {
			repeat(t, func() bool {
				probe := databaseFixture(t, f)
				p := profile(computation)

				rr := probe.invoke(p, "write", false)
				if !correct(rr, "written 7\n") {
					return false
				}
				return correct(probe.invoke(p, "read", false), "side-files=wal,shm\nvalue=7\n")
			})
		})
	}
}

// The database and each possible side file are explicit declared inputs.
func databaseFixture(t *testing.T, f fixture) fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal("canonical database scratch")
	}
	path := filepath.Join(root, "data")
	f.args = []string{path}
	f.params = []string{"HOME=" + root, "DECLARED=" + path}
	f.env = []string{"HOME=" + root, "TMPDIR=" + root, "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TZ=UTC"}
	return f
}

func TestOrdinaryProgramsSmoke(t *testing.T) {
	t.Log("smoke test: these kills do not isolate the operation rule")
	t.Parallel()
	f := newFixture(t)
	g := buildGoClean(t, f)
	for _, computation := range []bool{false, true} {
		for _, mode := range []string{"devices", "streams", "files", "identity", "signal", "timezone", "unlinked"} {
			runScenario(t, profileName(computation)+"/c/"+mode, func(t *testing.T) {
				repeat(t, func() bool {
					want := "ordinary ok\n"
					diagnostic := ""
					if mode == "streams" {
						want = "stream\n" + want
						diagnostic = "diagnostic\n"
					}
					r := f.invoke(profile(computation), "ordinary-"+mode, false)
					if !correct(r, want, diagnostic) {
						t.Logf("status=%d marked=%v", r.status, r.marked)
					}
					return correct(r, want, diagnostic)
				})
			})
		}
		runScenario(t, profileName(computation)+"/go", func(t *testing.T) {
			repeat(t, func() bool {
				r := g.invoke(profile(computation), "ordinary", false)
				if !correct(r, "ordinary ok\n") {
					t.Logf("status=%d marked=%v", r.status, r.marked)
				}
				return correct(r, "ordinary ok\n")
			})
		})
	}
	t.Log("open limit: /bin/sh here-documents and here-strings need the shared temporary directory")
	f = computationFixture(t)
	for _, entry := range []struct{ name, script, want string }{
		{"here-document", "exec cat <<'END'\nhello\nEND", ""},
		{"here-string", "read -r v <<< hello; test \"$v\" = hello; printf 'shell ok\\n'", "shell ok\n"},
		{"substitution", "v=$(printf hello); test \"$v\" = hello; printf 'shell ok\\n'", "shell ok\n"},
	} {
		runScenario(t, "computation/shell/"+entry.name, func(t *testing.T) {
			script := filepath.Join(f.root, "script-"+entry.name)
			if os.WriteFile(script, []byte("#!/bin/bash\nprintf 'operation-ready\\n' >&2\nset -eu\n"+entry.script+"\n"), 0700) != nil {
				t.Fatal("write ordinary shell program")
			}
			probe := f
			probe.probe = script
			repeat(t, func() bool {
				before := decoySnapshot(filepath.Join(f.home(), "state"))
				r := probe.invoke(profile(true), "unused", false)
				if entry.name != "substitution" {
					return r.killed() && before == decoySnapshot(filepath.Join(f.home(), "state"))
				}
				if !correct(r, entry.want) {
					t.Logf("status=%d marked=%v", r.status, r.marked)
				}
				return correct(r, entry.want)
			})
		})
	}
}

func TestDatabaseDeclaredInputSmoke(t *testing.T) {
	t.Log("smoke test: these kills do not isolate the operation rule")
	t.Parallel()
	f := computationFixture(t)
	f.probe = filepath.Join(f.root, "database")
	if r := run("/usr/bin/clang", []string{"-Wall", "-Wextra", "-Werror", "-o", f.probe, "testdata/programs/database.c", "-lsqlite3"}, f.env, ""); r.err != nil {
		t.Fatal("build database")
	}
	for _, form := range []string{"private-copy", "direct-seed", "twin", "positive"} {
		runScenario(t, form, func(t *testing.T) {
			repeat(t, func() bool {
				root, err := filepath.EvalSymlinks(t.TempDir())
				if err != nil {
					t.Fatal("resolve input root")
				}
				seed := filepath.Join(root, "seed")
				scratch := filepath.Join(root, "scratch")
				if os.Mkdir(scratch, 0700) != nil {
					t.Fatal("create database scratch")
				}
				setup := f
				setup.args = []string{seed}
				if !correct(setup.invoke("", "write", false), "written 7\n") {
					return false
				}
				before := decoySnapshot(seed)
				data, err := os.ReadFile(seed)
				if err != nil {
					t.Fatal("read seed copy")
				}
				copyPath := filepath.Join(scratch, "data")
				if os.WriteFile(copyPath, data, 0600) != nil {
					t.Fatal("prepare immutable input")
				}
				before = decoySnapshot(seed)
				probe := f
				probe.params = []string{"HOME=" + scratch, "SEED=" + seed, "DECLARED=" + seed}
				probe.env = []string{"HOME=" + scratch, "TMPDIR=" + scratch, "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TZ=UTC"}
				probe.args = []string{seed}
				if form == "private-copy" || form == "twin" {
					probe.args = []string{copyPath}
				}
				p := profile(true)
				if form == "positive" {
					p = ""
				}
				mode, want := "read-write", "read-write open\n"
				if form == "private-copy" {
					mode, want = "read", "side-files=wal,shm\nvalue=7\n"
				}
				r := probe.invoke(p, mode, false)
				if form == "direct-seed" {
					return r.killed() && before == decoySnapshot(seed)
				}
				return correct(r, want) && before == decoySnapshot(seed)
			})
		})
	}
}

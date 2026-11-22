//go:build confinementspike && darwin

package confinement

import (
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
	f.params = []string{"HOME=" + root, "DECLARED=" + path, "WAL=" + path + "-wal", "SHM=" + path + "-shm"}
	f.env = []string{"HOME=" + root, "TMPDIR=" + root, "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TZ=UTC"}
	return f
}

//go:build confinementspike && darwin

package confinement

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

const repetitions = 25

var executed atomic.Int64

func profileName(computation bool) string {
	if computation {
		return "computation"
	}
	return "subject"
}

func TestMain(m *testing.M) {
	code := m.Run()
	if executed.Load() == 0 {
		fmt.Fprintln(os.Stderr, "no confinement scenario executed")
		code = 1
	}
	os.Exit(code)
}

type fixture struct {
	root, launcher, probe string
	volume                string
	env                   []string
	args                  []string
	params                []string
	budget                time.Duration
	profileFile           string
	nested                bool
	launcherFiles         []*os.File
	firstLine             func(string) error
	prepared              bool
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal("resolve temporary root")
	}
	var fs syscall.Statfs_t
	if err := syscall.Statfs(root, &fs); err != nil {
		t.Fatal("inspect temporary filesystem")
	}
	var mount []byte
	for _, v := range fs.Mntonname {
		if v == 0 {
			break
		}
		mount = append(mount, byte(v))
	}
	f := fixture{volume: string(mount), root: root, launcher: filepath.Join(root, "launcher"), probe: filepath.Join(root, "probe")}
	home := filepath.Join(root, "home")
	if os.Mkdir(home, 0700) != nil {
		t.Fatal("create redirected home")
	}
	for _, name := range []string{"declared", "seed"} {
		writeDecoy(t, filepath.Join(root, name))
	}
	writeDecoy(t, filepath.Join(home, "state"))
	if os.Mkdir(filepath.Join(home, "tmp"), 0700) != nil {
		t.Fatal("create private temporary directory")
	}
	f.env = []string{"HOME=" + home, "TMPDIR=" + filepath.Join(home, "tmp"), "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TZ=UTC"}
	for _, build := range []struct{ source, binary string }{{"launcher.c", f.launcher}, {"probe.c", f.probe}} {
		r := run("/usr/bin/clang", []string{"-Wall", "-Wextra", "-Werror", "-o", build.binary, filepath.Join("testdata/programs", build.source)}, f.env, "")
		if r.err != nil {
			t.Fatal("compile fixture failed")
		}
	}
	return f
}

type imageCacheKey struct {
	root, probe, launcher   string
	probeHash, launcherHash [sha256.Size]byte
}

type cachedImages struct {
	root string
	info [2]os.FileInfo
}

var imageCache = struct {
	sync.Mutex
	sets map[imageCacheKey]cachedImages
}{sets: make(map[imageCacheKey]cachedImages)}

func exactImageEntries(path string, names ...string) bool {
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) != len(names) {
		return false
	}
	for i, entry := range entries {
		if entry.Name() != names[i] {
			return false
		}
	}
	return true
}

func (images cachedImages) intact(hashes [2][sha256.Size]byte) bool {
	if !exactImageEntries(images.root, "launcher", "program") {
		return false
	}
	for i, name := range []string{"program", "launcher"} {
		dir := filepath.Join(images.root, name)
		dirInfo, err := os.Lstat(dir)
		if err != nil || !dirInfo.IsDir() || !exactImageEntries(dir, "image") {
			return false
		}
		path := filepath.Join(dir, "image")
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode() != 0500 ||
			!os.SameFile(info, images.info[i]) || info.Size() != images.info[i].Size() ||
			!info.ModTime().Equal(images.info[i].ModTime()) {
			return false
		}
		data, err := os.ReadFile(path)
		if err != nil || sha256.Sum256(data) != hashes[i] {
			return false
		}
	}
	return true
}

func (f fixture) prepareImages() (fixture, func(), error) {
	if f.prepared {
		return f, func() {}, nil
	}
	probe, err := filepath.Abs(f.probe)
	if err != nil {
		return f, nil, err
	}
	launcher, err := filepath.Abs(f.launcher)
	if err != nil {
		return f, nil, err
	}
	imageCache.Lock()
	defer imageCache.Unlock()
	var data [2][]byte
	var hashes [2][sha256.Size]byte
	for i, path := range []string{probe, launcher} {
		data[i], err = os.ReadFile(path)
		if err != nil {
			return f, nil, err
		}
		hashes[i] = sha256.Sum256(data[i])
	}
	key := imageCacheKey{f.root, probe, launcher, hashes[0], hashes[1]}
	images, found := imageCache.sets[key]
	if found && !images.intact(hashes) {
		if err := os.RemoveAll(images.root); err != nil {
			return f, nil, err
		}
		delete(imageCache.sets, key)
		found = false
	}
	if !found {
		root, err := os.MkdirTemp(f.root, "images-")
		if err != nil {
			return f, nil, err
		}
		images = cachedImages{root: root}
		fail := func(err error) (fixture, func(), error) {
			_ = os.RemoveAll(root)
			return f, nil, err
		}
		for i, name := range []string{"program", "launcher"} {
			dir := filepath.Join(root, name)
			if err := os.Mkdir(dir, 0700); err != nil {
				return fail(err)
			}
			path := filepath.Join(dir, "image")
			if err := os.WriteFile(path, data[i], 0500); err != nil {
				return fail(err)
			}
			images.info[i], err = os.Lstat(path)
			if err != nil {
				return fail(err)
			}
		}
		if !images.intact(hashes) {
			return fail(fmt.Errorf("executable copy differs"))
		}
		imageCache.sets[key] = images
	}
	original := f.probe
	f.probe = filepath.Join(images.root, "program", "image")
	f.launcher = filepath.Join(images.root, "launcher", "image")
	f.args = append([]string{}, f.args...)
	for index, arg := range f.args {
		if arg == original {
			f.args[index] = f.probe
		}
	}
	f.params = append(append([]string{}, f.params...), "BIN_PARENT="+filepath.Dir(f.probe), "LAUNCHER_PARENT="+filepath.Dir(f.launcher))
	f.prepared = true
	return f, func() {}, nil
}

func (f fixture) invoke(text, mode string, twin bool) result {
	if !f.prepared {
		prepared, release, err := f.prepareImages()
		if err != nil {
			return result{err: err, startFailed: true}
		}
		defer release()
		f = prepared
	}
	args := []string{"-p", text}
	if f.profileFile != "" {
		args = []string{"-f", f.profileFile}
	}
	parameters := f.parameters()
	for _, parameter := range parameters {
		args = append(args, "-D", parameter)
	}
	args = append(args, f.launcher, f.probe, mode)
	if text == "" {
		args = []string{f.probe, mode}
	}
	args = append(args, f.args...)
	if twin {
		args = append(args, "twin")
	}
	executed.Add(1)
	budget := f.budget
	if budget == 0 {
		budget = 5 * time.Second
	}
	if text == "" {
		return runBudgetWithFiles(f.launcher, args, f.env, f.home(), budget, f.launcherFiles, f.firstLine)
	}
	if f.nested {
		args = append([]string{"-p", "(version 1)(allow default)", "/usr/bin/sandbox-exec"}, args...)
	}
	return runBudgetWithFiles("/usr/bin/sandbox-exec", args, f.env, f.home(), budget, f.launcherFiles, f.firstLine)
}

func (f fixture) home() string {
	for _, p := range f.parameters() {
		if strings.HasPrefix(p, "HOME=") {
			return strings.TrimPrefix(p, "HOME=")
		}
	}
	panic("missing home parameter")
}

func (f fixture) parameters() []string {
	zone, _ := filepath.EvalSymlinks("/usr/share/zoneinfo/America/New_York")
	zoneRules, _ := filepath.EvalSymlinks("/usr/share/zoneinfo/posixrules")
	parameters := []string{"ZONE_RULES=" + zoneRules, "ZONEFILE=" + zone, "BIN=" + f.probe, "BIN_PARENT=" + filepath.Dir(f.probe), "LAUNCHER_PARENT=" + filepath.Dir(f.launcher), "LAUNCHER=" + f.launcher, "PLANTED=" + filepath.Join(f.root, "home", "planted"), "SUBJECT=" + filepath.Join(f.root, "subject"), "SUBJECT_COPY=" + filepath.Join(f.root, "subject-copy"), "ZERO_DEVICE=/dev/zero", "STDOUT_DEVICE=/dev/fd/1", "STDERR_DEVICE=/dev/fd/2", "VOLUME=" + f.volume, "HOME=" + filepath.Join(f.root, "home"), "ROOT=/", "SYSTEM=/System/Library", "SYSTEM_ROOT=/System", "RUNTIME=/System/Volumes/Preboot/Cryptexes", "TRACE_DEVICE=/dev/dtracehelper", "NULL_DEVICE=/dev/null", "RANDOM_DEVICE=/dev/urandom", "SHELL=/bin/sh", "SHELL_IMPL=/bin/bash", "BIN_DIR=/bin", "USR_BIN_DIR=/usr/bin", "TTY_DEVICE=/dev/tty", "CAT=/bin/cat", "FILTER=/usr/bin/wc", "DECLARED=" + filepath.Join(f.root, "declared"), "SEED=" + filepath.Join(f.root, "seed"), "WAL=" + filepath.Join(f.root, "wal"), "SHM=" + filepath.Join(f.root, "shm")}
	for _, parameter := range f.params {
		key := strings.SplitN(parameter, "=", 2)[0] + "="
		replaced := false
		for i, old := range parameters {
			if strings.HasPrefix(old, key) {
				parameters[i] = parameter
				replaced = true
				break
			}
		}
		if !replaced {
			parameters = append(parameters, parameter)
		}
	}
	return parameters
}

func runScenario(t *testing.T, name string, body func(*testing.T)) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		defer func() {
			if t.Skipped() {
				t.Error("a confinement scenario was skipped")
			}
		}()
		body(t)
	})
}

func repeat(t *testing.T, check func() bool) {
	t.Helper()
	count, correct := 0, 0
	for range repetitions {
		if check() {
			correct++
		}
		count++
	}
	t.Logf("executed=%d correct=%d expected=%d", count, correct, repetitions)
	if correct != repetitions {
		t.Errorf("scenario failed in %d of %d runs", repetitions-correct, repetitions)
	}
}

func correct(r result, output string, diagnostic ...string) bool {
	wantDiagnostic := "confined\n"
	if len(diagnostic) == 1 {
		wantDiagnostic += diagnostic[0]
	}
	return len(diagnostic) <= 1 && !r.startFailed && r.err == nil && r.marked && !r.timedOut && r.status.Exited() && r.status.ExitStatus() == 0 && r.output == output && r.diagnostic == wantDiagnostic
}

// Only the raw wait status of the calling child is a kill witness. The
// supervising program must finish normally and print no child program output.
func childKilled(r result) bool {
	const prefix = "child-status="
	if !strings.HasPrefix(r.output, prefix) || !strings.HasSuffix(r.output, "\n") {
		return false
	}
	raw, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(r.output, prefix), "\n"))
	status := syscall.WaitStatus(raw)
	return r.ready && err == nil && status.Signaled() && status.Signal() == syscall.SIGKILL && correct(r, fmt.Sprintf("%s%d\n", prefix, raw))
}

func TestProcessIsolationSmoke(t *testing.T) {
	t.Log("smoke test: these kills do not isolate the operation rule")
	t.Parallel()
	for _, computation := range []bool{false, true} {
		name := "subject"
		if computation {
			name = "computation"
		}
		for _, mode := range []string{"clean", "child", "session", "group"} {
			if !computation && (mode == "session" || mode == "group") {
				continue
			}
			runScenario(t, name+"/"+mode, func(t *testing.T) {
				t.Parallel()
				f := newFixture(t)
				repeat(t, func() bool {
					r := f.invoke(profile(computation), mode, false)
					if mode == "clean" {
						return correct(r, "clean\n")
					}
					if computation && mode == "child" {
						return correct(r, "performed\nfallback\n")
					}
					if computation && (mode == "session" || mode == "group") {
						return childKilled(r)
					}
					return r.killed()
				})
			})
		}
	}
}

func TestPositiveControlsAndTwins(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"child", "session", "group"} {
		runScenario(t, "unconfined/"+mode, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			repeat(t, func() bool { return correct(f.invoke("", mode, false), "performed\nfallback\n") })
		})
		// Escapes have permitted twins only in a computation, where fork is allowed.
		computation := mode != "child"
		runScenario(t, "twin/"+mode, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			want := "performed\nfallback\n"
			if mode == "child" {
				want = "fallback\n"
			}
			repeat(t, func() bool { return correct(f.invoke(profile(computation), mode, true), want) })
		})
	}
}

func TestAllowanceWitnesses(t *testing.T) {
	t.Parallel()
	for _, computation := range []bool{false, true} {
		for index, allowance := range allowanceRemovalTargets(profile(computation)) {
			line := allowance.witness
			if !strings.HasPrefix(line, "(allow ") {
				continue
			}
			runScenario(t, fmt.Sprintf("%s/%02d-%s", profileName(computation), index, strings.Fields(line)[1]), func(t *testing.T) {
				t.Parallel()
				f := newFixture(t)
				goFixture := f
				// Compile only the clean program selected by this allowance.
				// Each scenario still owns its binary and build cache.
				if strings.Contains(line, "ZONEFILE") || strings.Contains(line, "sysctl-read") && !strings.Contains(line, "kern.hostname") && !strings.Contains(line, "kern.ngroups") && !strings.Contains(line, `(sysctl-name "hw.memsize")`) && !strings.Contains(line, `(sysctl-name "hw.pagesize")`) {
					goFixture = buildGoClean(t, f)
				}
				sortFixture := f
				sortFixture.probe = filepath.Join(f.root, "sort-clean")
				if os.WriteFile(sortFixture.probe, []byte("#!/bin/sh\nset -eu\nprintf 'b\\na\\n' | sort\n"), 0700) != nil {
					t.Fatal("write sort allowance witness")
				}
				database := f
				database.probe = filepath.Join(f.root, "database")
				if r := run("/usr/bin/clang", []string{"-Wall", "-Wextra", "-Werror", "-o", database.probe, "testdata/programs/database.c", "-lsqlite3"}, f.env, ""); r.err != nil {
					t.Fatal("build database witness")
				}

				mode, want := "clean", "clean\n"
				wantDiagnostic := ""
				witness := f
				if strings.Contains(line, "NULL_DEVICE") || strings.Contains(line, "RANDOM_DEVICE") || strings.Contains(line, "ZERO_DEVICE") {
					mode, want = "ordinary-devices", "ordinary ok\n"
				}
				if strings.Contains(line, "STDOUT_DEVICE") || strings.Contains(line, "STDERR_DEVICE") {
					mode, want = "ordinary-streams", "stream\nordinary ok\n"
					wantDiagnostic = "diagnostic\n"
				}
				if strings.Contains(line, "ZONEFILE") {
					witness = goFixture
					mode, want = "ordinary", "ordinary ok\n"
				}
				if strings.Contains(line, "sysctl-read") {
					witness = goFixture
					mode, want = "help", "usage: synthetic [help|user|exit]\n"
				}
				if strings.Contains(line, "kern.ngroups") {
					witness = f
					mode, want = "system-value", "value=7\n"
				}
				if strings.Contains(line, `(sysctl-name "hw.memsize")`) || strings.Contains(line, `(sysctl-name "hw.pagesize")`) {
					witness = sortFixture
					mode, want = "unused", "a\nb\n"
				}
				if strings.Contains(line, "ZONE_RULES") {
					witness = f
					mode, want = "ordinary-timezone", "ordinary ok\n"
				}
				if strings.Contains(line, `param "SYSTEM"`) {
					mode, want = "user", "user ok\n"
				}
				if strings.Contains(line, `param "HOME"`) {
					if strings.Contains(line, "file-read") {
						mode, want = "file-read", "performed\nfallback\n"
						path := filepath.Join(f.root, "home", "state")
						witness.args = []string{path, path, filepath.Join(f.root, "unused")}
					}
					if strings.Contains(line, "file-write") {
						mode, want = "file-home", "home written\n"
					}
				}
				if strings.Contains(line, `allow file-read* (subpath (param "HOME"))`) {
					witness = f
					witness.args = nil
					mode, want = "ordinary-files", "ordinary ok\n"
				}
				for _, input := range []string{"DECLARED", "SEED"} {
					if strings.Contains(line, `param "`+input+`"`) {
						mode, want = "file-read", "performed\nfallback\n"
						path := filepath.Join(f.root, strings.ToLower(input))
						witness.args = []string{path, path, filepath.Join(f.root, "unused")}
					}
				}
				for _, input := range []string{"SHELL", "CAT", "FILTER", "BIN_DIR", "USR_BIN_DIR"} {
					if strings.Contains(line, `param "`+input+`"`) {
						mode, want = "pipeline", "5\n"
						witness.args = []string{"/bin/sh", "/bin/cat", "/usr/bin/wc", filepath.Join(f.root, "declared")}
					}
				}
				if strings.Contains(line, "mach-lookup") || strings.Contains(line, "ipc-posix") {
					mode, want = "user", "user ok\n"
				}
				if strings.Contains(line, "process-fork") {
					mode, want = "child", "performed\nfallback\n"
				}
				if strings.Contains(line, "process-info") {
					mode, want = "self-information", "performed\nfallback\n"
					if computation {
						mode = "sandbox-arguments"
					}
				}
				if strings.Contains(line, "allow signal") {
					mode, want = "sandbox-signal", "performed\nfallback\n"
				}

				if strings.Contains(line, "fs-info") || strings.Contains(line, "file-lock") || strings.Contains(line, "membership") || strings.Contains(line, "system-info") || strings.Contains(line, `param "WAL"`) || strings.Contains(line, `param "SHM"`) {
					witness = database
					mode, want = "write", "written 7\n"
					if strings.Contains(line, `param "WAL"`) || strings.Contains(line, `param "SHM"`) {
						mode, want = "read", "side-files=wal,shm\nvalue=7\n"
					}
				}
				if computation && strings.Contains(line, `file-read-data (literal (param "HOME"))`) {
					mode, want = "file-directory", "directory ok\n"
					witness = f
					witness.args = nil
				}
				if strings.Contains(line, `(sysctl-name "kern.hostname")`) {
					witness = f
					witness.args = nil
					mode, want = "ordinary-identity", "ordinary ok\n"
				}
				if strings.Contains(line, "TTY_DEVICE") {
					mode, want = "system-value", "value=7\n"
					witness = f
					witness.args = nil
				}
				t.Logf("allowance=%s clean-program=%s", line, mode)
				runScenario(t, "baseline", func(t *testing.T) {
					repeat(t, func() bool {
						probe := witness
						if witness.probe == database.probe {
							probe = databaseFixture(t, witness)
							if mode == "read" && !correct(probe.invoke("", "write", false), "written 7\n") {
								return false
							}
						}
						return correct(probe.invoke(profile(computation), mode, false), want, wantDiagnostic)
					})
				})
				broken := strings.Replace(profile(computation), allowance.original, allowance.replacement, 1)
				runScenario(t, "removed", func(t *testing.T) {
					repeat(t, func() bool {
						probe := witness
						if witness.probe == database.probe {
							probe = databaseFixture(t, witness)
							if mode == "read" && !correct(probe.invoke("", "write", false), "written 7\n") {
								return false
							}
						}
						r := probe.invoke(broken, mode, false)
						return !r.timedOut && !r.startFailed && !r.harnessKilled && !correct(r, want, wantDiagnostic)
					})
				})
			})
		}
	}
}

func TestDefaultRuleMutations(t *testing.T) {
	t.Parallel()
	for _, mutation := range []struct{ name, from, to string }{
		{"remove-default-denial", "(deny default (with send-signal SIGKILL))", ""},
		{"remove-default-kill", "(deny default (with send-signal SIGKILL))", "(deny default)"},
	} {
		runScenario(t, mutation.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			repeat(t, func() bool {
				// The subject child-start self-test must reject each broken profile.
				p := strings.Replace(profile(false), mutation.from, mutation.to, 1)
				r := f.invoke(p, "child", false)
				if r.timedOut || !r.marked || r.killed() {
					return false
				}
				// Without an explicit default rule this platform still refuses
				// the fork, but the missing kill leaves an ordinary exit.
				return correct(r, "fallback\n")
			})
		})
	}
}

func TestChildInputs(t *testing.T) {
	f := newFixture(t)
	var canary [32]byte
	if _, err := rand.Read(canary[:]); err != nil {
		t.Fatal("create parent canary")
	}
	t.Setenv("SPIKE_PARENT_ONLY", hex.EncodeToString(canary[:]))
	decoy, err := os.OpenFile(filepath.Join(f.root, "descriptor-decoy"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal("create descriptor decoy")
	}
	defer func() {
		if err := decoy.Close(); err != nil {
			t.Error("close descriptor decoy")
		}
	}()
	f.launcherFiles = []*os.File{decoy}
	if decoy.Fd() >= 1024 {
		t.Fatal("descriptor lies beyond fixture scan bound")
	}
	if _, _, err := syscall.Syscall(syscall.SYS_FCNTL, decoy.Fd(), syscall.F_SETFD, 0); err != 0 {
		t.Fatal("make parent descriptor inheritable")
	}
	for _, computation := range []bool{false, true} {
		name := "subject"
		if computation {
			name = "computation"
		}
		runScenario(t, name+"/environment", func(t *testing.T) {
			repeat(t, func() bool {
				return correct(f.invoke(profile(computation), "environment", false), strings.Join(f.env, "\n")+"\n")
			})
		})
		runScenario(t, name+"/descriptors", func(t *testing.T) {
			repeat(t, func() bool {
				r := f.invoke(profile(computation), "descriptors", false)
				return correct(r, "descriptors clean\n")
			})
		})
	}
	runScenario(t, "launcher-descriptor-mutation", func(t *testing.T) {
		source, err := os.ReadFile("testdata/programs/launcher.c")
		if err != nil {
			t.Fatal("read launcher fixture")
		}
		broken := strings.Replace(string(source), " | POSIX_SPAWN_CLOEXEC_DEFAULT", "", 1)
		if broken == string(source) {
			t.Fatal("descriptor mutation did not apply")
		}
		path := filepath.Join(f.root, "broken-launcher.c")
		if err := os.WriteFile(path, []byte(broken), 0600); err != nil {
			t.Fatal("write launcher mutation")
		}
		mutant := f
		mutant.launcher = filepath.Join(f.root, "broken-launcher")
		if r := run("/usr/bin/clang", []string{"-Wall", "-Wextra", "-Werror", "-o", mutant.launcher, path}, f.env, f.root); r.err != nil {
			t.Fatal("compile launcher mutation")
		}
		repeat(t, func() bool {
			r := mutant.invoke(profile(false), "descriptors", false)
			return !r.timedOut && r.marked && r.status.Exited() && r.status.ExitStatus() == 9 && r.output == ""
		})
	})
}

func TestNetwork(t *testing.T) {
	t.Parallel()
	for _, computation := range []bool{false, true} {
		for _, mode := range []string{"tcp", "udp", "http", "dns", "unix", "bind", "listen"} {
			for _, form := range []string{"confined", "twin", "positive", "attribution", "without-kill"} {
				runScenario(t, fmt.Sprintf("%s/%s/%s", profileName(computation), mode, form), func(t *testing.T) {
					t.Parallel()
					f := newFixture(t)
					repeat(t, func() bool {
						probe, untouched, closeDecoy := networkDecoy(t, f, mode)
						defer closeDecoy()
						p := profile(computation)
						if form == "positive" {
							p = ""
						}
						if form == "attribution" {
							op := "network-outbound"
							if mode == "bind" {
								op = "network-bind"
							}
							if mode == "listen" {
								op = "network-inbound"
							}
							p += "\n(allow " + op + ")\n"
						}
						if form == "without-kill" {
							p = strings.ReplaceAll(p, "(with send-signal SIGKILL)", "")
						}
						if mode == "listen" && p != "" {
							// Isolate listen from bind, which is independently tested.
							// The twin needs this loopback-only setup operation too.
							p += `
(allow network-bind (local ip "localhost:*"))
`
						}
						r := probe.invoke(p, mode, form == "twin")
						quiet := untouched()
						switch form {
						case "confined":
							return r.killed() && quiet
						case "twin":
							return correct(r, "fallback\n") && quiet
						case "positive", "attribution":
							return correct(r, "performed\nfallback\n") && (mode == "bind" || mode == "listen" || !quiet)
						default:
							return correct(r, "fallback\n") && quiet
						}
					})
				})
			}
		}
	}
}

// A relative kernel wait cannot expire before the poll is entered.
func decoyQuiet(t *testing.T, socket interface {
	SyscallConn() (syscall.RawConn, error)
}) bool {
	t.Helper()
	raw, err := socket.SyscallConn()
	if err != nil {
		t.Fatal("obtain decoy descriptor")
	}
	queue, err := syscall.Kqueue()
	if err != nil {
		t.Fatal("create decoy observer")
	}
	defer syscall.Close(queue)
	var registrationErr error
	err = raw.Control(func(fd uintptr) {
		changes := []syscall.Kevent_t{{Ident: uint64(fd), Filter: syscall.EVFILT_READ, Flags: syscall.EV_ADD | syscall.EV_ONESHOT}}
		_, registrationErr = syscall.Kevent(queue, changes, nil, nil)
	})
	if err != nil || registrationErr != nil {
		t.Fatal("register decoy observer")
	}
	events := make([]syscall.Kevent_t, 1)
	n, err := waitEvent(queue, nil, events, 20*time.Millisecond)
	if err != nil {
		t.Fatal("poll decoy")
	}
	if n > 0 && events[0].Flags&(syscall.EV_ERROR|syscall.EV_EOF) != 0 {
		t.Fatal("invalid decoy observation")
	}
	return n == 0
}

func networkDecoy(t *testing.T, f fixture, mode string) (fixture, func() bool, func()) {
	t.Helper()
	if mode == "bind" || mode == "listen" {
		// These operations have no receiver; their socket dies with the probe.
		f.args = []string{"0"}
		return f, func() bool { return true }, func() {}
	}
	if mode == "udp" || mode == "dns" {
		c, err := net.ListenPacket("udp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal("create UDP decoy")
		}
		closeDecoy := sync.OnceFunc(func() {
			if c.Close() != nil {
				t.Error("close UDP decoy")
			}
		})
		t.Cleanup(closeDecoy)
		socket, ok := c.(*net.UDPConn)
		if !ok {
			t.Fatal("unexpected UDP socket type")
		}
		f.args = []string{strconv.Itoa(socket.LocalAddr().(*net.UDPAddr).Port)}
		return f, func() bool { return decoyQuiet(t, socket) }, closeDecoy
	}
	network, address := "tcp4", "127.0.0.1:0"
	if mode == "unix" {
		root, err := os.MkdirTemp("", "socket-")
		if err != nil {
			t.Fatal("create socket scratch")
		}
		t.Cleanup(func() {
			if os.RemoveAll(root) != nil {
				t.Error("remove socket scratch")
			}
		})
		network, address = "unix", filepath.Join(root, "socket")
	}
	listener, err := net.Listen(network, address)
	if err != nil {
		t.Fatal("create listener decoy")
	}
	closeDecoy := sync.OnceFunc(func() {
		if listener.Close() != nil {
			t.Error("close listener decoy")
		}
	})
	t.Cleanup(closeDecoy)
	if mode == "unix" {
		f.args = []string{address}
	} else {
		f.args = []string{strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)}
	}
	quiet := func() bool {
		switch socket := listener.(type) {
		case *net.TCPListener:
			return decoyQuiet(t, socket)
		case *net.UnixListener:
			return decoyQuiet(t, socket)
		default:
			t.Fatal("unexpected listener type")
			return false
		}
	}
	return f, quiet, closeDecoy
}

func processPolicy(computation bool, mode, form string) string {
	p := profile(computation)
	if mode == "process-codesignature" || mode == "process-container" {
		// Isolate this information class from generic process-information
		// refusals and the default kill in both controls.
		p = strings.Replace(p, "(deny default (with send-signal SIGKILL))", "(deny default)", 1)
		p = strings.Replace(p, "(deny process-info* (with send-signal SIGKILL))", "", 1)
		p = strings.Replace(p, "(version 1)", "(version 1)\n(allow process-info*)", 1)
	}
	// The priority read is setup for the scheduling call, in both profiles.
	if mode == "priority" {
		p += "\n(allow process-info-pidinfo)\n"
	}
	if form == "attribution" {
		operation := map[string]string{
			"process-codesignature": "process-info-codesignature",
			"process-container":     "process-info-sandbox-container",
			"arguments":             "process-info-pidinfo sysctl-read",
			"process-environment":   "process-info-pidinfo sysctl-read",
			"process-pidinfo":       "process-info-pidinfo",
			"process-record":        "process-info-pidinfo sysctl-read",
			"signal":                "signal",
			"codesigning":           "process-legacy-codesigning-status-get",
			"signature":             "process-legacy-codesigning-cdhash-get",
			"priority":              "system-sched",
			"rusage":                "process-info-rusage",
		}[mode]
		p += "\n(allow " + operation + ")\n"
	}
	if form == "without-deny" && mode == "process-codesignature" {
		p = strings.Replace(p, "(deny process-info-codesignature (with send-signal SIGKILL))", "", 1)
	}
	if form == "without-kill" {
		p = strings.ReplaceAll(p, "(with send-signal SIGKILL)", "")
	}
	if form == "positive" {
		p = ""
	}
	return p
}

func TestProcessAccess(t *testing.T) {
	t.Parallel()
	for _, victimSandboxed := range []bool{false, true} {
		for _, computation := range []bool{false, true} {
			for _, mode := range []string{"process-codesignature", "arguments", "process-environment", "process-pidinfo", "signal", "codesigning", "signature", "process-record", "priority", "rusage"} {
				forms := []string{"confined", "twin", "positive", "attribution", "without-kill"}
				if mode == "process-codesignature" {
					forms = append(forms, "without-deny")
				}
				for _, form := range forms {
					runScenario(t, fmt.Sprintf("%s/sandboxed-victim=%v/%s/%s", profileName(computation), victimSandboxed, mode, form), func(t *testing.T) {
						t.Parallel()
						f := newFixture(t)
						repeat(t, func() bool {
							prepared, release, err := f.prepareImages()
							if err != nil {
								t.Fatal("prepare private images")
							}
							defer release()
							ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
							defer cancel()
							var secret [32]byte
							if _, err := rand.Read(secret[:]); err != nil {
								t.Fatal("create victim canary")
							}
							canary := hex.EncodeToString(secret[:])
							p := processPolicy(computation, mode, form)
							victim := exec.CommandContext(ctx, prepared.probe, "victim", canary)
							if victimSandboxed {
								victimPolicy := profile(true)
								if computation && p != "" {
									victimPolicy = p
								}
								args := []string{"-p", victimPolicy}
								// Both executions use exactly the same private images,
								// path bindings, home and (for computations) policy text.
								for _, parameter := range prepared.parameters() {
									args = append(args, "-D", parameter)
								}
								args = append(args, prepared.launcher, prepared.probe, "victim", canary)
								victim = exec.CommandContext(ctx, "/usr/bin/sandbox-exec", args...)
							}
							victim.Env = append(append([]string{}, prepared.env...), "DECOY_CANARY="+canary)
							victim.Dir = prepared.home()
							victim.WaitDelay = time.Second
							out, err := victim.StdoutPipe()
							if err != nil {
								t.Fatal("victim pipe")
							}
							if victim.Start() != nil {
								t.Fatal("start victim")
							}
							// Never reap this pid while a probe can still use it.
							defer func() { _ = victim.Process.Kill(); _ = victim.Wait() }()
							if line, err := bufio.NewReader(out).ReadString('\n'); err != nil || line != "ready\n" {
								t.Fatal("victim startup")
							}
							queue, err := syscall.Kqueue()
							if err != nil {
								t.Fatal("create victim observer")
							}
							defer syscall.Close(queue)
							changes := []syscall.Kevent_t{{Ident: uint64(victim.Process.Pid), Filter: syscall.EVFILT_PROC, Flags: syscall.EV_ADD | syscall.EV_ONESHOT, Fflags: syscall.NOTE_EXIT}}
							if _, err := syscall.Kevent(queue, changes, nil, nil); err != nil {
								t.Fatal("register victim observer")
							}
							probe := prepared
							probe.args = []string{strconv.Itoa(victim.Process.Pid), canary}
							probe.env = append(append([]string{}, prepared.env...), "DECOY_CANARY="+canary)
							probeMode, twin := mode, form == "twin"
							if twin && computation && mode != "priority" && mode != "process-record" && mode != "process-container" {
								probeMode = "sandbox-" + mode
								twin = false
							}
							r := probe.invoke(p, probeMode, twin)
							wantDeath := (form == "positive" || form == "attribution") && mode == "signal"
							duration := 20 * time.Millisecond
							if wantDeath {
								duration = 100 * time.Millisecond
							}
							events := make([]syscall.Kevent_t, 1)
							n, err := waitEvent(queue, nil, events, duration)
							if err != nil || n > 0 && events[0].Flags&syscall.EV_ERROR != 0 {
								t.Fatal("poll victim")
							}
							alive := n == 0
							if wantDeath {
								return correct(r, "performed\nfallback\n") && !alive
							}
							if form == "confined" {
								return r.killed() && alive
							}
							if form == "without-kill" {
								return correct(r, "fallback\n") && alive
							}
							want := "performed\nfallback\n"
							if form == "twin" && (mode == "priority" || mode == "process-record" || mode == "process-container") {
								want = "fallback\n"
							}
							return correct(r, want) && alive
						})
					})
				}
			}
		}
	}
}

func TestServices(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	for _, computation := range []bool{false, true} {
		runScenario(t, fmt.Sprintf("%s/user", profileName(computation)), func(t *testing.T) {
			repeat(t, func() bool { return correct(f.invoke(profile(computation), "user", false), "user ok\n") })
		})
		for _, service := range []string{"com.apple.cfprefsd.daemon", "com.apple.pasteboard.1", "com.apple.securityd.xpc", "com.apple.coreservices.appleevents", "com.apple.bsd.dirhelper", "com.apple.coreservices.launchservicesd"} {
			for _, form := range []string{"confined", "twin", "positive", "attribution", "without-kill"} {
				runScenario(t, fmt.Sprintf("%s/%s/%s", profileName(computation), service, form), func(t *testing.T) {
					probe := f
					probe.args = []string{service}
					repeat(t, func() bool {
						p := profile(computation)
						if form == "positive" {
							p = ""
						}
						if form == "attribution" {
							p += "\n(allow mach-lookup (global-name \"" + service + "\"))\n"
						}
						if form == "without-kill" {
							p = strings.ReplaceAll(p, "(with send-signal SIGKILL)", "")
						}
						r := probe.invoke(p, "service", form == "twin")
						if form == "confined" {
							return r.killed()
						}
						if form == "attribution" {
							p += "\n(allow mach-lookup (global-name \"" + service + "\"))\n"
						}
						if form == "without-kill" {
							return correct(r, "fallback\n")
						}
						return correct(r, "performed\nfallback\n")
					})
				})
			}
		}
	}
	t.Log("open limit: protocols of allowed services are untested; refused services are tested by lookup only")
	t.Log("open limit: global file metadata visibility remains")
}

func TestComputationOwnProcesses(t *testing.T) {
	t.Parallel()
	t.Log("priority twin reads the unchanged priority and omits setpriority: there is no permitted setpriority form")
	for _, mode := range []string{"arguments", "process-environment", "process-pidinfo", "signal", "codesigning", "rusage"} {
		runScenario(t, mode, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			repeat(t, func() bool {
				r := f.invoke(profile(true), "sandbox-"+mode, false)
				if !correct(r, "performed\nfallback\n") {
					t.Logf("own-process status=%d ready=%v timed-out=%v output-bytes=%d diagnostic-bytes=%d", r.status, r.ready, r.timedOut, len(r.output), len(r.diagnostic))
				}
				return correct(r, "performed\nfallback\n")
			})
		})
	}
}

func TestDescendantsSmoke(t *testing.T) {
	t.Log("smoke test: these kills do not isolate the operation rule")
	t.Parallel()
	for _, mode := range []string{"session", "group"} {
		runScenario(t, mode+"/without-denial", func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			p := strings.Replace(profile(true), "(deny syscall-unix (syscall-number SYS_setsid SYS_setpgid) (with send-signal SIGKILL))", "", 1)
			repeat(t, func() bool { return correct(f.invoke(p, mode, false), "performed\nfallback\n") })
		})
		runScenario(t, mode+"/without-kill", func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			p := strings.Replace(profile(true), "(deny syscall-unix (syscall-number SYS_setsid SYS_setpgid) (with send-signal SIGKILL))", "(deny syscall-unix (syscall-number SYS_setsid SYS_setpgid))", 1)
			repeat(t, func() bool { return correct(f.invoke(p, mode, false), "fallback\nfallback\n") })
		})
	}
	for _, mode := range []string{"spawn-session", "spawn-group"} {
		for _, form := range []string{"confined", "positive", "twin", "without-kill"} {
			runScenario(t, mode+"/"+form, func(t *testing.T) {
				t.Parallel()
				f := newFixture(t)
				repeat(t, func() bool {
					probe, quiet, closeDecoy := networkDecoy(t, f, "tcp")
					defer closeDecoy()
					p := profile(true)
					if form == "positive" {
						p = ""
					}
					if form == "without-kill" {
						p = strings.ReplaceAll(p, "(with send-signal SIGKILL)", "")
					}
					r := probe.invoke(p, mode, form == "twin")
					unchanged := quiet()
					if form == "confined" {
						return childKilled(r) && unchanged
					}
					if form == "without-kill" {
						return correct(r, "fallback\nchild finished\n") && unchanged
					}
					if form == "twin" {
						return correct(r, "fallback\nchild finished\n") && unchanged
					}
					return correct(r, "performed\nfallback\nchild finished\n") && !unchanged
				})
			})
		}
	}
	t.Log("open limit: POSIX_SPAWN_SETSID and POSIX_SPAWN_SETPGROUP escape group cleanup; the escaped child remains confined and has a two-second alarm")
}

func TestSubjectNetworkImagesSmoke(t *testing.T) {
	t.Log("smoke test: these kills do not isolate the operation rule")
	t.Parallel()
	for _, mode := range []string{"child-network", "replace-network"} {
		for _, form := range []string{"confined", "twin", "positive", "without-kill"} {
			runScenario(t, mode+"/"+form, func(t *testing.T) {
				t.Parallel()
				f := newFixture(t)
				repeat(t, func() bool {
					probe, quiet, closeDecoy := networkDecoy(t, f, "tcp")
					defer closeDecoy()
					p := profile(false)
					if form == "positive" {
						p = ""
					}
					if form == "without-kill" {
						p = strings.ReplaceAll(p, "(with send-signal SIGKILL)", "")
					}
					r := probe.invoke(p, mode, form == "twin")
					unchanged := quiet()
					if form == "confined" {
						return r.killed() && unchanged
					}
					if form == "without-kill" {
						return correct(r, "fallback\n") && unchanged
					}
					if form == "twin" {
						want := "fallback\n"
						return correct(r, want) && unchanged
					}
					want := "performed\nfallback\n"
					if mode == "child-network" {
						want += "fallback\n"
					}
					return correct(r, want) && !unchanged
				})
			})
		}
	}
}

func TestRemainingGroupEnded(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	for _, mode := range []string{"left-child", "timeout-child"} {
		runScenario(t, mode, func(t *testing.T) {
			repeat(t, func() bool {
				probeWithBudget := f
				probeWithBudget.budget = 250 * time.Millisecond
				r := probeWithBudget.invoke(profile(true), mode, false)
				pid, err := strconv.Atoi(strings.TrimSpace(r.output))
				if err != nil || pid <= 1 || !r.marked {
					return false
				}
				// Query only the pid printed by the child we started. Zombies have ended.
				probe := f
				probe.args = []string{strconv.Itoa(pid)}
				check := probe.invoke("", "ended", false)
				return correct(check, "ended\n") && (r.timedOut == (mode == "timeout-child"))
			})
		})
	}
}

func TestProcessInformationMutations(t *testing.T) {
	t.Parallel()
	for _, computation := range []bool{false, true} {
		for _, mutation := range []struct{ name, from, to string }{
			{"remove-denial", "(deny process-info* (with send-signal SIGKILL))", ""},
			{"remove-kill", "(deny process-info* (with send-signal SIGKILL))", "(deny process-info*)"},
		} {
			runScenario(t, fmt.Sprintf("%s/%s", profileName(computation), mutation.name), func(t *testing.T) {
				t.Parallel()
				f := newFixture(t)
				repeat(t, func() bool {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					cmd := exec.CommandContext(ctx, f.probe, "victim")
					cmd.Env = f.env
					reader, err := cmd.StdoutPipe()
					if err != nil {
						t.Fatal("victim pipe")
					}
					if err := cmd.Start(); err != nil {
						t.Fatal("start victim")
					}
					defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
					if line, err := bufio.NewReader(reader).ReadString('\n'); err != nil || line != "ready\n" {
						t.Fatal("victim startup")
					}
					probe := f
					probe.args = []string{strconv.Itoa(cmd.Process.Pid)}
					p := strings.Replace(profile(computation), mutation.from, mutation.to, 1)
					r := probe.invoke(p, "process-pidinfo", false)
					want := "fallback\n"
					if mutation.name == "remove-denial" {
						want = "performed\nfallback\n"
					}
					return correct(r, want)
				})
			})
		}
	}
}

func TestNoControllingTerminal(t *testing.T) {
	t.Parallel()
	for _, computation := range []bool{false, true} {
		runScenario(t, profileName(computation), func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			repeat(t, func() bool { return correct(f.invoke(profile(computation), "terminal", false), "terminal absent\n") })
		})
	}
}

// These platform classes stay open implicitly. Each must have an executable
// witness; their unfiltered visibility is reported as a limit.
func TestImplicitAllowanceWitnesses(t *testing.T) {
	t.Parallel()
	for _, entry := range []struct{ operation, mode, want string }{
		{"dynamic-code-generation", "clean", "clean\n"},
		{"file-test-existence", "clean", "clean\n"},
		{"sandbox-check", "user", "user ok\n"},
		{"syscall-mach", "clean", "clean\n"},
		{"syscall-mig", "user", "user ok\n"},
		{"syscall-unix", "clean", "clean\n"},
		{"system-fcntl", "clean", "clean\n"},
		{"system-mac-syscall", "clean", "clean\n"},
		{"mach-derive-port", "user", "user ok\n"},
	} {
		for _, computation := range []bool{false, true} {
			runScenario(t, profileName(computation)+"/"+entry.operation, func(t *testing.T) {
				t.Parallel()
				f := newFixture(t)
				repeat(t, func() bool {
					p := profile(computation)
					if !correct(f.invoke(p, entry.mode, false), entry.want) {
						return false
					}
					broken := p + "\n(deny " + entry.operation + " (with send-signal SIGKILL))\n"
					r := f.invoke(broken, entry.mode, false)
					return !r.timedOut && r.status.Signaled() && r.status.Signal() == syscall.SIGKILL && r.output == ""
				})
				t.Logf("implicit allowance=%s clean-program=%s", entry.operation, entry.mode)
			})
		}
	}
}

func TestSubjectDirectSessionAndGroupSmoke(t *testing.T) {
	t.Log("smoke test: these kills do not isolate the operation rule")
	t.Parallel()
	f := newFixture(t)
	for _, mode := range []string{"direct-session", "direct-group"} {
		runScenario(t, mode, func(t *testing.T) {
			repeat(t, func() bool { return correct(f.invoke(profile(false), mode, false), "session leader refused\n") })
		})
	}
	runScenario(t, "plain-spawn", func(t *testing.T) {
		repeat(t, func() bool { return f.invoke(profile(false), "spawn", false).killed() })
	})
	runScenario(t, "plain-spawn-positive", func(t *testing.T) {
		repeat(t, func() bool { return correct(f.invoke("", "spawn", false), "clean\nperformed\nfallback\n") })
	})
	runScenario(t, "plain-spawn-twin", func(t *testing.T) {
		repeat(t, func() bool { return correct(f.invoke(profile(false), "spawn", true), "fallback\n") })
	})
}

func TestServiceNamesAreExact(t *testing.T) {
	t.Parallel()
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal("create unregistered suffix")
	}
	for _, computation := range []bool{false, true} {
		for _, allowed := range []string{"com.apple.system.opendirectoryd.libinfo", "com.apple.system.opendirectoryd.membership", "com.apple.system.notification_center", "com.apple.logd"} {
			for _, suffix := range []string{".sibling", "." + hex.EncodeToString(random[:])} {
				runScenario(t, profileName(computation)+"/"+allowed+suffix, func(t *testing.T) {
					t.Parallel()
					f := newFixture(t)
					probe := f
					probe.args = []string{allowed + suffix}
					repeat(t, func() bool {
						return probe.invoke(profile(computation), "service", false).killed() && correct(probe.invoke(profile(computation)+"\n(allow mach-lookup)", "service", false), "fallback\n")
					})
				})
			}
		}
	}
}

type allowanceRemoval struct{ original, witness, replacement string }

func allowanceRemovalTargets(p string) []allowanceRemoval {
	var removals []allowanceRemoval
	for _, line := range strings.Split(p, "\n") {
		if !strings.HasPrefix(line, "(allow ") {
			continue
		}
		depth, start := 0, 0
		var filters []string
		first := -1
		for index, char := range line {
			if char == '(' {
				if depth == 1 {
					start = index
					if first < 0 {
						first = index
					}
				}
				depth++
			}
			if char == ')' {
				depth--
				if depth == 1 {
					filters = append(filters, line[start:index+1])
				}
			}
		}
		if len(filters) == 0 {
			removals = append(removals, allowanceRemoval{original: line, witness: line})
			continue
		}
		for _, filter := range filters {
			replacement := ""
			if len(filters) > 1 {
				replacement = strings.Replace(line, " "+filter, "", 1)
			}
			removals = append(removals, allowanceRemoval{line, strings.TrimSpace(line[:first]) + " " + filter + ")", replacement})
		}
	}
	return removals
}

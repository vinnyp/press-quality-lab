//go:build confinementspike && darwin

package confinement

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The clean and violating binaries occupy precisely the same executable grant.
// Missing application and missing kill are both failures of this check, distinct
// from the exit code of a later subject. It never invokes an unconfined fallback.
func confinementSelfTest(t *testing.T, f fixture, p string, computation bool) bool {
	t.Helper()
	root, err := os.MkdirTemp(f.root, "selftest-")
	if err != nil {
		return false
	}
	defer os.RemoveAll(root)
	target := filepath.Join(root, "target")
	if os.WriteFile(target, []byte("decoy"), 0600) != nil {
		return false
	}
	f.args = nil
	if !correct(f.invoke(p, "clean", false), "clean\n") {
		return false
	}
	for _, mode := range []string{"file-read", "file-map", "file-write", "file-create"} {
		probe, rules := f, p
		probe.args = []string{target, target, filepath.Join(root, "unused")}
		if mode == "file-create" {
			probe.args[0] = filepath.Join(root, "created")
			probe.params = append(append([]string{}, f.params...), "CREATE_TARGET="+probe.args[0])
			// Opening for creation also checks write access; isolate creation.
			rules += "\n(allow file-write-data (literal (param \"CREATE_TARGET\")))\n"
		}
		before := decoySnapshot(probe.args[0])
		if !probe.invoke(rules, mode, false).killed() || before != decoySnapshot(probe.args[0]) {
			return false
		}
	}
	for _, mode := range []string{"tcp", "unix", "udp", "bind", "listen"} {
		network, quiet, closeDecoy := networkDecoy(t, f, mode)
		rules := p
		if mode == "listen" {
			rules += "\n(allow network-bind (local ip \"localhost:*\"))\n"
		}
		r := network.invoke(rules, mode, false)
		untouched := quiet()
		closeDecoy()
		if !r.killed() || !untouched {
			return false
		}
	}
	service := f
	service.args = []string{"com.apple.pasteboard.1"}
	if !service.invoke(p, "service", false).killed() {
		return false
	}
	if computation {
		if !correct(f.invoke(p, "child", false), "performed\nfallback\n") {
			return false
		}
		for _, mode := range []string{"session", "group"} {
			if !childKilled(f.invoke(p, mode, false)) {
				return false
			}
		}
	} else if !f.invoke(p, "child", false).killed() {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	victim := exec.CommandContext(ctx, f.probe, "victim")
	victim.Env = f.env
	victim.Dir = f.root
	victim.WaitDelay = time.Second
	out, err := victim.StdoutPipe()
	if err != nil {
		return false
	}
	if victim.Start() != nil {
		return false
	}
	defer func() { _ = victim.Process.Kill(); _ = victim.Wait() }()
	if line, err := bufio.NewReader(out).ReadString('\n'); err != nil || line != "ready\n" {
		return false
	}
	queue, err := syscall.Kqueue()
	if err != nil {
		return false
	}
	defer syscall.Close(queue)
	changes := []syscall.Kevent_t{{Ident: uint64(victim.Process.Pid), Filter: syscall.EVFILT_PROC, Flags: syscall.EV_ADD | syscall.EV_ONESHOT, Fflags: syscall.NOTE_EXIT}}
	if _, err := syscall.Kevent(queue, changes, nil, nil); err != nil {
		return false
	}
	for _, mode := range []string{"arguments", "process-environment", "process-pidinfo", "signal", "codesigning", "signature", "priority", "rusage", "process-record"} {
		probe := f
		probe.args = []string{strconv.Itoa(victim.Process.Pid)}
		rules := p
		// Reading unchanged priority is setup, not the scheduling operation.
		if mode == "priority" {
			rules += "\n(allow process-info-pidinfo)\n"
		}
		if !probe.invoke(rules, mode, false).killed() {
			return false
		}
		events := make([]syscall.Kevent_t, 1)
		if n, err := waitEvent(queue, nil, events, 20*time.Millisecond); err != nil || n != 0 {
			return false
		}
	}
	return true
}

func selfTestFixture(t *testing.T) fixture {
	t.Helper()
	f := newFixture(t)
	outside, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal("resolve outside decoy")
	}
	f.args = []string{filepath.Join(outside, "target")}

	writeDecoy(t, filepath.Join(outside, "target"))
	return f
}

func TestConfinementApplication(t *testing.T) {
	t.Parallel()
	for _, computation := range []bool{false, true} {
		for _, form := range []string{"ordinary", "missing-profile", "unparsable-profile", "without-kill", "missing-launcher"} {
			runScenario(t, profileName(computation)+"/"+form, func(t *testing.T) {
				t.Parallel()
				f := selfTestFixture(t)
				repeat(t, func() bool {
					probe, p := f, profile(computation)
					switch form {
					case "missing-profile":
						probe.profileFile = filepath.Join(f.root, "missing-profile")
					case "unparsable-profile":
						p = "(version 1)("
					case "without-kill":
						p = strings.ReplaceAll(p, "(with send-signal SIGKILL)", "")
					case "missing-launcher":
						probe.launcher = filepath.Join(f.root, "missing-launcher")
					}
					before := decoySnapshot(f.args[0])
					passed := confinementSelfTest(t, probe, p, computation)
					if form == "unparsable-profile" || form == "missing-profile" || form == "missing-launcher" {
						application := probe
						application.args = nil
						r := application.invoke(p, "clean", false)
						if r.marked || r.timedOut || r.err == nil || r.output != "" {
							return false
						}
					}
					return passed == (form == "ordinary") && before == decoySnapshot(f.args[0])
				})
			})
		}
		runScenario(t, profileName(computation)+"/nested-parent", func(t *testing.T) {
			t.Parallel()
			f := selfTestFixture(t)
			repeat(t, func() bool {
				nested := f
				nested.nested = true
				if confinementSelfTest(t, nested, profile(computation), computation) {
					return false
				}
				args := []string{"-p", "(version 1)(allow default)", "/usr/bin/sandbox-exec", "-p", profile(computation)}
				for _, parameter := range f.parameters() {
					args = append(args, "-D", parameter)
				}
				args = append(args, f.launcher, f.probe, "clean")
				executed.Add(1)
				r := run("/usr/bin/sandbox-exec", args, f.env, f.root)
				return !r.timedOut && r.status.Exited() && r.status.ExitStatus() == 71 && !r.marked && r.output == ""
			})
		})
	}
}

func TestPathParametersSmoke(t *testing.T) {
	t.Log("smoke test: these kills do not isolate the operation rule")
	t.Parallel()
	f := selfTestFixture(t)
	original := f
	root := filepath.Join(f.root, "quotes \" ) (allow default) ;\nspaces")
	if os.Mkdir(root, 0700) != nil {
		t.Fatal("create syntax-bearing path")
	}
	for _, name := range []string{"probe", "launcher"} {
		data, err := os.ReadFile(filepath.Join(f.root, name))
		if err != nil {
			t.Fatal("read generated binary")
		}
		if os.WriteFile(filepath.Join(root, name), data, 0700) != nil {
			t.Fatal("copy generated binary")
		}
	}
	f.root, f.probe, f.launcher = root, filepath.Join(root, "probe"), filepath.Join(root, "launcher")
	home := filepath.Join(root, "home")
	if os.Mkdir(home, 0700) != nil {
		t.Fatal("create syntax-bearing home")
	}
	f.env = []string{"HOME=" + home, "TMPDIR=" + home, "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TZ=UTC"}
	for _, computation := range []bool{false, true} {
		runScenario(t, profileName(computation), func(t *testing.T) {
			repeat(t, func() bool {
				f.args = nil
				if !correct(f.invoke(profile(computation), "clean", false), "clean\n") {
					return false
				}
				f.args = []string{original.args[0], original.args[0], filepath.Join(root, "unused")}
				return f.invoke(profile(computation), "file-read", false).killed()
			})
		})
	}
}

// Structural validation has no role in the behavioral self-test.
func TestRefusalStructure(t *testing.T) {
	t.Parallel()
	for _, computation := range []bool{false, true} {
		if !refusalsIntact(profile(computation), computation) {
			t.Fatal("required structural refusal missing")
		}
	}
}

func TestSelfTestMutations(t *testing.T) {
	t.Parallel()
	for _, computation := range []bool{false, true} {
		p := profile(computation)
		runScenario(t, profileName(computation)+"/baseline", func(t *testing.T) {
			t.Parallel()
			f := selfTestFixture(t)
			repeat(t, func() bool { return confinementSelfTest(t, f, p, computation) })
		})
		mutations := []struct{ name, text string }{
			{"outbound", p + "\n(allow network-outbound)"},
			{"inbound", p + "\n(allow network-inbound)"},
			{"services", p + "\n(allow mach-lookup)"},
			{"contents", p + "\n(allow file-read-data)"},
			{"kills", strings.ReplaceAll(p, "(with send-signal SIGKILL)", "")},
			{"process-information", p + "\n(allow process-info*)"},
			{"file-writes", p + "\n(allow file-write-data)"},
			{"file-creation", p + "\n(allow file-write-create)"},
			{"legacy-status", p + "\n(allow process-legacy-codesigning-status-get)"},
			{"legacy-signature", p + "\n(allow process-legacy-codesigning-cdhash-get)"},
			{"scheduling", p + "\n(allow system-sched)"},
		}
		if !computation {
			mutations = append(mutations, struct{ name, text string }{"children", p + "\n(allow process-fork)"})
		}
		if computation {
			mutations = append(mutations, struct{ name, text string }{
				"session-group-kill",
				strings.Replace(p, "(deny syscall-unix (syscall-number SYS_setsid SYS_setpgid) (with send-signal SIGKILL))", "(deny syscall-unix (syscall-number SYS_setsid SYS_setpgid))", 1),
			})
		}
		for _, mutation := range mutations {
			runScenario(t, profileName(computation)+"/"+mutation.name, func(t *testing.T) {
				t.Parallel()
				f := selfTestFixture(t)
				repeat(t, func() bool { return !confinementSelfTest(t, f, mutation.text, computation) })
			})
		}
	}
}

// A trusted launcher reports its own exec failure separately from a program
// which starts successfully and returns the launcher's reserved exit status.
func TestLauncherExecFailure(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	invalid := filepath.Join(f.root, "invalid-image")
	if os.WriteFile(invalid, []byte("invalid executable image\n"), 0700) != nil {
		t.Fatal("write invalid image")
	}
	for _, computation := range []bool{false, true} {
		for _, failure := range []bool{false, true} {
			name := "program-exit"
			if failure {
				name = "launcher-exec-failure"
			}
			runScenario(t, profileName(computation)+"/"+name, func(t *testing.T) {
				repeat(t, func() bool {
					probe := f
					if failure {
						probe.probe = invalid
					}
					r := probe.invoke(profile(computation), "exit-launch-code", false)
					diagnostic := "confined\n"
					if failure {
						diagnostic += "launch-failed\n"
					}
					return r.startFailed == failure && r.marked && !r.ready && !r.timedOut && !r.harnessKilled && r.status.Exited() && r.status.ExitStatus() == 71 && r.output == "" && r.diagnostic == diagnostic
				})
			})
		}
	}
}

func TestLauncherFailureMarkerMutation(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	source, err := os.ReadFile("testdata/programs/launcher.c")
	if err != nil {
		t.Fatal("read launcher source")
	}
	broken := strings.Replace(string(source), `if (write(2, "launch-failed\n", 14) != 14) return 70;`, "", 1)
	if broken == string(source) {
		t.Fatal("launcher mutation did not apply")
	}
	path := filepath.Join(f.root, "mutant-launcher.c")
	if os.WriteFile(path, []byte(broken), 0600) != nil {
		t.Fatal("write launcher mutant")
	}
	f.launcher = filepath.Join(f.root, "mutant-launcher")
	if r := run("/usr/bin/clang", []string{"-Wall", "-Wextra", "-Werror", "-o", f.launcher, path}, f.env, f.root); r.err != nil {
		t.Fatal("compile launcher mutant")
	}
	invalid := filepath.Join(f.root, "invalid-image")
	if os.WriteFile(invalid, []byte("invalid executable image\n"), 0700) != nil {
		t.Fatal("write invalid image")
	}
	f.probe = invalid
	for _, computation := range []bool{false, true} {
		runScenario(t, profileName(computation), func(t *testing.T) {
			repeat(t, func() bool {
				r := f.invoke(profile(computation), "unused", false)
				// The ordinary failure test requires startFailed. This mutant
				// loses that distinction while preserving the same exit code.
				return !r.startFailed && r.marked && !r.ready && !r.timedOut && !r.harnessKilled && r.status.Exited() && r.status.ExitStatus() == 71 && r.output == "" && r.diagnostic == "confined\n"
			})
		})
	}
}

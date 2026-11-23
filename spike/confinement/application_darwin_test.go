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
	f.params = append(append([]string{}, f.params...), "SELFTEST_TARGET="+filepath.Join(root, "target"), "SELFTEST_MAP="+filepath.Join(root, "mapped"), "SELFTEST_CREATE="+filepath.Join(root, "created"))
	target := filepath.Join(root, "target")
	if os.WriteFile(target, []byte("decoy"), 0600) != nil {
		return false
	}
	passed := true
	check := func(mode string, ok bool) {
		if f.selfTestResults != nil {
			f.selfTestResults[mode] = ok
		}
		passed = passed && ok
	}
	f.args = nil
	if !correct(f.invokeOn(t, p, "clean", false), "clean\n") {
		return false
	}
	for _, mode := range []string{"file-read", "file-map", "file-write", "file-create"} {
		probe, rules := f, p
		probe.args = []string{target, target, filepath.Join(root, "unused")}
		probe.params = append(append([]string{}, f.params...), "SELFTEST_TARGET="+target)
		if mode == "file-map" {
			probe.args[0] = filepath.Join(root, "mapped")
			if os.WriteFile(probe.args[0], []byte("decoy"), 0600) != nil {
				return false
			}
			probe.params = append(probe.params, "SELFTEST_MAP="+probe.args[0])
		}
		if mode == "file-create" {
			probe.args[0] = filepath.Join(root, "created")
			probe.params = append(append([]string{}, f.params...), "CREATE_TARGET="+probe.args[0])
			// Opening for creation also checks write access; isolate creation.
			rules += "\n(allow file-write-data (literal (param \"CREATE_TARGET\")))\n"
		}
		before := decoySnapshot(probe.args[0])
		check(mode, probe.invokeOn(t, rules, mode, false).killed() && before == decoySnapshot(probe.args[0]))
	}
	for _, mode := range []string{"tcp", "unix", "udp", "bind", "listen"} {
		network, quiet, closeDecoy := networkDecoy(t, f, mode)
		rules := p
		if mode == "listen" {
			rules += "\n(allow network-bind (local ip \"localhost:*\"))\n"
		}
		r := network.invokeOn(t, rules, mode, false)
		untouched := quiet()
		closeDecoy()
		check(mode, r.killed() && untouched)
	}
	check("sysctl-read", f.invokeOn(t, p, "witness-sysctl", false).killed())
	service := f
	service.args = []string{"com.apple.pasteboard.1"}
	check("service", service.invokeOn(t, p, "service", false).killed())
	if computation {
		check("child", correct(f.invokeOn(t, p, "child", false), "performed\nfallback\n"))
		for _, mode := range []string{"session", "group"} {
			check(mode, childKilled(f.invokeOn(t, p, mode, false)))
		}
	} else {
		check("child", f.invokeOn(t, p, "child", false).killed())
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
	changes := []syscall.Kevent_t{{Ident: uint64(victim.Process.Pid), Filter: syscall.EVFILT_PROC, Flags: syscall.EV_ADD | syscall.EV_ONESHOT, Fflags: syscall.NOTE_EXIT | syscall.NOTE_EXIT_DETAIL}}
	if _, err := syscall.Kevent(queue, changes, nil, nil); err != nil {
		return false
	}
	for _, mode := range []string{"arguments", "process-environment", "process-pidinfo", "codesigning", "signature", "priority", "rusage", "process-record", "signal"} {
		probe := f
		probe.args = []string{strconv.Itoa(victim.Process.Pid)}
		rules := p
		// Reading unchanged priority is setup, not the scheduling operation.
		if mode == "priority" {
			rules += "\n(allow process-info-pidinfo)\n"
		}
		killed := probe.invokeOn(t, rules, mode, false).killed()
		events := make([]syscall.Kevent_t, 1)
		n, err := waitEvent(queue, nil, events, 20*time.Millisecond)
		check(mode, killed && err == nil && n == 0)
	}
	return passed
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
						probe.missingLauncher = true
					}
					before := decoySnapshot(f.args[0])
					passed := confinementSelfTest(t, probe, p, computation)
					if form == "unparsable-profile" || form == "missing-profile" || form == "missing-launcher" {
						application := probe
						application.args = nil
						application, release, err := application.prepareImagesOn(t)
						if err != nil {
							t.Fatal("prepare application images")
						}
						defer release()
						r := application.invokeOn(t, p, "clean", false)
						if form == "missing-launcher" {
							want := "sandbox-exec: execvp() of '" + application.launcher + "' failed: No such file or directory\n"
							if !r.status.Exited() || r.status.ExitStatus() != 71 || r.diagnostic != want {
								return false
							}
						}
						if r.marked || r.timedOut || r.err == nil || r.output != "" {
							return false
						}
					}
					if form == "without-kill" && !profileApplied(t, probe, p) {
						return false
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
				if !correct(f.invokeOn(t, profile(computation), "clean", false), "clean\n") {
					return false
				}
				f.args = []string{original.args[0], original.args[0], filepath.Join(root, "unused")}
				return f.invokeOn(t, profile(computation), "file-read", false).killed()
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

// A mutant that fails the self-test only counts when sandbox-exec applied it:
// a profile it rejects fails every program and proves nothing about the rule.
func profileApplied(t *testing.T, f fixture, text string) bool {
	probe := f
	probe.args = nil
	return probe.invokeOn(t, text, "clean", false).marked
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
			{"tcp", p + "\n(allow network-outbound (remote tcp \"localhost:*\"))"},
			{"unix", p + "\n(allow network-outbound (remote unix-socket))"},
			{"udp", p + "\n(allow network-outbound (remote udp \"localhost:*\"))"},
			{"bind", p + "\n(allow network-bind)"},
			{"sysctl-read", p + "\n(allow sysctl-read (sysctl-name \"kern.osrelease\"))"},
			{"inbound", p + "\n(allow network-inbound)"},
			{"services", p + "\n(allow mach-lookup (global-name \"com.apple.pasteboard.1\"))"},
			{"file-read", p + "\n(allow file-read-data (literal (param \"SELFTEST_TARGET\")))"},
			{"file-map", p + "\n(allow file-read-data (literal (param \"SELFTEST_MAP\")))"},
			{"kills", strings.ReplaceAll(p, "(with send-signal SIGKILL)", "")},
			{"arguments", p + "\n(allow process-info-pidinfo)"},
			{"rusage", p + "\n(allow process-info-rusage)"},

			{"file-writes", p + "\n(allow file-write-data (literal (param \"SELFTEST_TARGET\")))"},
			{"signal", p + "\n(allow signal)"},
			{"file-creation", p + "\n(allow file-write-create (literal (param \"SELFTEST_CREATE\")))"},
			{"legacy-status", p + "\n(allow process-legacy-codesigning-status-get)"},
			{"legacy-signature", p + "\n(allow process-legacy-codesigning-cdhash-get)"},
			{"scheduling", p + "\n(allow system-sched)"},
		}
		for _, mode := range []string{"process-environment", "process-pidinfo", "process-record"} {
			mutations = append(mutations, struct{ name, text string }{mode, p + "\n(allow process-info-pidinfo)"})
		}
		if !computation {
			mutations = append(mutations, struct{ name, text string }{"children", p + "\n(allow process-fork)"})
		}
		if computation {
			mutations = append(mutations, struct{ name, text string }{"missing-child-allow", strings.Replace(p, "(allow process-fork)", "", 1)})
			for _, number := range []string{"SYS_setsid", "SYS_setpgid"} {
				other := "SYS_setsid"
				if number == other {
					other = "SYS_setpgid"
				}
				mutations = append(mutations, struct{ name, text string }{number, strings.Replace(p, "(deny syscall-unix (syscall-number SYS_setsid SYS_setpgid) (with send-signal SIGKILL))", "(deny syscall-unix (syscall-number "+other+") (with send-signal SIGKILL))\n(deny syscall-unix (syscall-number "+number+"))", 1)})
			}
			mutations = append(mutations, struct{ name, text string }{
				"session-group-kill",
				strings.Replace(p, "(deny syscall-unix (syscall-number SYS_setsid SYS_setpgid) (with send-signal SIGKILL))", "(deny syscall-unix (syscall-number SYS_setsid SYS_setpgid))", 1),
			})
		}
		for _, mutation := range mutations {
			runScenario(t, profileName(computation)+"/"+mutation.name, func(t *testing.T) {
				t.Parallel()
				f := selfTestFixture(t)
				repeat(t, func() bool {
					f.selfTestResults = map[string]bool{}
					failed := !confinementSelfTest(t, f, mutation.text, computation)
					witnesses := map[string]string{"inbound": "listen", "services": "service", "file-writes": "file-write", "file-creation": "file-create", "legacy-status": "codesigning", "legacy-signature": "signature", "scheduling": "priority", "children": "child", "missing-child-allow": "child", "SYS_setsid": "session", "SYS_setpgid": "group"}
					mode := mutation.name
					if replacement, ok := witnesses[mode]; ok {
						mode = replacement
					}
					if mode == "kills" || mode == "session-group-kill" {
						return failed && profileApplied(t, f, mutation.text)
					}
					held, ran := f.selfTestResults[mode]
					return failed && ran && !held
				})
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
					r := probe.invokeOn(t, profile(computation), "exit-launch-code", false)
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
				r := f.invokeOn(t, profile(computation), "unused", false)
				// The ordinary failure test requires startFailed. This mutant
				// loses that distinction while preserving the same exit code.
				return !r.startFailed && r.marked && !r.ready && !r.timedOut && !r.harnessKilled && r.status.Exited() && r.status.ExitStatus() == 71 && r.output == "" && r.diagnostic == "confined\n"
			})
		})
	}
}

func TestExitedChildObservation(t *testing.T) {
	f := newFixture(t)
	runScenario(t, "exited-unreaped", func(t *testing.T) {
		repeat(t, func() bool {
			executed.Add(1)
			registered := false
			r := runBudgetWithObserver(f.probe, []string{"clean"}, f.env, f.home(), 5*time.Second, nil, nil, func(pid int) {
				queue, err := syscall.Kqueue()
				if err != nil {
					t.Fatal("create exit queue")
				}
				defer syscall.Close(queue)
				changes := []syscall.Kevent_t{{Ident: uint64(pid), Filter: syscall.EVFILT_PROC, Flags: syscall.EV_ADD | syscall.EV_ONESHOT, Fflags: syscall.NOTE_EXIT | syscall.NOTE_EXIT_DETAIL}}
				events := make([]syscall.Kevent_t, 1)
				n, err := waitEvent(queue, changes, events, 5*time.Second)
				if err != nil && err != syscall.ESRCH {
					t.Fatal("observe child exit")
				}
				if err == nil && (n != 1 || events[0].Fflags&syscall.NOTE_EXIT == 0) {
					t.Fatal("child did not exit")
				}
				registered = true
			})
			return registered && !r.harnessKilled && !r.timedOut && r.err == nil && r.status.Exited() && r.status.ExitStatus() == 0 && r.output == "clean\n"
		})
	})
}

func TestKillExitDetails(t *testing.T) {
	for _, row := range []struct {
		name   string
		detail uint32
		want   bool
	}{
		{"ordinary", 0, true}, {"memory", syscall.NOTE_EXIT_MEMORY, false}, {"code-signing", syscall.NOTE_EXIT_CSERROR, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			r := result{ready: true, marked: true, status: syscall.WaitStatus(syscall.SIGKILL), diagnostic: "confined\n", exitDetail: row.detail}
			if r.killed() != row.want {
				t.Error("kill classification differs")
			}
		})
	}
}

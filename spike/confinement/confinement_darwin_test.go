//go:build confinementspike && darwin

package confinement

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
	env                   []string
	args                  []string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal("resolve temporary root")
	}
	f := fixture{root: root, launcher: filepath.Join(root, "launcher"), probe: filepath.Join(root, "probe")}
	f.env = []string{"HOME=" + root, "TMPDIR=" + root, "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TZ=UTC"}
	for _, build := range []struct{ source, binary string }{{"launcher.c", f.launcher}, {"probe.c", f.probe}} {
		r := run("/usr/bin/clang", []string{"-Wall", "-Wextra", "-Werror", "-o", build.binary, filepath.Join("testdata/programs", build.source)}, f.env, "")
		if r.err != nil {
			t.Fatal("compile fixture failed")
		}
	}
	return f
}

func (f fixture) invoke(text, mode string, twin bool) result {
	args := []string{"-p", text, "-D", "BIN=" + f.probe, "-D", "LAUNCHER=" + f.launcher, "-D", "WORKDIR=" + f.root, "-D", "ROOT=/", "-D", "SYSTEM=/System", "-D", "TRACE_DEVICE=/dev/dtracehelper", f.launcher, f.probe, mode}
	if text == "" {
		args = []string{f.probe, mode}
	}
	args = append(args, f.args...)
	if twin {
		args = append(args, "twin")
	}
	executed.Add(1)
	if text == "" {
		return run(f.launcher, args, f.env, f.root)
	}
	return run("/usr/bin/sandbox-exec", args, f.env, f.root)
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

func correct(r result, output string) bool {
	return r.err == nil && r.marked && !r.timedOut && r.status.Exited() && r.status.ExitStatus() == 0 && r.output == output
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
	return err == nil && status.Signaled() && status.Signal() == syscall.SIGKILL && correct(r, fmt.Sprintf("%s%d\n", prefix, raw))
}

func TestProcessIsolation(t *testing.T) {
	f := newFixture(t)
	for _, computation := range []bool{false, true} {
		name := "subject"
		if computation {
			name = "computation"
		}
		for _, mode := range []string{"clean", "child", "session", "group"} {
			runScenario(t, name+"/"+mode, func(t *testing.T) {
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
	f := newFixture(t)
	for _, mode := range []string{"child", "session", "group"} {
		runScenario(t, "unconfined/"+mode, func(t *testing.T) {
			repeat(t, func() bool { return correct(f.invoke("", mode, false), "performed\nfallback\n") })
		})
		// Escapes have permitted twins only in a computation, where fork is allowed.
		computation := mode != "child"
		runScenario(t, "twin/"+mode, func(t *testing.T) {
			want := "performed\nfallback\n"
			if mode == "child" {
				want = "fallback\n"
			}
			repeat(t, func() bool { return correct(f.invoke(profile(computation), mode, true), want) })
		})
	}
}

func TestAllowanceWitnesses(t *testing.T) {
	f := newFixture(t)
	for _, computation := range []bool{false, true} {
		for index, line := range strings.Split(profile(computation), "\n") {
			if !strings.HasPrefix(line, "(allow ") {
				continue
			}
			runScenario(t, fmt.Sprintf("%s/%02d-%s", profileName(computation), index, strings.Fields(line)[1]), func(t *testing.T) {
				mode, want := "clean", "clean\n"
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
				runScenario(t, "baseline", func(t *testing.T) {
					repeat(t, func() bool { return correct(f.invoke(profile(computation), mode, false), want) })
				})
				broken := strings.Replace(profile(computation), line, "", 1)
				runScenario(t, "removed", func(t *testing.T) {
					repeat(t, func() bool {
						r := f.invoke(broken, mode, false)
						return !r.timedOut && r.status.Signaled() && r.status.Signal() == syscall.SIGKILL && r.output == ""
					})
				})
			})
		}
	}
}

func TestDefaultRuleMutations(t *testing.T) {
	f := newFixture(t)
	for _, mutation := range []struct{ name, from, to string }{
		{"remove-default-denial", "(deny default (with send-signal SIGKILL))", ""},
		{"remove-default-kill", "(deny default (with send-signal SIGKILL))", "(deny default)"},
	} {
		runScenario(t, mutation.name, func(t *testing.T) {
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
	f := newFixture(t)
	for _, computation := range []bool{false, true} {
		for _, mode := range []string{"tcp", "udp", "http", "dns", "unix", "bind", "listen"} {
			for _, form := range []string{"confined", "twin", "positive", "without-kill"} {
				runScenario(t, fmt.Sprintf("%s/%s/%s", profileName(computation), mode, form), func(t *testing.T) {
					repeat(t, func() bool {
						probe, untouched, closeDecoy := networkDecoy(t, f, mode)
						defer closeDecoy()
						p := profile(computation)
						if form == "positive" {
							p = ""
						}
						if form == "without-kill" {
							p = strings.ReplaceAll(p, "(with send-signal SIGKILL)", "")
						}
						r := probe.invoke(p, mode, form == "twin")
						quiet := untouched()
						switch form {
						case "confined":
							return r.killed() && quiet
						case "twin":
							return correct(r, "fallback\n") && quiet
						case "positive":
							return correct(r, "performed\nfallback\n") && (mode == "bind" || mode == "listen" || !quiet)
						default:
							return r.marked && !r.timedOut && !r.killed()
						}
					})
				})
			}
		}
	}
}

func networkDecoy(t *testing.T, f fixture, mode string) (fixture, func() bool, func()) {
	t.Helper()
	if mode == "bind" || mode == "listen" {
		f.args = []string{"0"}
		return f, func() bool { return true }, func() {}
	}
	if mode == "udp" || mode == "dns" {
		c, err := net.ListenPacket("udp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal("create UDP decoy")
		}
		f.args = []string{strconv.Itoa(c.LocalAddr().(*net.UDPAddr).Port)}
		return f, func() bool {
				if err := c.SetReadDeadline(time.Now().Add(time.Millisecond)); err != nil {
					t.Fatal("deadline")
				}
				var b [512]byte
				n, _, _ := c.ReadFrom(b[:])
				return n == 0
			}, func() {
				if err := c.Close(); err != nil {
					t.Error("close UDP decoy")
				}
			}
	}
	network, address := "tcp4", "127.0.0.1:0"
	if mode == "unix" {
		network, address = "unix", filepath.Join(f.root, "socket")
	}
	l, err := net.Listen(network, address)
	if err != nil {
		t.Fatal("create listener decoy")
	}
	if mode == "unix" {
		f.args = []string{address}
	} else {
		f.args = []string{strconv.Itoa(l.Addr().(*net.TCPAddr).Port)}
	}
	return f, func() bool {
			switch v := l.(type) {
			case *net.TCPListener:
				if err := v.SetDeadline(time.Now().Add(time.Millisecond)); err != nil {
					t.Fatal("deadline")
				}
			case *net.UnixListener:
				if err := v.SetDeadline(time.Now().Add(time.Millisecond)); err != nil {
					t.Fatal("deadline")
				}
			}
			c, err := l.Accept()
			if err != nil {
				return true
			}
			if err := c.Close(); err != nil {
				t.Error("close accepted decoy")
			}
			return false
		}, func() {
			if err := l.Close(); err != nil {
				t.Error("close listener decoy")
			}
		}
}

func TestProcessAccess(t *testing.T) {
	f := newFixture(t)
	for _, computation := range []bool{false, true} {
		for _, mode := range []string{"arguments", "process-environment", "process-pidinfo", "signal"} {
			for _, form := range []string{"confined", "twin", "positive", "without-kill"} {
				runScenario(t, fmt.Sprintf("%s/%s/%s", profileName(computation), mode, form), func(t *testing.T) {
					repeat(t, func() bool {
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()
						var secret [32]byte
						if _, err := rand.Read(secret[:]); err != nil {
							t.Fatal("create victim canary")
						}
						canary := hex.EncodeToString(secret[:])
						cmd := exec.CommandContext(ctx, f.probe, "victim", canary)
						cmd.Env = append(append([]string{}, f.env...), "DECOY_CANARY="+canary)
						cmd.Dir = f.root
						reader, err := cmd.StdoutPipe()
						if err != nil {
							t.Fatal("victim pipe")
						}
						if err := cmd.Start(); err != nil {
							t.Fatal("start victim")
						}
						done := make(chan error, 1)
						go func() { done <- cmd.Wait() }()
						waited := false
						defer func() {
							if !waited {
								_ = cmd.Process.Kill()
								<-done
							}
						}()
						if line, err := bufio.NewReader(reader).ReadString('\n'); err != nil || line != "ready\n" {
							t.Fatal("victim startup")
						}
						probe := f
						probe.args = []string{strconv.Itoa(cmd.Process.Pid), canary}
						probe.env = append(append([]string{}, f.env...), "DECOY_CANARY="+canary)
						p := profile(computation)
						if form == "positive" {
							p = ""
						}
						if form == "without-kill" {
							p = strings.ReplaceAll(p, "(with send-signal SIGKILL)", "")
						}
						r := probe.invoke(p, mode, form == "twin")
						alive := true
						select {
						case <-done:
							alive = false
							waited = true
						default:
						}
						if form == "positive" && mode == "signal" {
							if alive {
								select {
								case <-done:
									waited = true
									alive = false
								case <-time.After(100 * time.Millisecond):
								}
							}
							return correct(r, "performed\nfallback\n") && !alive
						}
						if form == "confined" {
							return r.killed() && alive
						}
						if form == "without-kill" {
							return !r.killed() && r.marked && !r.timedOut && alive
						}
						return correct(r, "performed\nfallback\n") && alive
					})
				})
			}
		}
	}
}

func TestServices(t *testing.T) {
	f := newFixture(t)
	for _, computation := range []bool{false, true} {
		runScenario(t, fmt.Sprintf("%s/user", profileName(computation)), func(t *testing.T) {
			repeat(t, func() bool { return correct(f.invoke(profile(computation), "user", false), "user ok\n") })
		})
		for _, service := range []string{"com.apple.cfprefsd.daemon", "com.apple.pasteboard.1", "com.apple.securityd.xpc", "com.apple.coreservices.appleevents", "com.apple.bsd.dirhelper", "com.apple.coreservices.launchservicesd"} {
			for _, form := range []string{"confined", "twin", "positive", "without-kill"} {
				runScenario(t, fmt.Sprintf("%s/%s/%s", profileName(computation), service, form), func(t *testing.T) {
					probe := f
					probe.args = []string{service}
					repeat(t, func() bool {
						p := profile(computation)
						if form == "positive" {
							p = ""
						}
						if form == "without-kill" {
							p = strings.ReplaceAll(p, "(with send-signal SIGKILL)", "")
						}
						r := probe.invoke(p, "service", form == "twin")
						if form == "confined" {
							return r.killed()
						}
						if form == "without-kill" {
							return r.marked && !r.timedOut && !r.killed()
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
	f := newFixture(t)
	for _, mode := range []string{"arguments", "process-environment", "process-pidinfo", "signal"} {
		runScenario(t, mode, func(t *testing.T) {
			repeat(t, func() bool { return correct(f.invoke(profile(true), "sandbox-"+mode, false), "performed\nfallback\n") })
		})
	}
}

func TestDescendants(t *testing.T) {
	f := newFixture(t)
	for _, mode := range []string{"session", "group"} {
		runScenario(t, mode+"/without-denial", func(t *testing.T) {
			p := strings.Replace(profile(true), "(deny syscall-unix (syscall-number SYS_setsid SYS_setpgid) (with send-signal SIGKILL))", "", 1)
			repeat(t, func() bool { return correct(f.invoke(p, mode, false), "performed\nfallback\n") })
		})
		runScenario(t, mode+"/without-kill", func(t *testing.T) {
			p := strings.Replace(profile(true), "(deny syscall-unix (syscall-number SYS_setsid SYS_setpgid) (with send-signal SIGKILL))", "(deny syscall-unix (syscall-number SYS_setsid SYS_setpgid))", 1)
			repeat(t, func() bool { return correct(f.invoke(p, mode, false), "fallback\nfallback\n") })
		})
	}
	for _, mode := range []string{"spawn-session", "spawn-group"} {
		for _, form := range []string{"confined", "positive", "twin", "without-kill"} {
			runScenario(t, mode+"/"+form, func(t *testing.T) {
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

func TestSubjectNetworkImages(t *testing.T) {
	f := newFixture(t)
	for _, mode := range []string{"child-network", "replace-network"} {
		for _, form := range []string{"confined", "twin", "positive", "without-kill"} {
			runScenario(t, mode+"/"+form, func(t *testing.T) {
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
						return r.marked && !r.timedOut && !r.killed() && unchanged
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
	f := newFixture(t)
	for _, mode := range []string{"left-child", "timeout-child"} {
		runScenario(t, mode, func(t *testing.T) {
			repeat(t, func() bool {
				args := []string{"-p", profile(true), "-D", "BIN=" + f.probe, "-D", "LAUNCHER=" + f.launcher, "-D", "WORKDIR=" + f.root, "-D", "ROOT=/", "-D", "SYSTEM=/System", "-D", "TRACE_DEVICE=/dev/dtracehelper", f.launcher, f.probe, mode}
				r := runBudget("/usr/bin/sandbox-exec", args, f.env, f.root, 250*time.Millisecond)
				executed.Add(1)
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
	f := newFixture(t)
	for _, computation := range []bool{false, true} {
		for _, mutation := range []struct{ name, from, to string }{
			{"remove-denial", "(deny process-info* (with send-signal SIGKILL))", ""},
			{"remove-kill", "(deny process-info* (with send-signal SIGKILL))", "(deny process-info*)"},
		} {
			runScenario(t, fmt.Sprintf("%s/%s", profileName(computation), mutation.name), func(t *testing.T) {
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
					return r.marked && !r.timedOut && !r.killed()
				})
			})
		}
	}
}

//go:build confinementspike && darwin

package confinement

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The clean and violating binaries occupy precisely the same executable grant.
// Missing application and missing kill are both failures of this check, distinct
// from the exit code of a later subject. It never invokes an unconfined fallback.
func confinementSelfTest(f fixture, p string) bool {
	if !refusalsIntact(p, strings.Contains(p, "(allow process-fork)")) {
		return false
	}
	target := f.args[0]
	f.args = nil
	if !correct(f.invoke(p, "clean", false), "clean\n") {
		return false
	}

	for _, mode := range []string{"file-read", "file-map", "file-write"} {
		probe := f
		probe.args = []string{target, target, filepath.Join(f.root, "unused")}
		if !probe.invoke(p, mode, false).killed() {
			return false
		}
	}
	network := f
	network.args = []string{"0"}
	if !network.invoke(p, "bind", false).killed() {
		return false
	}
	service := f
	service.args = []string{"com.apple.pasteboard.1"}
	if !service.invoke(p, "service", false).killed() {
		return false
	}
	if !strings.Contains(p, "(allow process-fork)") && !f.invoke(p, "child", false).killed() {
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
	for _, mode := range []string{"arguments", "process-environment", "process-pidinfo", "signal", "codesigning"} {
		probe := f
		probe.args = []string{strconv.Itoa(victim.Process.Pid)}
		if !probe.invoke(p, mode, false).killed() {
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
	f := selfTestFixture(t)
	for _, computation := range []bool{false, true} {
		for _, form := range []string{"ordinary", "missing-profile", "unparsable-profile", "without-kill", "missing-launcher"} {
			runScenario(t, profileName(computation)+"/"+form, func(t *testing.T) {
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
					passed := confinementSelfTest(probe, p)
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
			repeat(t, func() bool {
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

func TestPathParameters(t *testing.T) {
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

func TestRefusalMutations(t *testing.T) {
	f := selfTestFixture(t)
	for _, computation := range []bool{false, true} {
		p := profile(computation)
		runScenario(t, profileName(computation)+"/baseline", func(t *testing.T) { repeat(t, func() bool { return confinementSelfTest(f, p) }) })
		for index, line := range strings.Split(p, "\n") {
			if !strings.HasPrefix(line, "(deny ") {
				continue
			}
			mutations := []string{"remove-denial"}
			if strings.Contains(line, "(with send-signal SIGKILL)") {
				mutations = append(mutations, "remove-kill")
			}
			for _, mutation := range mutations {
				runScenario(t, fmt.Sprintf("%s/%d/%s", profileName(computation), index, mutation), func(t *testing.T) {
					repeat(t, func() bool {
						replacement := ""
						if mutation == "remove-kill" {
							replacement = strings.Replace(line, " (with send-signal SIGKILL)", "", 1)
						}
						broken := strings.Replace(p, line, replacement, 1)
						return !confinementSelfTest(f, broken)
					})
				})
			}
		}
	}
	t.Log("query-only refusal modifiers are checked structurally; destructive effects are not attempted")
}

//go:build confinementspike && startupdiagnosis && darwin

package confinement

import (
	"os"
	"strings"
	"testing"
)

// TestStartupDiagnosis reports platform startup needs without changing the profile.
func TestStartupDiagnosis(t *testing.T) {
	data, err := os.ReadFile("testdata/programs/operations.txt")
	if err != nil {
		t.Log("startup-diagnosis: profile=all program=all class=catalog unreadable")
		return
	}
	operations := strings.Fields(string(data))
	c := newFixture(t)
	g := buildGoClean(t, c)
	// Parse each rendered profile line alone, so every line this system
	// rejects is named with sandbox-exec's own error text.
	for _, computation := range []bool{false, true} {
		var defines []string
		for _, parameter := range c.parameters() {
			defines = append(defines, "-D", parameter)
		}
		for _, line := range strings.Split(profile(computation), "\n") {
			if !strings.HasPrefix(line, "(") || strings.HasPrefix(line, "(version") {
				continue
			}
			args := append(append([]string{"-p", "(version 1)\n(allow default)\n" + line}, defines...), "/usr/bin/true")
			r := run("/usr/bin/sandbox-exec", args, c.env, c.root)
			executed.Add(1)
			if r.status.Exited() && r.status.ExitStatus() != 0 {
				t.Logf("startup-diagnosis: profile=%s rejected-line=%q exit=%d error=%q", profileName(computation), line, r.status.ExitStatus(), strings.TrimSpace(r.diagnostic))
			}
		}
	}
	for _, computation := range []bool{false, true} {
		for _, program := range []struct {
			mode, want string
			fixture    fixture
		}{
			{"clean", "clean\n", c},
			{"help", "usage: synthetic [help|user|exit]\n", g},
			{"user", "user ok\n", g},
		} {
			name := profileName(computation)
			base := profile(computation)
			runs := 0
			attempt := func(p string) result {
				runs++
				return program.fixture.invoke(p, program.mode, false)
			}
			logResult := func(class string, r result) {
				t.Logf("startup-diagnosis: profile=%s program=%s class=%s correct=%v marker=%v exited=%v exit=%d wait-status=%d exit-detail=%d start-failed=%v timed-out=%v harness-killed=%v", name, program.mode, class, correct(r, program.want), r.marked, r.status.Exited(), r.status.ExitStatus(), r.status, r.exitDetail, r.startFailed, r.timedOut, r.harnessKilled)
			}
			baseline := attempt(base)
			logResult("rendered", baseline)
			// A class this system does not know makes the profile unparsable
			// (exit 65), so the full set holds only classes that parse alone.
			var allows, known []string
			for _, op := range operations {
				allow := "(allow " + op + ")"
				r := attempt(base + "\n" + allow)
				if r.status.Exited() && r.status.ExitStatus() == 65 && !r.marked {
					t.Logf("startup-diagnosis: profile=%s program=%s class=unknown-%s", name, program.mode, op)
					continue
				}
				allows, known = append(allows, allow), append(known, op)
				if !correct(baseline, program.want) && correct(r, program.want) {
					logResult("added-"+op, r)
				}
			}
			full := attempt(base + "\n" + strings.Join(allows, "\n"))
			logResult("all", full)
			if correct(full, program.want) {
				for i, op := range known {
					remaining := append([]string{}, allows[:i]...)
					remaining = append(remaining, allows[i+1:]...)
					r := attempt(base + "\n" + strings.Join(remaining, "\n"))
					if !correct(r, program.want) {
						logResult("removed-"+op, r)
					}
				}
			}
			t.Logf("startup-diagnosis: profile=%s program=%s class=summary executed=%d", name, program.mode, runs)
		}
	}
}

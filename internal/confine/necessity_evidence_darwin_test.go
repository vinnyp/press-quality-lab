//go:build darwin && necessity_test && macos_evidence

package confine

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/throwaway"
	"golang.org/x/sys/unix"
)

// No released renderer hook: the test uses startWith, retaining copy checks,
// the real launcher protocol, event observation, budgets and group cleanup.
type necessityScenario struct {
	w     *throwaway.Workspace
	c     *Copies
	inv   Invocation
	files *throwaway.SelfTestFiles
	decoy *selfTestDecoy
}

func newNecessityScenario(t *testing.T, binary string, computation bool) *necessityScenario {
	t.Helper()
	w, err := throwaway.BeginAt(filepath.Join(t.TempDir(), "runs"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := w.Remove(); err != nil {
			t.Error(err)
		}
	})
	c, err := NewCopies(w.Path(), binary, binary)
	if err != nil {
		t.Fatal(err)
	}
	files, err := w.PrepareSelfTest()
	if err != nil {
		t.Fatal(err)
	}
	l, p := c.CopyPaths()
	var fs unix.Statfs_t
	if err := unix.Statfs(files.Home, &fs); err != nil {
		t.Fatal(err)
	}
	var mount []byte
	for _, b := range fs.Mntonname {
		if b == 0 {
			break
		}
		mount = append(mount, byte(b))
	}
	values := map[string]string{
		"ROOT":            "/",
		"VOLUME":          string(mount),
		"RUNTIME":         "/System/Volumes/Preboot/Cryptexes",
		"HOME":            files.Home,
		"LAUNCHER":        l,
		"BIN":             p,
		"LAUNCHER_PARENT": filepath.Dir(l),
		"BIN_PARENT":      filepath.Dir(p),
		"TRACE_DEVICE":    "/dev/dtracehelper",
		"NULL_DEVICE":     "/dev/null",
		"RANDOM_DEVICE":   "/dev/urandom",
		"ZERO_DEVICE":     "/dev/zero",
		"STDOUT_DEVICE":   "/dev/fd/1",
		"STDERR_DEVICE":   "/dev/fd/2",
		"ZONEFILE":        "/usr/share/zoneinfo/UTC",
		"ZONE_RULES":      "/usr/share/zoneinfo/posixrules",
		"SERVICES":        "/private/etc/services",
		"PROTOCOLS":       "/private/etc/protocols",
		"PLANTED":         files.Calibration,
	}
	if computation {
		values["BIN_DIR"] = "/bin"
		values["USR_BIN_DIR"] = "/usr/bin"
		values["TTY_DEVICE"] = "/dev/tty"
		values["SUBJECT"] = files.Outside
		values["SUBJECT_COPY"] = files.Outside
	}
	inv := Invocation{Workspace: w, Paths: Paths{Values: values}, Directory: files.Home, Computation: computation, Budget: 10 * time.Second}
	decoy, err := startSelfTestDecoy(l, files.Home, w)
	if err != nil {
		t.Fatal(err)
	}
	scenario := &necessityScenario{w: w, c: c, inv: inv, files: files, decoy: decoy}
	t.Cleanup(func() {
		if scenario.decoy != nil {
			if err := scenario.decoy.close(); err != nil {
				t.Error(err)
			}
		}
	})
	return scenario
}

func (s *necessityScenario) run(t *testing.T, words []string, grant *necessityGrant, remove, prestart string) observe.Run {
	t.Helper()
	inv := s.inv
	inv.Words = words
	p, err := s.c.startWith(inv, func(path string, args []string, dir string, env []string, marker string) (Process, error) {
		if len(args) < 3 || args[1] != "-p" {
			return nil, fmt.Errorf("profile argument layout changed")
		}
		profile, err := necessityProfile(args[2], grant, remove)
		if err != nil {
			return nil, err
		}
		switch prestart {
		case "Exit":
			return spawn(path, []string{path, "-p", profile}, dir, env, marker)
		case "Rejected":
			profile = "(version 1) (u16-invalid-operation)"
		}
		args = slices.Clone(args)
		args[2] = profile
		// Extra paths are parameters, never interpolated into SBPL source.
		extra := []string{"-D", "U16_OUTSIDE=" + s.files.Outside, "-D", "U16_CREATE=" + s.files.Create}
		args = slices.Insert(args, 3, extra...)
		return spawn(path, args, dir, env, marker)
	})
	if err != nil {
		t.Fatal(err)
	}
	r, err := p.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if r.End.Code == 71 {
		t.Fatal("environment fault: sandbox-exec exited 71")
	}
	return r
}

func (s *necessityScenario) witnesses(t *testing.T, g *necessityGrant, repeat int) ([]string, []observe.Run) {
	t.Helper()
	names := []string{"tcp", "unix", "udp", "bind", "listen", "read", "map", "write", "create", "service", "sysctl", "arguments", "environment", "process", "signal", "codesign", "priority", "resource"}
	if s.inv.Computation {
		names = append(names, "setsid", "setpgid", "allowed_child")
	} else {
		names = append(names, "child")
	}
	names = append(names, "clean")
	runs := make([]observe.Run, 0, len(names))
	for _, name := range names {
		target := ""
		switch name {
		case "tcp":
			target = strconv.Itoa(s.decoy.tcp)
		case "udp":
			target = strconv.Itoa(s.decoy.udp)
		case "unix":
			target = "socket"
		case "read", "write":
			target = s.files.Outside
		case "create":
			target = s.files.Create
		case "map":
			_, target = s.c.CopyPaths()
		case "service":
			target = "org.pql.selftest.decoy"
		case "child", "allowed_child":
			target = "/usr/bin/true"
		case "clean":
			target = filepath.Join(s.files.Home, fmt.Sprintf("clean_%d_%d", repeat, len(runs)))
		}
		op := name
		if name == "allowed_child" {
			op = "child"
		}
		// A unique create target for each repetition, with one stable image pair.
		if name == "create" {
			s.files.Create = filepath.Join(filepath.Dir(s.files.Outside), fmt.Sprintf("create_%d", repeat))
			target = s.files.Create
		}
		r := s.run(t, []string{SelfTestMode, op, target, strconv.Itoa(s.decoy.pid)}, g, "", "")
		runs = append(runs, r)
	}
	return names, runs
}

func TestMacOSNecessity(t *testing.T) {
	// QH9, QH11, GR14: every one-operation grant fails the self-test with the
	// complete, exact problem list; all other operations still die at their step.
	binary := necessityBuild(t, false)
	for _, computation := range []bool{false, true} {
		profile := "Subject"
		if computation {
			profile = "Computation"
		}
		t.Run(profile, func(t *testing.T) {
			for _, grant := range necessityGrants {
				if grant.computationOnly && !computation || grant.subjectOnly && computation {
					continue
				}
				t.Run(grant.name, func(t *testing.T) {
					s := newNecessityScenario(t, binary, computation)
					t.Logf("operation=%s expected_problems=%v repeats=25", grant.operation, grant.programs)
					for repeat := range 25 {
						if repeat > 0 {
							s.refreshDecoy(t)
						}
						// Control and changed profile share their private executable pair.
						names, control := s.witnesses(t, nil, 2*repeat)
						problems, err := necessityProblems(names, control)
						if err != nil || len(problems) != 0 {
							t.Fatalf("repeat %d control: %v %v runs=%+v", repeat, problems, err, control)
						}
						names, runs := s.witnesses(t, &grant, 2*repeat+1)
						problems, err = necessityProblems(names, runs)
						if err != nil || !slices.Equal(problems, grant.programs) {
							t.Fatalf("repeat %d problems=%v want=%v error=%v runs=%+v", repeat, problems, grant.programs, err, runs)
						}
						for i, name := range names {
							if slices.Contains(grant.programs, name) && !necessityReturned(name, runs[i]) {
								t.Fatalf("repeat %d %s did not return at its operation: %+v", repeat, name, runs[i])
							}
						}
					}
				})
			}
		})
	}
}

func TestMacOSAllowances(t *testing.T) {
	// QH6, QH11, QH13, QH14, QH15, QH16: removal changes the sandbox
	// decision at the governed step; a native error is a returned operation.
	binary := necessityBuild(t, true)
	for _, computation := range []bool{false, true} {
		profile := "Subject"
		if computation {
			profile = "Computation"
		}
		t.Run(profile, func(t *testing.T) {
			for _, row := range necessityAllowances {
				if row.computationOnly && !computation {
					continue
				}
				t.Run(row.name, func(t *testing.T) {
					s := newNecessityScenario(t, binary, computation)
					homeFile := filepath.Join(s.files.Home, "value")
					declared := filepath.Join(filepath.Dir(s.files.Outside), "declared")
					seed := filepath.Join(filepath.Dir(s.files.Outside), "seed")
					for _, path := range []string{homeFile, declared, seed} {
						if err := os.WriteFile(path, []byte("clean_value\n"), 0600); err != nil {
							t.Fatal(err)
						}
					}
					if computation {
						s.inv.Paths.Declared = []string{declared}
						s.inv.Paths.Seed = []string{seed}
					}
					params := necessityPaths(t, s)
					argument := row.parameter
					switch argument {
					case "HOME_FILE":
						argument = homeFile
					case "DECLARED":
						argument = declared
					case "SEED":
						argument = seed
					default:
						if v, ok := params[argument]; ok {
							argument = v
						}
					}
					words := []string{SelfTestMode, "u16_allowance", row.operation, argument}
					if row.startup {
						s.inv.Fast = true
						words = nil
					}
					if row.operation == "child" {
						words = []string{SelfTestMode, "child", "/usr/bin/true", strconv.Itoa(s.decoy.pid)}
					}
					rendered := render(computation, false, 0, len(s.inv.Paths.Declared), len(s.inv.Paths.Seed))
					line, err := necessityAllowanceLine(rendered, row.prefix)
					if err != nil {
						t.Fatal(err)
					}
					t.Logf("removed=%s startup=%t repeats=25", line, row.startup)
					for repeat := range 25 {
						control := s.run(t, words, nil, "", "")
						if !necessityAllowanceControl(row, control) {
							t.Fatalf("repeat %d unmodified control not evaluated at step %s: %+v stdout=%q stderr=%q", repeat, row.operation, control, control.Stdout, control.Stderr)
						}
						removed := s.run(t, words, nil, line, "")
						marker := !row.startup
						output := "attempt_" + row.operation + "\n"
						if row.startup {
							output = ""
						}
						if !necessityEvaluated(removed, marker) || removed.End.Signal != 9 || string(removed.Stdout) != output || len(removed.Stderr) != 0 {
							t.Fatalf("repeat %d removal did not kill at governed step; profile=%s parameters=%v end=%+v marker=%+v stdout=%q stderr=%q", repeat, rendered, params, removed.End, removed.Markers, removed.Stdout, removed.Stderr)
						}
					}
				})
			}
		})
	}
}

func necessityPaths(t *testing.T, s *necessityScenario) map[string]string {
	t.Helper()
	values := map[string]string{}
	for k, v := range s.inv.Paths.Values {
		if k == "PLANTED" || k == "SUBJECT" || k == "SUBJECT_COPY" {
			values[k] = v
			continue
		}
		path, err := canonical(v, false)
		if err != nil {
			t.Fatal(k, err)
		}
		values[k] = path
	}
	return values
}

func TestMacOSNecessityPrestart(t *testing.T) {
	// QH6: all three pre-marker failures have an unmodified marker-producing
	// control in the same scenario/session. An unexpected missing marker fails.
	binary := necessityBuild(t, false)
	for _, name := range []string{"Exit", "Rejected", "LauncherAllowance"} {
		t.Run(name, func(t *testing.T) {
			s := newNecessityScenario(t, binary, false)
			s.inv.Fast = true
			for repeat := range 25 {
				control := s.run(t, nil, nil, "", "")
				if !necessityEvaluated(control, true) || control.End.Signal != 0 || control.End.Code != 0 || len(control.Stdout) != 0 || len(control.Stderr) != 0 {
					t.Fatalf("repeat %d control=%+v", repeat, control)
				}
				remove, mode := "", name
				if name == "LauncherAllowance" {
					remove = `(allow process-exec* (literal (param "LAUNCHER")) (literal (param "BIN")))`
					mode = ""
				}
				r := s.run(t, nil, nil, remove, mode)
				if !necessityEvaluated(r, false) || len(r.Stdout) != 0 {
					t.Fatalf("repeat %d not evaluated: %+v", repeat, r)
				}
				switch name {
				case "Exit":
					if r.End.Code != 64 || r.End.Signal != 0 {
						t.Fatalf("sandbox-exec usage exit: %+v", r)
					}
				case "Rejected":
					if r.End.Code != 65 || r.End.Signal != 0 || len(r.Stderr) == 0 {
						t.Fatalf("rejected profile: %+v", r)
					}
				case "LauncherAllowance":
					if r.End.Signal != 9 || len(r.Stderr) != 0 {
						t.Fatalf("startup removal: %+v", r)
					}
				}
			}
		})
	}
}

// TCP and Unix decoys intentionally do not accept connections. Refresh their
// owned endpoints between repetitions so a granted connect cannot fill a
// backlog and turn a later repeat into a budget expiry. Images stay unchanged.
func (s *necessityScenario) refreshDecoy(t *testing.T) {
	t.Helper()
	if !s.c.launcher.unchanged() || !s.c.program.unchanged() {
		t.Fatal("private image changed before decoy start")
	}
	if err := s.decoy.close(); err != nil {
		t.Fatal(err)
	}
	s.decoy = nil
	files, err := s.w.PrepareSelfTest()
	if err != nil {
		t.Fatal(err)
	}
	s.files = files
	s.inv = selfTestInvocation(s.inv, files, s.w)
	l, _ := s.c.CopyPaths()
	s.decoy, err = startSelfTestDecoy(l, files.Home, s.w)
	if err != nil {
		t.Fatal(err)
	}
}

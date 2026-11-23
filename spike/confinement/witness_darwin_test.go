//go:build confinementspike && darwin

package confinement

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSilentRefusals(t *testing.T) {
	t.Parallel()
	for _, computation := range []bool{false, true} {
		f := newFixture(t)
		for _, op := range []string{"exception-entitlement", "process-exec-update-label"} {
			runScenario(t, profileName(computation)+"/"+op, func(t *testing.T) {
				repeat(t, func() bool {
					f.args = []string{op}
					return correct(f.invoke(profile(computation), "inventory", false), op+"=0\n")
				})
			})
		}
	}
}

func TestBehaviouralRefusalModifiers(t *testing.T) {
	t.Parallel()
	witnesses := []struct{ op, mode string }{
		{"iokit-get-properties", "properties"}, {"consume-extension", "extension"}, {"mach-task-exception-port-set", "exception-port"}, {"necp-client-open", "necp"},
		{"file-read-xattr", "xattr"}, {"file-link", "link"}, {"file-clone", "clone"},
		{"sysctl-read", "sysctl"}, {"file-lock", "lock"}, {"file-map-executable", "map"}, {"fs-info", "fs"},
		{"process-iopolicy*", "policy"}, {"socket-option-get", "socket-get"},
		{"socket-option-set", "socket-set"}, {"socket-ioctl", "socket-ioctl"},
	}
	for _, computation := range []bool{false, true} {
		for _, w := range witnesses {
			for _, form := range []string{"confined", "positive", "twin", "attribution", "without-kill", "without-deny"} {
				if form == "without-deny" && w.mode != "xattr" {
					continue
				}
				runScenario(t, profileName(computation)+"/"+w.op+"/"+form, func(t *testing.T) {
					t.Parallel()
					f := newFixture(t)

					repeat(t, func() bool {
						root, err := filepath.EvalSymlinks(t.TempDir())
						if w.mode == "link" || w.mode == "clone" {
							// The complete home workspace removes directory and
							// metadata setup denials from these global rules.
							root, err = os.MkdirTemp(f.home(), "link-clone-")
							if err == nil {
								defer os.RemoveAll(root)
							}
						}
						if err != nil {
							t.Fatal("canonical decoy root")
						}
						source := filepath.Join(root, "source")
						writeDecoy(t, source)
						mapped := f
						if w.mode == "map" {
							mapped.freshImages = true
							var release func()
							mapped, release, err = mapped.prepareImages()
							if err != nil {
								t.Fatal("prepare mapped image")
							}
							defer release()
							source = mapped.probe
						}
						if w.mode == "xattr" {
							setup := f
							setup.args = []string{source, source, filepath.Join(root, "unused")}
							if !correct(setup.invoke("", "file-xattr", false), "performed\nfallback\n") {
								t.Fatal("prepare xattr decoy")
							}
						}
						extensionTarget := ""
						if w.mode == "extension" {
							extensionTarget = source
							token := filepath.Join(root, "extension-token")
							issuer := f
							issuer.args = []string{source, token}
							if !correct(issuer.invoke("", "extension-token", false), "issued\n") {
								t.Log("native extension issuance for the decoy failed")
								return false
							}
							source = token
						}
						destination := filepath.Join(root, "destination")
						probe := mapped
						probe.args = []string{source, destination}
						probe.params = []string{"DECLARED=" + source, "SOURCE=" + source, "DESTINATION=" + destination}
						p := profile(computation)
						// Isolate the explicit xattr kill from the default kill: removing
						// this deny must change observed behaviour, not a text comparison.
						p = strings.Replace(p, "(deny default (with send-signal SIGKILL))", "(deny default)", 1)
						// Setup grants isolate the operation being tested from content/path denials.
						p += "\n(allow file-read-data (literal (param \"SOURCE\")))\n(allow file-write* (literal (param \"DESTINATION\")))\n(allow network-outbound)\n"
						if w.mode == "clone" {
							// clonefile needs class-level xattr permission in its control;
							// a source-filtered grant did not suffice.
							p += "\n(allow file-read-xattr)\n"
						}
						if w.mode == "link" {
							// Subtraction from the operation catalog requires each grant
							// for the link control. The probe invokes only a link on decoys.
							p += "\n(allow file-clone file-graft file-map-executable file-read-xattr file-ungraft fs-snapshot-mount)\n"
						}
						if w.mode == "fs" {
							probe.params = append(probe.params, "VOLUME="+filepath.Join(root, "unused-volume"))
						}
						if w.mode == "properties" {
							p += "\n(allow iokit-open-service)\n(allow file-read-data (subpath (param \"SYSTEM_ROOT\")))\n"
						}
						line := "(deny " + w.op + " (with send-signal SIGKILL))"
						if form == "attribution" {
							if w.mode == "sysctl" {
								p += "\n(allow sysctl-read (sysctl-name \"kern.osrelease\"))\n"
							} else {
								p += "\n(allow " + w.op + ")\n"
							}
						}
						if form == "without-kill" {
							p = strings.Replace(p, line, "(deny "+w.op+")", 1)
						}
						if form == "without-deny" {
							p = strings.Replace(p, line, "", 1)
						}
						if form == "positive" {
							p = ""
						}
						before := decoySnapshot(source)
						targetBefore := ""
						if extensionTarget != "" {
							targetBefore = decoySnapshot(extensionTarget)
						}
						r := probe.invoke(p, "witness-"+w.mode, form == "twin")
						unchanged := before == decoySnapshot(source)
						if extensionTarget != "" {
							unchanged = unchanged && targetBefore == decoySnapshot(extensionTarget)
						}
						if form == "confined" {
							return r.killed() && unchanged && decoySnapshot(destination) == "absent"
						}
						if form == "without-kill" {
							return correct(r, "fallback\n") && unchanged && decoySnapshot(destination) == "absent"
						}
						if form == "without-deny" {
							return correct(r, "fallback\n")
						}
						want := "performed\nfallback\n"
						if form == "twin" && w.mode != "sysctl" {
							want = "fallback\n"
						}
						return correct(r, want)
					})
				})
			}
		}
		covered := map[string]bool{"process-info-codesignature": true, "default": true, "process-info*": true, "syscall-unix": true, "file-write*": true, "file-write-unlink": true, "file-read-data": true}
		// Only these operations lack a usable harmless behavioral witness. The list
		// is closed; an additional unwitnessed operation is a test failure.
		structuralOnly := map[string]bool{
			// Own-pid memorystatus getters return EPERM even without confinement.
			"system-memorystatus-control": true,
			// Container-path lookup returns no path for even an unconfined victim.
			"process-info-sandbox-container": true,
			// The host-special-port read survives this deny and cannot witness it.
			"mach-task-special-port*": true,
			// The own-coalition query never succeeds without confinement.
			"coalition-info":        true,
			"exception-entitlement": true, "process-exec-update-label": true,
			// Opening an IOKit user client may invoke hardware or driver behavior;
			// no harmless service-open control has been established.
			"iokit-open-service": true,
			"fs-quota*":          true, "nvram*": true, "asr-parser-enter": true, "darwin-notification-post": true,
			"file-graft": true, "file-ungraft": true, "fs-snapshot-mount": true,
			"isp-command-send": true, "mach-bootstrap": true, "mach-cross-domain-lookup": true,
			"system-necp-client-action": true, "system-privilege": true,
		}
		for _, w := range witnesses {
			covered[w.op] = true
		}
		for _, line := range strings.Split(profile(computation), "\n") {
			if strings.HasPrefix(line, "(deny ") {
				op := strings.Fields(line)[1]
				op = strings.TrimSuffix(op, ")")
				if !covered[op] {
					if !structuralOnly[op] {
						t.Errorf("no behavioral witness for %s", op)
					} else {
						t.Logf("structural only: %s", op)
					}
				}
			}
		}
	}
}

func TestSystemResolversSmoke(t *testing.T) {
	t.Log("smoke test: these kills do not isolate the operation rule")
	t.Parallel()
	f := newFixture(t)
	g := buildGoClean(t, f)
	for _, computation := range []bool{false, true} {
		for _, entry := range []struct {
			name string
			f    fixture
		}{{"c", f}, {"go", g}} {
			for _, confined := range []bool{false, true} {
				runScenario(t, fmt.Sprintf("%s/%s/confined=%v", profileName(computation), entry.name, confined), func(t *testing.T) {
					repeat(t, func() bool {
						p := ""
						if confined {
							p = profile(computation)
						}
						r := entry.f.invoke(p, "resolver", false)
						if confined {
							return r.killed()
						}
						return correct(r, "lookup failed\n")
					})
				})
			}
		}
	}
}

func TestPrebootListingSmoke(t *testing.T) {
	t.Log("smoke test: these kills do not isolate the operation rule")
	t.Parallel()
	f := newFixture(t)
	tree := filepath.Join(f.root, "runtime-decoy")
	if os.MkdirAll(filepath.Join(tree, "allowed"), 0700) != nil {
		t.Fatal("create runtime decoy")
	}
	f.params = []string{"RUNTIME_DECOY=" + filepath.Join(tree, "allowed")}
	f.args = []string{tree}
	for _, computation := range []bool{false, true} {
		for _, form := range []string{"confined", "twin", "positive"} {
			runScenario(t, profileName(computation)+"/"+form, func(t *testing.T) {
				repeat(t, func() bool {
					p := profile(computation) + `
(allow file-read-data (subpath (param "RUNTIME_DECOY")))
`
					if form == "positive" {
						p = ""
					}
					r := f.invoke(p, "preboot", form == "twin")
					if form == "confined" {
						return r.killed()
					}
					return correct(r, "performed\nfallback\n")
				})
			})
		}
	}
}

func TestExecutableScriptPipelines(t *testing.T) {
	t.Parallel()
	f := computationFixture(t)
	for _, entry := range []struct{ tool, command, want string }{
		{"awk", "printf 'b\\na\\n' | awk '{print $1}'", "b\na\n"},
		{"sed", "printf 'b\\na\\n' | sed 's/b/c/'", "c\na\n"},
		{"sort", "printf 'b\\na\\n' | sort", "a\nb\n"},
		{"tr", "printf 'b\\na\\n' | tr a z", "b\nz\n"},
	} {
		runScenario(t, entry.tool, func(t *testing.T) {
			script := filepath.Join(f.root, "script-"+entry.tool)
			if os.WriteFile(script, []byte("#!/bin/sh\nset -eu\n"+entry.command+"\n"), 0700) != nil {
				t.Fatal("write computation")
			}
			probe := f
			probe.probe = script
			repeat(t, func() bool { return correct(probe.invoke(profile(true), "unused", false), entry.want) })
		})
	}
}

func TestSystemToolBoundariesSmoke(t *testing.T) {
	t.Log("smoke test: these kills do not isolate the operation rule")
	t.Parallel()
	f := computationFixture(t)
	subject := filepath.Join(f.root, "subject")
	if r := run("/usr/bin/clang", []string{"-Wall", "-Wextra", "-Werror", "-o", subject, "testdata/programs/subject.c"}, f.env, ""); r.err != nil {
		t.Fatal("build subject")
	}
	outside := filepath.Join(f.root, "outside")
	writeDecoy(t, outside)
	for _, scenario := range []string{"read", "write", "subject-read", "subject-execute", "network", "http"} {
		for _, form := range []string{"confined", "positive", "twin"} {
			runScenario(t, scenario+"/"+form, func(t *testing.T) {
				repeat(t, func() bool {
					probe := f
					target := outside
					var quiet func() bool
					if scenario == "network" || scenario == "http" {
						var closeDecoy func()
						probe, quiet, closeDecoy = networkDecoy(t, f, "tcp")
						defer closeDecoy()
						port := probe.args[0]
						if scenario == "network" {
							probe.args = []string{"/usr/bin/nc", "-n", "-z", "-w", "1", "127.0.0.1", port}
						} else {
							probe.args = []string{"/usr/bin/curl", "--disable", "--noproxy", "*", "--silent", "--max-time", "1", "http://127.0.0.1:" + port + "/"}
						}
					} else {
						if strings.HasPrefix(scenario, "subject-") {
							target = subject
						}
						switch scenario {
						case "read", "subject-read":
							probe.args = []string{"/bin/cat", target}
						case "subject-execute":
							probe.args = []string{target, "clean"}
						case "write":
							probe.args = []string{"/bin/sh", "-c", `printf other > "$1"`, "write", target}
						}
					}
					if form == "twin" {
						switch scenario {
						case "read", "subject-read":
							probe.args = []string{"/bin/cat", filepath.Join(f.root, "declared")}
						case "write":
							probe.args = []string{"/bin/sh", "-c", `printf other > "$1"`, "write", filepath.Join(f.root, "scratch", "target")}
						case "subject-execute":
							probe.args = []string{f.probe, "clean"}
						case "network", "http":
							probe.args = []string{"/bin/sh", "-c", "exit 0"}
						}
					}
					if scenario == "write" {
						writeDecoy(t, target)
					}
					before := decoySnapshot(target)
					p := profile(true)
					if form == "positive" {
						p = ""
					}
					r := probe.invoke(p, "system-child", false)
					unchanged := before == decoySnapshot(target)
					untouched := true
					if quiet != nil {
						untouched = quiet()
					}
					if form == "confined" {
						return childKilled(r) && unchanged && untouched
					}
					if form == "twin" {
						want := "child-status=0\n"
						if scenario == "read" || scenario == "subject-read" {
							want = "decoy" + want
						}
						if scenario == "subject-execute" {
							want = "clean\n" + want
						}
						return correct(r, want) && unchanged && untouched
					}
					if quiet != nil {
						// A timeout from the HTTP tool is its own exit, and the listener must
						// observe the connection. It is never accepted as a confinement kill.
						want := "child-status=0\n"
						if scenario == "http" {
							want = "child-status=7168\n"
						}
						if scenario == "network" {
							diagnostic := "confined\nConnection to 127.0.0.1 port " + probe.args[len(probe.args)-1] + " [tcp/*] succeeded!\n"
							return r.err == nil && !r.timedOut && r.status.Exited() && r.status.ExitStatus() == 0 && r.output == want && r.diagnostic == diagnostic && !untouched
						}
						return correct(r, want) && !untouched
					}
					if scenario == "write" {
						return correct(r, "child-status=0\n") && !unchanged
					}
					if scenario == "subject-execute" {
						return correct(r, "performed\nfallback\nchild-status=0\n")
					}
					data, err := os.ReadFile(target)
					return err == nil && correct(r, string(data)+"child-status=0\n")
				})
			})
		}
	}
}

func TestSystemHTTPClientSmoke(t *testing.T) {
	t.Parallel()
	t.Log("smoke test: a killed HTTP tool does not prove the network rule")
	f := newFixture(t)
	for _, computation := range []bool{false, true} {
		for _, confined := range []bool{false, true} {
			runScenario(t, fmt.Sprintf("%s/confined=%v", profileName(computation), confined), func(t *testing.T) {
				repeat(t, func() bool {
					probe, quiet, closeDecoy := networkDecoy(t, f, "tcp")
					defer closeDecoy()
					port := probe.args[0]
					probe.params = append(probe.params, "HTTP_TOOL=/usr/bin/curl")
					probe.args = []string{"/usr/bin/curl", "--disable", "--noproxy", "*", "--silent", "--max-time", "1", "http://127.0.0.1:" + port + "/"}
					p := ""
					if confined {
						p = profile(computation) + `
(allow process-exec* file-read-data (literal (param "HTTP_TOOL")))
`
					}
					r := probe.invoke(p, "replace-tool", false)
					untouched := quiet()
					if confined {
						return r.killed() && untouched
					}
					return r.marked && !r.timedOut && r.status.Exited() && r.status.ExitStatus() == 28 && r.output == "" && !untouched
				})
			})
		}
	}
}

func TestComputationTerminalOpen(t *testing.T) {
	t.Parallel()
	f := computationFixture(t)
	runScenario(t, "open-tty", func(t *testing.T) {
		repeat(t, func() bool { return correct(f.invoke(profile(true), "terminal-open", false), "terminal absent\n") })
	})
}

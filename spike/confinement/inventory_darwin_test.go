//go:build confinementspike && darwin

package confinement

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The platform compiler's operation table supplies this reviewed catalog.
// Firmware, mount, quota, privilege and machine administration are queried only.
func TestOperationInventory(t *testing.T) {
	f := newFixture(t)
	data, err := os.ReadFile("testdata/programs/operations.txt")
	if err != nil {
		t.Fatal("read operation catalog")
	}
	operations := strings.Fields(string(data))
	if len(operations) == 0 {
		t.Fatal("empty operation inventory")
	}
	for _, computation := range []bool{false, true} {
		for _, op := range operations {
			runScenario(t, profileName(computation)+"/"+op, func(t *testing.T) {
				repeat(t, func() bool {
					probe := f
					probe.args = []string{op}
					want := "0"
					// Each open class is required by startup, metadata lookup or Go's numeric
					// hardware queries. Unsupported kill modifiers are deliberately excluded.
					switch op {
					case "dynamic-code-generation", "exception-entitlement", "process-exec-update-label", "system-info", "file-read-metadata", "process-codesigning", "sandbox-check", "syscall-mach", "syscall-mig", "syscall-unix", "file-test-existence", "system-fcntl", "system-mac-syscall", "mach-derive-port", "sysctl-read", "mach-priv-host-port":
						want = "1"
					case "process-fork":
						if computation {
							want = "1"
						}
					}
					r := probe.invoke(profile(computation), "inventory", false)
					if want == "0" && r.killed() {
						return true
					}
					return correct(r, op+"="+want+"\n")
				})
			})
		}
	}
	t.Log("firmware, mount, quota, privilege and machine administration: queried, not attempted")
}

// Entitlements must not bypass the file boundary even though the platform
// rejects a kill modifier on its entitlement operation.
func TestEntitlementFileBoundary(t *testing.T) {
	f := selfTestFixture(t)
	entitlements := filepath.Join(f.root, "entitlements.plist")
	data := `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>com.apple.security.temporary-exception.files.absolute-path.read-only</key><array><string>` + filepath.Dir(f.args[0]) + `/</string></array></dict></plist>`
	if os.WriteFile(entitlements, []byte(data), 0600) != nil {
		t.Fatal("write temporary exception entitlement")
	}
	if r := run("/usr/bin/codesign", []string{"--force", "--sign", "-", "--entitlements", entitlements, f.probe}, f.env, f.root); r.err != nil {
		t.Fatal("sign exception probe")
	}
	for _, computation := range []bool{false, true} {
		for _, form := range []string{"confined", "twin", "positive", "without-kill"} {
			runScenario(t, profileName(computation)+"/"+form, func(t *testing.T) {
				repeat(t, func() bool {
					probe := f
					good := filepath.Join(f.root, "home", "state")
					probe.args = []string{f.args[0], good, filepath.Join(f.root, "unused")}
					probe.params = []string{"DECLARED=" + good}
					p := profile(computation)
					if form == "positive" {
						p = ""
					}
					if form == "without-kill" {
						p = strings.ReplaceAll(p, "(with send-signal SIGKILL)", "")
					}
					before := decoySnapshot(f.args[0])
					r := probe.invoke(p, "file-read", form == "twin")
					if before != decoySnapshot(f.args[0]) {
						return false
					}
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
	t.Log("open limit: exception-entitlement and process-exec-update-label reject send-signal; neither has a plain deny")
}

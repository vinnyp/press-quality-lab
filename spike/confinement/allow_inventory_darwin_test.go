//go:build confinementspike && darwin

package confinement

import (
	"path/filepath"
	"strings"
	"testing"
)

// These literal inventories require review whenever a grant changes.
const reviewedSubjectAllows = `(allow mach-task-special-port-set (target self))
(allow fs-info (literal (param "VOLUME")))
(allow file-lock (subpath (param "HOME")))
(allow process-exec* (literal (param "LAUNCHER")) (literal (param "BIN")))
(allow mach-priv-host-port)
(allow sysctl-read (sysctl-name "kern.hostname") (sysctl-name "kern.maxfilesperproc") (sysctl-name "kern.osproductversion") (sysctl-name "kern.iossupportversion") (sysctl-name "kern.bootargs") (sysctl-name "hw.ncpu") (sysctl-name "hw.optional.arm.FEAT_SB") (sysctl-name "hw.optional.arm.FEAT_DIT") (sysctl-name "security.mac.lockdown_mode_state") (sysctl-name "kern.osvariant_status") (sysctl-name "hw.pagesize_compat") (sysctl-name "hw.optional.armv8_crc32") (sysctl-name "hw.optional.armv8_1_atomics") (sysctl-name "hw.optional.armv8_2_sha512") (sysctl-name "hw.optional.armv8_2_sha3"))
(allow process-codesigning* (target self))
(allow file-read-metadata)
(allow file-ioctl (literal (param "TRACE_DEVICE")))
(allow file-read-data (literal (param "BIN_PARENT")) (literal (param "LAUNCHER_PARENT")))
(allow file-read-data file-write-data (literal (param "NULL_DEVICE")))
(allow file-read-data (literal (param "RANDOM_DEVICE")) (literal (param "ZERO_DEVICE")))
(allow file-write-data (literal (param "STDOUT_DEVICE")) (literal (param "STDERR_DEVICE")))
(allow file-read-data (literal (param "ZONEFILE")))
(allow file-read-data (literal (param "ZONE_RULES")))
(allow file-read-data (literal (param "BIN")) (literal (param "LAUNCHER")))
(allow file-read* (literal (param "ROOT")))
(allow file-read-data (subpath (param "RUNTIME")))
(allow file-read-data (literal (param "TRACE_DEVICE")))
(allow file-write-data (literal (param "TRACE_DEVICE")))
(allow mach-lookup (global-name "com.apple.system.opendirectoryd.libinfo"))
(allow mach-lookup (global-name "com.apple.system.notification_center"))
(allow mach-lookup (global-name "com.apple.logd"))
(allow ipc-posix-shm-read* (ipc-posix-name "apple.shm.notification_center"))
(allow process-info* (target self))
(allow file-read* (subpath (param "HOME")))
(allow file-write* (subpath (param "HOME")))
(allow mach-lookup (global-name "com.apple.system.opendirectoryd.membership"))
(allow system-info)`
const reviewedComputationAllows = `(allow mach-task-special-port-set (target same-sandbox))
(allow fs-info (literal (param "VOLUME")))
(allow file-lock (subpath (param "HOME")))
(allow process-exec* (literal (param "LAUNCHER")) (literal (param "BIN")))
(allow mach-priv-host-port)
(allow sysctl-read (sysctl-name "kern.hostname") (sysctl-name "kern.maxfilesperproc") (sysctl-name "kern.osproductversion") (sysctl-name "kern.iossupportversion") (sysctl-name "kern.bootargs") (sysctl-name "hw.ncpu") (sysctl-name "hw.optional.arm.FEAT_SB") (sysctl-name "hw.optional.arm.FEAT_DIT") (sysctl-name "security.mac.lockdown_mode_state") (sysctl-name "kern.osvariant_status") (sysctl-name "hw.pagesize_compat") (sysctl-name "hw.optional.armv8_crc32") (sysctl-name "hw.optional.armv8_1_atomics") (sysctl-name "hw.optional.armv8_2_sha512") (sysctl-name "hw.optional.armv8_2_sha3"))
(allow process-codesigning* (target same-sandbox))
(allow file-read-metadata)
(allow file-ioctl (literal (param "TRACE_DEVICE")))
(allow file-read-data (literal (param "BIN_PARENT")) (literal (param "LAUNCHER_PARENT")))
(allow file-read-data file-write-data (literal (param "NULL_DEVICE")))
(allow file-read-data (literal (param "RANDOM_DEVICE")) (literal (param "ZERO_DEVICE")))
(allow file-write-data (literal (param "STDOUT_DEVICE")) (literal (param "STDERR_DEVICE")))
(allow file-read-data (literal (param "ZONEFILE")))
(allow file-read-data (literal (param "ZONE_RULES")))
(allow file-read-data (literal (param "BIN")) (literal (param "LAUNCHER")))
(allow file-read* (literal (param "ROOT")))
(allow file-read-data (subpath (param "RUNTIME")))
(allow file-read-data (literal (param "TRACE_DEVICE")))
(allow file-write-data (literal (param "TRACE_DEVICE")))
(allow process-fork)
(allow signal (target same-sandbox))
(allow mach-lookup (global-name "com.apple.system.opendirectoryd.libinfo"))
(allow mach-lookup (global-name "com.apple.system.notification_center"))
(allow mach-lookup (global-name "com.apple.logd"))
(allow ipc-posix-shm-read* (ipc-posix-name "apple.shm.notification_center"))
(allow process-info* (target same-sandbox))
(allow sysctl-read (sysctl-name "kern.ngroups"))
(allow sysctl-read (sysctl-name "hw.memsize") (sysctl-name "hw.pagesize"))
(allow process-exec* file-read-data (subpath (param "BIN_DIR")) (subpath (param "USR_BIN_DIR")))
(allow file-read-data file-write-data (literal (param "TTY_DEVICE")))
(allow file-read-data (literal (param "DECLARED")))
(allow file-read-data (literal (param "SEED")))
(allow file-read* (subpath (param "HOME")))
(allow file-write* (subpath (param "HOME")))
(allow mach-lookup (global-name "com.apple.system.opendirectoryd.membership"))
(allow system-info)`

func TestExactAllowInventory(t *testing.T) {
	t.Parallel()
	for _, computation := range []bool{false, true} {
		if !profileLinesReviewed(profile(computation)) {
			t.Error("profile contains a hidden or indented form")
		}
		var lines []string
		for _, line := range strings.Split(profile(computation), "\n") {
			if strings.HasPrefix(line, "(allow ") {
				lines = append(lines, line)
			}
		}
		want := reviewedSubjectAllows
		if computation {
			want = reviewedComputationAllows
		}
		if strings.Join(lines, "\n") != want {
			t.Errorf("%s allowance inventory changed", profileName(computation))
		}
	}
}

// Parameter values widen a grant without changing its text, and a violator
// may target only decoys, so each fixed system path is pinned to a reviewed
// literal. Widening RUNTIME to all of Preboot fails here.
func TestExactSystemParameters(t *testing.T) {
	t.Parallel()
	reviewed := map[string]string{
		"ROOT": "/", "SYSTEM_ROOT": "/System",
		"RUNTIME": "/System/Volumes/Preboot/Cryptexes", "BIN_DIR": "/bin", "USR_BIN_DIR": "/usr/bin",
		"NULL_DEVICE": "/dev/null", "RANDOM_DEVICE": "/dev/urandom", "ZERO_DEVICE": "/dev/zero",
		"STDOUT_DEVICE": "/dev/fd/1", "STDERR_DEVICE": "/dev/fd/2", "TTY_DEVICE": "/dev/tty",
		"TRACE_DEVICE": "/dev/dtracehelper",
	}
	for key, source := range map[string]string{"ZONEFILE": "/usr/share/zoneinfo/America/New_York", "ZONE_RULES": "/usr/share/zoneinfo/posixrules"} {
		resolved, err := filepath.EvalSymlinks(source)
		if err != nil {
			t.Fatalf("resolve %s", source)
		}
		reviewed[key] = resolved
	}
	fixtureKeys := map[string]bool{}
	for _, key := range strings.Fields("BIN BIN_PARENT LAUNCHER LAUNCHER_PARENT PLANTED SUBJECT SUBJECT_COPY VOLUME HOME DECLARED SEED CREATE_TARGET SELFTEST_CREATE SELFTEST_MAP SELFTEST_TARGET DECOY_DIRECTORY DIRECTORY DESTINATION ROOT_DEST TARGET HTTP_TOOL RUNTIME_DECOY SOURCE") {
		fixtureKeys[key] = true
	}
	seen := map[string]bool{}
	for _, parameter := range newFixture(t).parameters() {
		key, value, _ := strings.Cut(parameter, "=")
		if !fixtureKeys[key] {
			if _, ok := reviewed[key]; !ok {
				t.Errorf("unreviewed parameter key %s", key)
			}
		}
		if want, ok := reviewed[key]; ok {
			seen[key] = true
			if value != want {
				t.Errorf("parameter %s is %q, reviewed value %q", key, value, want)
			}
		}
	}
	for key := range reviewed {
		if !seen[key] {
			t.Errorf("reviewed parameter %s is missing", key)
		}
	}
}

// Each profile form occupies one unindented line so inventories and removal
// witnesses cannot overlook a second form hidden behind a reviewed one.
func profileLinesReviewed(p string) bool {
	for _, line := range strings.Split(p, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, ";") {
			continue
		}
		if line != trimmed || !strings.HasPrefix(line, "(") {
			return false
		}
		depth, forms := 0, 0
		quoted, escaped := false, false
		for _, c := range line {
			if escaped {
				escaped = false
				continue
			}
			if quoted && c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				quoted = !quoted
				continue
			}
			if quoted {
				continue
			}
			if c == ';' {
				break
			}
			if c == '(' {
				if depth == 0 {
					forms++
				}
				depth++
			}
			if c == ')' {
				depth--
				if depth < 0 {
					return false
				}
			}
		}
		if depth != 0 || quoted || forms != 1 {
			return false
		}
	}
	return true
}

func TestHiddenProfileForms(t *testing.T) {
	for _, computation := range []bool{false, true} {
		p := profile(computation)
		for _, mutant := range []string{
			strings.Replace(p, "(deny default (with send-signal SIGKILL))", "(deny default (with send-signal SIGKILL)) (allow default)", 1),
			strings.Replace(p, "(allow mach-priv-host-port)", " (allow mach-priv-host-port)", 1),
		} {
			runScenario(t, profileName(computation), func(t *testing.T) {
				repeat(t, func() bool { executed.Add(1); return profileLinesReviewed(p) && !profileLinesReviewed(mutant) })
			})
		}
	}
}

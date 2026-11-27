//go:build necessity_test

package confine

import (
	"fmt"
	"strings"
	"testing"
)

type necessityAllowance struct {
	name, prefix, operation, parameter string
	startup, computationOnly           bool
}

// One row per QH11 allowance line. Combined path grants get separate witness
// rows below, so e.g. RANDOM_DEVICE cannot stand in for ZERO_DEVICE.
var necessityAllowances = []necessityAllowance{
	{name: "BootstrapPort", prefix: "(allow mach-task-special-port-set ", startup: true},
	{name: "Volume", prefix: "(allow fs-info ", operation: "volume", parameter: "HOME"},
	{name: "Lock", prefix: "(allow file-lock ", operation: "lock", parameter: "HOME"},
	{name: "Exec", prefix: "(allow process-exec* (literal", startup: true},
	{name: "HostPort", prefix: "(allow mach-priv-host-port)", startup: true},
	{name: "StartupSysctl", prefix: `(allow sysctl-read (sysctl-name "kern.hostname")`, startup: true},
	{name: "Codesigning", prefix: "(allow process-codesigning* ", startup: true},
	{name: "Metadata", prefix: "(allow file-read-metadata)", startup: true},
	{name: "TraceIoctl", prefix: "(allow file-ioctl ", startup: true},
	{name: "ImageParents", prefix: `(allow file-read-data (literal (param "BIN_PARENT"))`, startup: true},
	{name: "Null", prefix: `(allow file-read-data file-write-data (literal (param "NULL_DEVICE")))`, operation: "null", parameter: "NULL_DEVICE"},
	{name: "Random", prefix: `(allow file-read-data (literal (param "RANDOM_DEVICE"))`, operation: "random", parameter: "RANDOM_DEVICE"},
	{name: "Zero", prefix: `(allow file-read-data (literal (param "RANDOM_DEVICE"))`, operation: "zero", parameter: "ZERO_DEVICE"},
	{name: "Stdout", prefix: `(allow file-write-data (literal (param "STDOUT_DEVICE"))`, operation: "stream", parameter: "STDOUT_DEVICE"},
	{name: "Stderr", prefix: `(allow file-write-data (literal (param "STDOUT_DEVICE"))`, operation: "stream", parameter: "STDERR_DEVICE"},
	{name: "Zone", prefix: `(allow file-read-data (literal (param "ZONEFILE"))`, operation: "read", parameter: "ZONEFILE"},
	{name: "ZoneRules", prefix: `(allow file-read-data (literal (param "ZONE_RULES"))`, operation: "read", parameter: "ZONE_RULES"},
	{name: "Services", prefix: `(allow file-read-data (literal (param "SERVICES"))`, operation: "read", parameter: "SERVICES"},
	{name: "Protocols", prefix: `(allow file-read-data (literal (param "SERVICES"))`, operation: "read", parameter: "PROTOCOLS"},
	{name: "ImageReads", prefix: `(allow file-read-data (literal (param "BIN"))`, startup: true},
	{name: "Root", prefix: "(allow file-read* (literal", startup: true},
	{name: "Runtime", prefix: `(allow file-read-data (subpath (param "RUNTIME"))`, startup: true},
	{name: "TraceRead", prefix: `(allow file-read-data (literal (param "TRACE_DEVICE"))`, startup: true},
	{name: "TraceWrite", prefix: `(allow file-write-data (literal (param "TRACE_DEVICE"))`, startup: true},
	{name: "DirectoryService", prefix: `(allow mach-lookup (global-name "com.apple.system.opendirectoryd.libinfo"))`, operation: "service", parameter: "com.apple.system.opendirectoryd.libinfo"},
	{name: "NotificationService", prefix: `(allow mach-lookup (global-name "com.apple.system.notification_center"))`, operation: "service", parameter: "com.apple.system.notification_center"},
	{name: "LogService", prefix: `(allow mach-lookup (global-name "com.apple.logd"))`, operation: "service", parameter: "com.apple.logd"},
	{name: "NotificationMemory", prefix: "(allow ipc-posix-shm-read* ", operation: "shm", parameter: "apple.shm.notification_center"},
	{name: "OwnProcess", prefix: "(allow process-info* ", operation: "own_process"},
	{name: "HomeRead", prefix: "(allow file-read* (subpath", operation: "read", parameter: "HOME_FILE"},
	{name: "HomeWrite", prefix: "(allow file-write* (subpath", operation: "write", parameter: "HOME_FILE"},
	{name: "MembershipService", prefix: `(allow mach-lookup (global-name "com.apple.system.opendirectoryd.membership"))`, operation: "service", parameter: "com.apple.system.opendirectoryd.membership"},
	{name: "SystemInfo", prefix: "(allow system-info)", operation: "system_info"},
	{name: "Fork", prefix: "(allow process-fork)", operation: "child", computationOnly: true},
	{name: "ChildSignal", prefix: "(allow signal ", operation: "child_signal", computationOnly: true},
	{name: "Groups", prefix: `(allow sysctl-read (sysctl-name "kern.ngroups"))`, operation: "groups", computationOnly: true},
	{name: "Memory", prefix: `(allow sysctl-read (sysctl-name "hw.memsize")`, operation: "memory", computationOnly: true},
	{name: "PageSize", prefix: `(allow sysctl-read (sysctl-name "hw.memsize")`, operation: "pagesize", computationOnly: true},
	{name: "SystemTools", prefix: "(allow process-exec* file-read-data ", operation: "child", computationOnly: true},
	{name: "TTY", prefix: `(allow file-read-data file-write-data (literal (param "TTY_DEVICE")))`, operation: "tty", parameter: "TTY_DEVICE", computationOnly: true},
	{name: "Declared", prefix: `(allow file-read-data (literal (param "DECLARED_1")))`, operation: "read", parameter: "DECLARED", computationOnly: true},
	{name: "Seed", prefix: `(allow file-read-data (literal (param "SEED_1")))`, operation: "read", parameter: "SEED", computationOnly: true},
}

func necessityAllowanceLine(profile, prefix string) (string, error) {
	var matches []string
	for _, line := range strings.Split(profile, "\n") {
		if strings.HasPrefix(line, prefix) {
			matches = append(matches, line)
		}
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("allowance matches %d lines: %s", len(matches), prefix)
	}
	return matches[0], nil
}

func TestNecessityAllowanceCoverage(t *testing.T) {
	// QH11, QH15, QH16: every rendered allowance has a reviewed witness, in
	// both variants, including each expanded declared/seed grant.
	for _, computation := range []bool{false, true} {
		declared, seed := 0, 0
		if computation {
			declared, seed = 1, 1
		}
		profile := render(computation, false, 0, declared, seed)
		covered := map[string]bool{}
		for _, row := range necessityAllowances {
			if row.computationOnly && !computation {
				continue
			}
			line, err := necessityAllowanceLine(profile, row.prefix)
			if err != nil {
				t.Fatal(row.name, err)
			}
			if row.startup && row.operation != "" || !row.startup && row.operation == "" {
				t.Fatal("missing witness", row.name)
			}
			covered[line] = true
		}
		for _, line := range strings.Split(profile, "\n") {
			if strings.HasPrefix(line, "(allow ") && !covered[line] {
				t.Errorf("unwitnessed allowance: %s", line)
			}
		}
	}
}

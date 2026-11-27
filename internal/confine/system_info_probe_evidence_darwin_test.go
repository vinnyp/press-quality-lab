//go:build darwin && necessity_test && macos_evidence

package confine

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestMacOSSystemInfoProbe(t *testing.T) {
	// Discovery only: no claim of allowance necessity, and no rule verdict.
	binary := necessityBuild(t, true)
	for _, computation := range []bool{false, true} {
		profile := "Subject"
		if computation {
			profile = "Computation"
		}
		s := newNecessityScenario(t, binary, computation)
		candidates := []string{"gethostuuid", "getattrlist", "getattrlistbulk", "statfs", "fstatfs", "getfsstat", "fullfsync", "barrierfsync", "setlk", "temp_dir", "cache_dir", "host_info", "host_statistics", "proc_pidinfo", "getrusage", "uname", "getifaddrs"}
		// Discover every allowed name from this profile, including computation-only
		// SQLite/libSystem sysctls. Each read gets its own process and attempt marker.
		rendered := render(computation, false, 0, 0, 0)
		names := regexp.MustCompile(`sysctl-name "([^"]+)"`).FindAllStringSubmatch(rendered, -1)
		for _, name := range names {
			candidates = append(candidates, "sysctl_"+name[1])
		}
		if computation {
			candidates = append(candidates, "sqlite")
		}
		t.Log("profile candidate variant outcome result")
		for i, op := range candidates {
			for _, variant := range []string{"unmodified", "removed"} {
				path := s.files.Home
				switch op {
				case "fstatfs", "fullfsync", "barrierfsync", "setlk":
					path = filepath.Join(s.files.Home, fmt.Sprintf("file_%d_%s", i, variant))
					if err := os.WriteFile(path, []byte("probe\n"), 0600); err != nil {
						t.Logf("%s %s %s setup_error=%v", profile, op, variant, err)
						continue
					}
				case "sqlite":
					path = filepath.Join(s.files.Home, "probe_"+variant+".db")
				}
				remove := ""
				if variant == "removed" {
					remove = "(allow system-info)"
				}
				r := s.run(t, []string{SelfTestMode, "u16_system_probe", op, path}, nil, remove, "")
				attempt := "attempt_" + op + "\n"
				outcome := "unevaluated"
				if r.Markers.Confinement && strings.HasPrefix(string(r.Stdout), attempt) {
					tail := strings.TrimPrefix(string(r.Stdout), attempt)
					if strings.HasPrefix(tail, "returned ") || strings.Contains(tail, "\nreturned ") {
						outcome = "returned"
					} else if tail == "" && r.End.Signal == 9 {
						outcome = "killed"
					}
				}
				t.Logf("%s %s %s %s end=%+v stdout=%q stderr=%q", profile, op, variant, outcome, r.End, r.Stdout, r.Stderr)
			}
		}
	}
}

//go:build necessity_test

package confine

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/observe"
)

type necessityGrant struct {
	name, operation, rule        string
	programs                     []string
	computationOnly, subjectOnly bool
}

// These are operations, not one-program assumptions. KERN_PROCARGS2 (argv
// and environment) and KERN_PROC_PID all use process-info-pidinfo on Darwin.
// network-inbound joins {bind, listen}; network-bind alone permits only bind.
var necessityGrants = []necessityGrant{
	{name: "TCP", operation: "network-outbound TCP", rule: `(allow network-outbound (remote tcp "localhost:*"))`, programs: []string{"tcp"}},
	{name: "Unix", operation: "network-outbound Unix", rule: `(allow network-outbound (remote unix-socket))`, programs: []string{"unix"}},
	{name: "UDP", operation: "network-outbound UDP", rule: `(allow network-outbound (remote udp "localhost:*"))`, programs: []string{"udp"}},
	{name: "Bind", operation: "network-bind", rule: `(allow network-bind)`, programs: []string{"bind"}},
	{name: "Listen", operation: "network-inbound", rule: `(allow network-inbound)`, programs: []string{"bind", "listen"}},
	{name: "Read", operation: "file-read-data outside HOME", rule: `(allow file-read-data (literal (param "U16_OUTSIDE")))`, programs: []string{"read"}},
	{name: "Map", operation: "file-map-executable private BIN", rule: `(allow file-map-executable (literal (param "BIN")))`, programs: []string{"map"}},
	{name: "Write", operation: "file-write-data outside HOME", rule: `(allow file-write-data (literal (param "U16_OUTSIDE")))`, programs: []string{"write"}},
	{name: "Create", operation: "file-write-create outside HOME", rule: `(allow file-write-create (literal (param "U16_CREATE")))`, programs: []string{"create"}},
	{name: "Service", operation: "mach-lookup decoy", rule: `(allow mach-lookup (global-name "org.pql.selftest.decoy"))`, programs: []string{"service"}},
	{name: "Sysctl", operation: "sysctl-read kern.osrelease", rule: `(allow sysctl-read (sysctl-name "kern.osrelease"))`, programs: []string{"sysctl"}},
	{name: "ProcessInfo", operation: "process-info-pidinfo: KERN_PROCARGS2 and KERN_PROC_PID", rule: `(allow process-info-pidinfo)`, programs: []string{"arguments", "environment", "process"}},
	{name: "Signal", operation: "signal", rule: `(allow signal)`, programs: []string{"signal"}},
	{name: "Codesign", operation: "process-legacy-codesigning-status-get", rule: `(allow process-legacy-codesigning-status-get)`, programs: []string{"codesign"}},
	{name: "Priority", operation: "system-sched", rule: `(allow system-sched)`, programs: []string{"priority"}},
	{name: "Resource", operation: "process-info-rusage", rule: `(allow process-info-rusage)`, programs: []string{"resource"}},
	{name: "Child", operation: "process-fork", rule: `(allow process-fork)`, programs: []string{"child"}, subjectOnly: true},
	{name: "Setsid", operation: "SYS_setsid", rule: `(allow syscall-unix (syscall-number SYS_setsid))`, programs: []string{"setsid"}, computationOnly: true},
	{name: "Setpgid", operation: "SYS_setpgid", rule: `(allow syscall-unix (syscall-number SYS_setpgid))`, programs: []string{"setpgid"}, computationOnly: true},
}

func necessityProfile(base string, grant *necessityGrant, remove string) (string, error) {
	if grant != nil && remove != "" {
		return "", fmt.Errorf("two mutations")
	}
	if grant != nil {
		return base + "\n" + grant.rule + "\n", nil
	}
	if remove != "" {
		line := remove + "\n"
		if strings.Count(base, line) != 1 {
			return "", fmt.Errorf("allowance must occur exactly once: %s", remove)
		}
		return strings.Replace(base, line, "", 1), nil
	}
	return base, nil
}

func necessityEvaluated(r observe.Run, marker bool) bool {
	return r.Markers.Confinement == marker && !r.Markers.CouldNotStart && !r.CopyChanged && !r.End.EngineKilled && !r.End.BudgetExpired && !r.End.MemoryKilled && !r.End.CodeSigningKilled && r.End.ExitDetail == 0 && !r.StdoutCut && !r.StderrCut && !r.OutputCutAfterExit
}

func necessityProblems(names []string, runs []observe.Run) ([]string, error) {
	if len(names) != len(runs) {
		return nil, fmt.Errorf("missing runs")
	}
	problems := []string{}
	for i, name := range names {
		r := runs[i]
		if !necessityEvaluated(r, true) {
			return nil, fmt.Errorf("not evaluated: %s", name)
		}
		p := SelfTestProgram{Name: name, Killed: name != "clean" && name != "allowed_child", Output: "clean_value\n"}
		if name == "allowed_child" {
			p.Name = "child"
			p.Output = "attempt_child\nchild_ok\n"
		}
		if !SelfTestProgramPassed(p, r) {
			problems = append(problems, p.Name)
		}
	}
	return problems, nil
}

func TestNecessityProfileLastGrant(t *testing.T) {
	// QH9, QH11: a single grant follows the unchanged production template.
	base := render(false, false, 0, 0, 0)
	for _, g := range necessityGrants {
		got, err := necessityProfile(base, &g, "")
		if err != nil || got != base+"\n"+g.rule+"\n" {
			t.Fatalf("%s: %q %v", g.name, got, err)
		}
	}
	line := `(allow file-lock (subpath (param "HOME")))`
	got, err := necessityProfile(base, nil, line)
	if err != nil || got != strings.Replace(base, line+"\n", "", 1) {
		t.Fatal("removal changed other rules", err)
	}
	if _, err := necessityProfile(base, nil, line+"missing"); err == nil {
		t.Fatal("missing allowance accepted")
	}
}

func TestNecessityNotEvaluated(t *testing.T) {
	// QH6, QH9: missing markers and unevaluated repeats never become problems
	// attributed to a permitted operation, nor a passing refusal.
	good := observe.Run{Markers: observe.Markers{Confinement: true}, Stdout: []byte("attempt_read\n"), End: observe.End{Signal: 9}}
	names := []string{"read"}
	got, err := necessityProblems(names, []observe.Run{good})
	if err != nil || len(got) != 0 {
		t.Fatal("control", got, err)
	}
	for _, row := range []struct {
		name   string
		change func(*observe.Run)
	}{
		{"Marker", func(r *observe.Run) { r.Markers.Confinement = false }},
		{"Budget", func(r *observe.Run) { r.End.BudgetExpired = true }},
		{"EngineKill", func(r *observe.Run) { r.End.EngineKilled = true }},
		{"Copy", func(r *observe.Run) { r.CopyChanged = true }},
		{"Truncated", func(r *observe.Run) { r.StdoutCut = true }},
		{"Start", func(r *observe.Run) { r.Markers.CouldNotStart = true }},
		{"Memory", func(r *observe.Run) { r.End.MemoryKilled = true }},
		{"Signature", func(r *observe.Run) { r.End.CodeSigningKilled = true }},
		{"Detail", func(r *observe.Run) { r.End.ExitDetail = 0x10000 }},
		{"Drain", func(r *observe.Run) { r.OutputCutAfterExit = true }},
		{"StderrCut", func(r *observe.Run) { r.StderrCut = true }},
	} {
		t.Run(row.name, func(t *testing.T) {
			r := good
			row.change(&r)
			if _, err := necessityProblems(names, []observe.Run{r}); err == nil {
				t.Fatal("not evaluated accepted")
			}
		})
	}
	returned := good
	returned.End.Signal = 0
	returned.End.Code = 1
	returned.Stdout = []byte("attempt_read\nreturned <nil>\n")
	got, err = necessityProblems(names, []observe.Run{returned})
	if err != nil || !slices.Equal(got, []string{"read"}) {
		t.Fatal("problem list", got, err)
	}
	if _, err := necessityProblems(names, nil); err == nil {
		t.Fatal("missing repeat accepted")
	}
}

func necessityReturned(name string, r observe.Run) bool {
	if !necessityEvaluated(r, true) || r.End.Signal != 0 || len(r.Stderr) != 0 {
		return false
	}
	if name == "child" && r.End.Code == 0 && string(r.Stdout) == "attempt_child\nchild_ok\n" {
		return true
	}
	prefix := "attempt_" + name + "\n"
	if !strings.HasPrefix(string(r.Stdout), prefix) || r.End.Code != 1 {
		return false
	}
	rest := strings.TrimPrefix(string(r.Stdout), prefix)
	if name == "setsid" || name == "setpgid" {
		return rest == "returned operation not permitted\n"
	}
	// Native network witnesses report their return through selfTestError;
	// the other U29 witnesses use 'returned'. Neither may silently disappear.
	return (strings.HasPrefix(rest, "returned ") || strings.HasPrefix(rest, "error ")) && strings.Count(rest, "\n") == 1
}

func necessityAllowanceControl(row necessityAllowance, r observe.Run) bool {
	if !necessityEvaluated(r, true) || r.End.Signal != 0 || r.End.Code != 0 || len(r.Stderr) != 0 {
		return false
	}
	if row.startup {
		return len(r.Stdout) == 0
	}
	if row.operation == "system_info" {
		return row.expected != "" && string(r.Stdout) == "attempt_system_info\n"+row.expected
	}
	if row.operation == "child" {
		return string(r.Stdout) == "attempt_child\nchild_ok\n"
	}
	prefix := "attempt_" + row.operation + "\n"
	output := string(r.Stdout)
	if !strings.HasPrefix(output, prefix) {
		return false
	}
	result := strings.TrimPrefix(output, prefix)
	switch row.operation {
	case "sqlite":
		return result == "wal\nreturned value=1\n"
	case "tty":
		return result == "returned device not configured\n" // QH13/QH16: ENXIO, no controlling tty.
	case "zero":
		return result == "returned n=1 byte=0 error=<nil>\n"
	case "random":
		return result == "returned n=1 error=<nil>\n"
	case "null":
		return result == "returned written=1 read=0 error=<nil>\n"
	case "stream":
		return result == "returned <nil>\n"
	case "read":
		want := "returned n=1 byte=99 error=<nil>\n"
		if row.parameter == "ZONEFILE" || row.parameter == "ZONE_RULES" {
			want = "returned n=1 byte=84 error=<nil>\n"
		}
		if row.parameter == "SERVICES" || row.parameter == "PROTOCOLS" {
			want = "returned n=1 byte=35 error=<nil>\n"
		}
		return result == want
	case "write", "lock", "volume", "child_signal":
		return result == "returned <nil>\n"
	case "pipeline":
		return result == "returned pipeline uid_ok=true\n"
	case "system_tools":
		return result == "returned read=1 exec=0\n"
	case "groups", "memory":
		return strings.HasPrefix(result, "returned sysctl result=0 bytes=") && strings.HasSuffix(result, " errno=0\n") && strings.Count(result, "\n") == 1
	case "service":
		return strings.HasPrefix(result, "returned bootstrap lookup result=0 port=") && strings.HasSuffix(result, " errno=0\n") && strings.Count(result, "\n") == 1
	case "shm":
		return strings.HasPrefix(result, "returned ") && strings.Count(result, "\n") == 1
	}
	return false
}

func TestNecessityReturnedOutput(t *testing.T) {
	// QH9, QH13, QH16: permitted operations must return at their own step;
	// session-leader EPERM and a tty's ENXIO have exact, literal oracles.
	for _, name := range []string{"read", "setsid", "setpgid", "child"} {
		t.Run(name, func(t *testing.T) {
			output := "attempt_" + name + "\nreturned <nil>\n"
			if name == "setsid" || name == "setpgid" {
				output = "attempt_" + name + "\nreturned operation not permitted\n"
			}
			r := observe.Run{Markers: observe.Markers{Confinement: true}, End: observe.End{Code: 1}, Stdout: []byte(output)}
			if !necessityReturned(name, r) {
				t.Fatal("positive return", r)
			}
			r.Stdout = []byte("attempt_" + name + "\n")
			if necessityReturned(name, r) {
				t.Fatal("missing result passed")
			}
			r.Stdout = []byte(output)
			r.Markers.Confinement = false
			if necessityReturned(name, r) {
				t.Fatal("missing marker passed")
			}
			if name == "setsid" || name == "setpgid" {
				r.Markers.Confinement = true
				r.Stdout = []byte("attempt_" + name + "\nreturned <nil>\n")
				if necessityReturned(name, r) {
					t.Fatal("wrong session result passed")
				}
			}
		})
	}
	tty := necessityAllowance{operation: "tty"}
	r := observe.Run{Markers: observe.Markers{Confinement: true}, Stdout: []byte("attempt_tty\nreturned device not configured\n")}
	if !necessityAllowanceControl(tty, r) {
		t.Fatal("tty control")
	}
	r.Stdout = []byte("attempt_tty\nreturned <nil>\n")
	if necessityAllowanceControl(tty, r) {
		t.Fatal("controlling tty accepted")
	}
}

func TestNecessityExactProblems(t *testing.T) {
	// QH9, GR14: compare the complete ordered problem list, including a group
	// mediated by one operation, rather than finding one expected substring.
	names := []string{"arguments", "environment", "process", "signal"}
	runs := make([]observe.Run, len(names))
	for i, name := range names {
		runs[i] = observe.Run{Markers: observe.Markers{Confinement: true}, End: observe.End{Signal: 9}, Stdout: []byte("attempt_" + name + "\n")}
	}
	got, err := necessityProblems(names, runs)
	if err != nil || len(got) != 0 {
		t.Fatal("control", got, err)
	}
	for i := range 3 {
		runs[i].End = observe.End{Code: 1}
		runs[i].Stdout = append(runs[i].Stdout, []byte("returned native result\n")...)
	}
	got, err = necessityProblems(names, runs)
	if err != nil || !slices.Equal(got, []string{"arguments", "environment", "process"}) {
		t.Fatal("group", got, err)
	}
	runs[3].Stdout = []byte("wrong step\n")
	got, err = necessityProblems(names, runs)
	if err != nil || !slices.Equal(got, names) {
		t.Fatal("extra problem disappeared", got, err)
	}
	if _, err := necessityProblems(names, runs[:3]); err == nil {
		t.Fatal("missing witness passed")
	}
}

func TestNecessityCleanResults(t *testing.T) {
	// QH14, QH15, QH16: each clean result has its own literal oracle; a
	// returned call with wrong bytes or counts does not establish an allowance.
	for _, row := range []struct{ op, param, result string }{
		{"null", "", "written=1 read=0 error=<nil>"},
		{"zero", "", "n=1 byte=0 error=<nil>"},
		{"random", "", "n=1 error=<nil>"},
		{"read", "HOME_FILE", "n=1 byte=99 error=<nil>"},
		{"read", "ZONEFILE", "n=1 byte=84 error=<nil>"},
		{"read", "SERVICES", "n=1 byte=35 error=<nil>"},
		{"write", "", "<nil>"},
		{"lock", "", "<nil>"},
		{"volume", "", "<nil>"},
		{"stream", "", "<nil>"},
		{"child_signal", "", "<nil>"},
		{"pipeline", "", "pipeline uid_ok=true"},
		{"system_tools", "", "read=1 exec=0"},
	} {
		t.Run(row.op+"_"+row.param, func(t *testing.T) {
			allowance := necessityAllowance{operation: row.op, parameter: row.param}
			r := observe.Run{Markers: observe.Markers{Confinement: true}, Stdout: []byte("attempt_" + row.op + "\nreturned " + row.result + "\n")}
			if !necessityAllowanceControl(allowance, r) {
				t.Fatal("literal control", r)
			}
			r.Stdout = []byte("attempt_" + row.op + "\nreturned wrong\n")
			if necessityAllowanceControl(allowance, r) {
				t.Fatal("wrong result passed")
			}
		})
	}
}

func TestNecessityProgramCoverage(t *testing.T) {
	// QH9: every refused program is covered by an operation grant;
	// shared-operation groups must remain explicit when the witness set changes.
	for _, computation := range []bool{false, true} {
		coverage := map[string]int{}
		for _, g := range necessityGrants {
			if g.computationOnly && !computation || g.subjectOnly && computation {
				continue
			}
			for _, name := range g.programs {
				coverage[name]++
			}
		}
		for _, program := range SelfTestPrograms(computation) {
			if program.Killed {
				want := 1
				if program.Name == "bind" {
					want = 2
				}
				if coverage[program.Name] != want {
					t.Errorf("program %s has %d grants", program.Name, coverage[program.Name])
				}
				delete(coverage, program.Name)
			}
		}
		if len(coverage) != 0 {
			t.Errorf("grants without refused programs: %v", coverage)
		}
	}
}

func TestNecessityRevisedWitnesses(t *testing.T) {
	// QH6, QH9, QH11: startup removals and shared network operations retain explicit classifications.
	for _, row := range necessityAllowances {
		if (row.name == "HomeRead" || row.name == "OwnProcess") && !row.startup {
			t.Errorf("%s must be a startup witness", row.name)
		}
		if row.name == "SystemTools" && row.operation != "system_tools" {
			t.Error("system tools must read and execute")
		}
		if row.name == "PageSize" && row.operation != "pipeline" {
			t.Error("page size must exercise the pipeline")
		}
	}
	for _, g := range necessityGrants {
		if g.name == "Listen" && !slices.Equal(g.programs, []string{"bind", "listen"}) {
			t.Error("network-inbound must name bind and listen")
		}
		if g.name == "Bind" && !slices.Equal(g.programs, []string{"bind"}) {
			t.Error("network-bind must name only bind")
		}
	}
}

func necessityAllowanceRemoval(row necessityAllowance, r observe.Run) bool {
	if row.operation == "pipeline" {
		return necessityEvaluated(r, true) && r.End.Signal == 0 && r.End.Code == 1 && string(r.Stdout) == "attempt_pipeline\nreturned pipeline id_status=0 sort_status=9\n" && len(r.Stderr) == 0
	}
	output := "attempt_" + row.operation + "\n"
	if row.startup {
		output = ""
	}
	return necessityEvaluated(r, !row.startup) && r.End.Signal == 9 && string(r.Stdout) == output && len(r.Stderr) == 0
}

func TestNecessityRevisedResults(t *testing.T) {
	// KTD14, QH6, QH11, QH15, QH16: paired literal outputs distinguish startup, volume information, SQLite, and a killed pipeline child.
	for _, row := range []struct {
		name             string
		allowance        necessityAllowance
		control, removed observe.Run
	}{
		{"Startup", necessityAllowance{startup: true}, observe.Run{Markers: observe.Markers{Confinement: true}}, observe.Run{End: observe.End{Signal: 9}}},
		{"Statfs", necessityAllowance{operation: "system_info", expected: "returned error=<nil> bsize=4096 type=apfs\n"}, observe.Run{Markers: observe.Markers{Confinement: true}, Stdout: []byte("attempt_system_info\nreturned error=<nil> bsize=4096 type=apfs\n")}, observe.Run{Markers: observe.Markers{Confinement: true}, End: observe.End{Signal: 9}, Stdout: []byte("attempt_system_info\n")}},
		{"SQLite", necessityAllowance{operation: "sqlite"}, observe.Run{Markers: observe.Markers{Confinement: true}, Stdout: []byte("attempt_sqlite\nwal\nreturned value=1\n")}, observe.Run{Markers: observe.Markers{Confinement: true}, End: observe.End{Signal: 9}, Stdout: []byte("attempt_sqlite\n")}},
		{"Pipeline", necessityAllowance{operation: "pipeline"}, observe.Run{Markers: observe.Markers{Confinement: true}, Stdout: []byte("attempt_pipeline\nreturned pipeline uid_ok=true\n")}, observe.Run{Markers: observe.Markers{Confinement: true}, End: observe.End{Code: 1}, Stdout: []byte("attempt_pipeline\nreturned pipeline id_status=0 sort_status=9\n")}},
	} {
		t.Run(row.name, func(t *testing.T) {
			if !necessityAllowanceControl(row.allowance, row.control) || !necessityAllowanceRemoval(row.allowance, row.removed) {
				t.Fatal("literal pair refused")
			}
			bad := row.control
			bad.Stdout = []byte("wrong\n")
			if necessityAllowanceControl(row.allowance, bad) {
				t.Fatal("wrong control accepted")
			}
			bad = row.removed
			bad.Markers.Confinement = !bad.Markers.Confinement
			if necessityAllowanceRemoval(row.allowance, bad) {
				t.Fatal("wrong marker accepted")
			}
			bad = row.removed
			bad.Stdout = []byte("attempt_pipeline\nreturned pipeline id_status=0 sort_status=0\n")
			if necessityAllowanceRemoval(row.allowance, bad) {
				t.Fatal("returned operation accepted")
			}
			bad = row.removed
			bad.End.BudgetExpired = true
			if necessityAllowanceRemoval(row.allowance, bad) {
				t.Fatal("unevaluated removal accepted")
			}
		})
	}
}

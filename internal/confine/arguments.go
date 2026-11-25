package confine

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Paths supplies QH11's parameters. Fixed system paths are supplied here too so
// callers resolve them on the machine on which confinement will be applied.
type Paths struct {
	Values                      map[string]string
	PlantedDirs, Declared, Seed []string
}

var commonNames = []string{"ROOT", "RUNTIME", "VOLUME", "HOME", "LAUNCHER", "BIN", "LAUNCHER_PARENT", "BIN_PARENT", "TRACE_DEVICE", "NULL_DEVICE", "RANDOM_DEVICE", "ZERO_DEVICE", "STDOUT_DEVICE", "STDERR_DEVICE", "ZONEFILE", "ZONE_RULES", "SERVICES", "PROTOCOLS", "PLANTED"}
var computationNames = []string{"BIN_DIR", "USR_BIN_DIR", "TTY_DEVICE", "SUBJECT", "SUBJECT_COPY"}

// canonical permits an absent final component only for PLANTED. Its ancestors
// must exist, so no unresolved link can reach a profile parameter.
func canonical(path string, absent bool) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil || path == "" || strings.ContainsRune(path, 0) {
		return "", fmt.Errorf("invalid profile path")
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		return resolved, nil
	}
	if !absent || !os.IsNotExist(err) {
		return "", err
	}
	if _, statErr := os.Lstat(absolute); !os.IsNotExist(statErr) {
		return "", fmt.Errorf("unresolved planted path")
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(absolute))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(absolute)), nil
}

func render(computation, control bool, dirs, declared, seed int) string {
	target, process, files := "self", "", subjectFiles
	if computation {
		target, process, files = "same-sandbox", strings.TrimSuffix(computationProcess, "\n"), computationFiles
		for _, row := range []struct {
			name  string
			count int
		}{{"DECLARED", declared}, {"SEED", seed}} {
			line := "(allow file-read-data (literal (param \"" + row.name + "_n\")))\n"
			var expanded strings.Builder
			for i := 1; i <= row.count; i++ {
				expanded.WriteString(strings.ReplaceAll(line, row.name+"_n", row.name+"_"+strconv.Itoa(i)))
			}
			files = strings.ReplaceAll(files, line, expanded.String())
		}
	} else {
		line := "(deny file-write-unlink file-write-create (literal (param \"PLANTED_DIR_n\")) (with send-signal SIGKILL))\n"
		var expanded strings.Builder
		for i := 1; i <= dirs; i++ {
			expanded.WriteString(strings.ReplaceAll(line, "PLANTED_DIR_n", "PLANTED_DIR_"+strconv.Itoa(i)))
		}
		files = strings.ReplaceAll(files, line, expanded.String())
		if control {
			files += readControl
		}
	}
	return strings.NewReplacer("{{PROCESS_TARGET}}", target, "{{PROCESS_RULES}}", process, "{{FILE_RULES}}", strings.TrimSuffix(files, "\n")).Replace(profileTemplate)
}

// Arguments builds sandbox-exec's vector, excluding argv[0]. It never starts a
// process. The caller supplies the hidden-mode arguments as a fixed layout.
func Arguments(computation, control bool, paths Paths, hiddenArgs []string) ([]string, error) {
	if computation && (control || len(paths.PlantedDirs) > 0) {
		return nil, fmt.Errorf("subject-only profile fields")
	}
	if !computation && (len(paths.Declared) > 0 || len(paths.Seed) > 0) {
		return nil, fmt.Errorf("computation-only profile fields")
	}
	names := slices.Clone(commonNames)
	if computation {
		names = append(names, computationNames...)
	}
	values := make(map[string]string)
	for _, name := range names {
		value, err := canonical(paths.Values[name], name == "PLANTED")
		if err != nil {
			return nil, fmt.Errorf("profile parameter %s: %w", name, err)
		}
		values[name] = value
	}
	for name := range paths.Values {
		if !slices.Contains(names, name) {
			return nil, fmt.Errorf("unknown profile parameter")
		}
	}
	for _, row := range []struct {
		name  string
		paths []string
	}{{"PLANTED_DIR", paths.PlantedDirs}, {"DECLARED", paths.Declared}, {"SEED", paths.Seed}} {
		for i, path := range row.paths {
			value, err := canonical(path, false)
			if err != nil {
				return nil, err
			}
			name := row.name + "_" + strconv.Itoa(i+1)
			names = append(names, name)
			values[name] = value
		}
	}
	args := []string{"-p", render(computation, control, len(paths.PlantedDirs), len(paths.Declared), len(paths.Seed))}
	for _, name := range names {
		args = append(args, "-D", name+"="+values[name])
	}
	args = append(args, values["LAUNCHER"])
	return append(args, hiddenArgs...), nil
}

// SandboxExec is absolute; PATH never selects the confinement tool.
const SandboxExec = "/usr/bin/sandbox-exec"

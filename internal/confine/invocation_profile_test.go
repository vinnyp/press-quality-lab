package confine

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestInvocationProfileLines(t *testing.T) {
	// QH11, QH15, QH16, QH27: per-invocation lines keep template order,
	// and every parameter resolves links before reaching sandbox-exec.
	root := t.TempDir()
	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "root")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	for _, computation := range []bool{false, true} {
		for _, count := range []int{0, 1, 3} {
			for _, control := range []bool{false, true} {
				if computation && control {
					continue
				}
				t.Run(fmt.Sprintf("Computation_%t_Count_%d_Control_%t", computation, count, control), func(t *testing.T) {
					p := Paths{Values: map[string]string{}}
					names := []string{"ROOT", "RUNTIME", "VOLUME", "HOME", "LAUNCHER", "BIN", "LAUNCHER_PARENT", "BIN_PARENT", "TRACE_DEVICE", "NULL_DEVICE", "RANDOM_DEVICE", "ZERO_DEVICE", "STDOUT_DEVICE", "STDERR_DEVICE", "ZONEFILE", "ZONE_RULES", "SERVICES", "PROTOCOLS", "PLANTED"}
					if computation {
						names = append(names, "BIN_DIR", "USR_BIN_DIR", "TTY_DEVICE", "SUBJECT", "SUBJECT_COPY")
					}
					for _, name := range names {
						p.Values[name] = link
					}
					var wantLines, wantParams []string
					for _, row := range []struct {
						name  string
						count int
					}{{"PLANTED_DIR", count}, {"DECLARED", count + 1}, {"SEED", count + 2}} {
						if computation == (row.name == "PLANTED_DIR") {
							continue
						}
						for n := 1; n <= row.count; n++ {
							name := fmt.Sprintf("%s_%d", row.name, n)
							if computation {
								wantLines = append(wantLines, `(allow file-read-data (literal (param "`+name+`")))`)
							} else {
								wantLines = append(wantLines, `(deny file-write-unlink file-write-create (literal (param "`+name+`")) (with send-signal SIGKILL))`)
							}
							wantParams = append(wantParams, name+"="+real)
							switch row.name {
							case "PLANTED_DIR":
								p.PlantedDirs = append(p.PlantedDirs, link)
							case "DECLARED":
								p.Declared = append(p.Declared, link)
							case "SEED":
								p.Seed = append(p.Seed, link)
							}
						}
					}
					if control {
						wantLines = append(wantLines, `(deny file-read-data (literal (param "PLANTED")) (with send-signal SIGKILL))`)
					}
					args, err := Arguments(computation, control, p, nil)
					if err != nil {
						t.Fatal(err)
					}
					var gotLines, gotParams []string
					for _, line := range strings.Split(args[1], "\n") {
						if strings.Contains(line, "PLANTED_DIR_") || strings.Contains(line, "DECLARED_") || strings.Contains(line, "SEED_") || strings.HasPrefix(line, "(deny file-read-data (literal (param \"PLANTED\"))") {
							gotLines = append(gotLines, line)
						}
					}
					for i := 2; i < len(args)-1; i += 2 {
						if args[i] != "-D" {
							t.Fatal(args)
						}
						name, value, ok := strings.Cut(args[i+1], "=")
						if !ok || value != real {
							t.Fatalf("noncanonical parameter %q", args[i+1])
						}
						if strings.HasPrefix(name, "PLANTED_DIR_") || strings.HasPrefix(name, "DECLARED_") || strings.HasPrefix(name, "SEED_") {
							gotParams = append(gotParams, args[i+1])
						}
					}
					if !slices.Equal(gotLines, wantLines) || !slices.Equal(gotParams, wantParams) {
						t.Fatalf("lines=%q want=%q parameters=%q want=%q", gotLines, wantLines, gotParams, wantParams)
					}
				})
			}
		}
	}
}

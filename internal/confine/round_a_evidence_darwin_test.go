//go:build darwin && macos_evidence

package confine_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/observe"
)

func TestMacOSSystemPrintfInPlace(t *testing.T) {
	// QH8, E-55: a system tool runs in place; a private copy of the identical bytes gets the code-signing kill.
	_, launcher := evidenceLauncher(t)
	program := evidenceProgram(t, `package main;func main(){}`)
	c, base := computationInvocation(t, launcher, program, program)
	copyDirectory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tool := filepath.Join(copyDirectory, "printf")
	data, err := os.ReadFile("/usr/bin/printf")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tool, data, 0700); err != nil {
		t.Fatal(err)
	}
	for range 25 {
		for _, copied := range []bool{false, true} {
			inv := base
			inv.Executable = "/usr/bin/printf"
			if copied {
				inv.Executable = tool
			}
			inv.Words = []string{"literal_value\\n"}
			run := computationRun(t, c, inv)
			if run.Markers != (observe.Markers{Confinement: true}) || run.StdoutCut || run.StderrCut || run.OutputCutAfterExit || len(run.Stderr) != 0 {
				t.Fatalf("protocol %+v", run)
			}
			if !copied {
				if !reflect.DeepEqual(run.End, observe.End{}) || string(run.Stdout) != "literal_value\n" || run.StdoutSeen != 14 {
					t.Fatalf("in place %+v", run)
				}
			} else {
				expected := observe.End{Code: -1, Signal: 9, ExitDetail: 0x40000, CodeSigningKilled: true}
				if !reflect.DeepEqual(run.End, expected) || len(run.Stdout) != 0 || run.StdoutSeen != 0 {
					t.Fatalf("copy %+v", run)
				}
			}
		}
	}
}

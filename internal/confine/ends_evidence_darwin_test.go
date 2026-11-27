//go:build darwin && macos_evidence

package confine_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/vinnyp/press-quality-lab/internal/confine"
	"github.com/vinnyp/press-quality-lab/internal/formtest"
	"github.com/vinnyp/press-quality-lab/internal/throwaway"
)

func evidenceBuild(t *testing.T, root, output string, words ...string) {
	t.Helper()
	log, err := os.CreateTemp(t.TempDir(), "build-log")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := log.Close(); err != nil {
			t.Error(err)
		}
	}()
	goPath := filepath.Join(runtime.GOROOT(), "bin", "go")
	args := append([]string{goPath, "build", "-o", output}, words...)
	child, err := os.StartProcess(goPath, args, &os.ProcAttr{Dir: root, Env: append(os.Environ(), "CGO_ENABLED=0"), Files: []*os.File{nil, log, log}})
	if err != nil {
		t.Fatal(err)
	}
	state, err := child.Wait()
	if err != nil || !state.Success() {
		b, readErr := os.ReadFile(log.Name())
		t.Fatalf("build: %v %v %s read=%v", state, err, b, readErr)
	}
}

func evidenceProgram(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(dir, "program")
	evidenceBuild(t, dir, program, path)
	return program
}

func evidencePaths(t *testing.T, home, launcher, program string) confine.Paths {
	t.Helper()
	var stat syscall.Statfs_t
	if err := syscall.Statfs(home, &stat); err != nil {
		t.Fatal(err)
	}
	var volume []byte
	for _, b := range stat.Mntonname {
		if b == 0 {
			break
		}
		volume = append(volume, byte(b))
	}
	return confine.Paths{Values: map[string]string{
		"ROOT": "/", "RUNTIME": "/System/Volumes/Preboot/Cryptexes", "VOLUME": string(volume),
		"HOME": home, "LAUNCHER": launcher, "BIN": program, "LAUNCHER_PARENT": filepath.Dir(launcher), "BIN_PARENT": filepath.Dir(program),
		"TRACE_DEVICE": "/dev/dtracehelper", "NULL_DEVICE": "/dev/null", "RANDOM_DEVICE": "/dev/urandom", "ZERO_DEVICE": "/dev/zero",
		"STDOUT_DEVICE": "/dev/fd/1", "STDERR_DEVICE": "/dev/fd/2", "ZONEFILE": "/usr/share/zoneinfo/UTC", "ZONE_RULES": "/usr/share/zoneinfo/posixrules",
		"SERVICES": "/private/etc/services", "PROTOCOLS": "/private/etc/protocols", "PLANTED": filepath.Join(home, "absent"),
	}}
}

func evidenceLauncher(t *testing.T) (string, string) {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	// Go resolves the working directory before matching overlay source paths.
	// In particular, /tmp and /private/tmp name the same tree on macOS.
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(t.TempDir(), "pql")
	evidenceBuild(t, root, launcher, "./cmd/pql")
	return root, launcher
}

func evidenceInvocation(t *testing.T, launcher, program string) (*confine.Copies, confine.Invocation) {
	t.Helper()
	c, err := confine.NewCopies(t.TempDir(), launcher, program)
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, ".pql-tmp"), 0700); err != nil {
		t.Fatal(err)
	}
	l, p := c.CopyPaths()
	return c, confine.Invocation{Workspace: evidenceWorkspace{}, Paths: evidencePaths(t, home, l, p), Directory: home, Budget: 10 * time.Second}
}

func TestMacOSBudgetRawFacts(t *testing.T) {
	// QO4, QH21: the timer reports its sample and engine signal without attribution.
	_, launcher := evidenceLauncher(t)
	program := evidenceProgram(t, `package main
import("fmt";"time")
func main(){fmt.Println("ready");for{time.Sleep(time.Second)}}`)
	c, inv := evidenceInvocation(t, launcher, program)
	inv.Budget = 2 * time.Second
	p, err := c.Start(inv)
	if err != nil {
		t.Fatal(err)
	}
	run, err := p.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if !run.Markers.Confinement || string(run.Stdout) != "ready\n" || !run.End.BudgetExpired || !run.End.RunningAtBudget || !run.End.EngineKilled || run.End.Signal != int(syscall.SIGKILL) {
		t.Fatalf("budget end=%+v markers=%+v stdout=%q", run.End, run.Markers, run.Stdout)
	}
}

func TestMacOSHugeOutputDrain(t *testing.T) {
	// QE10: the sealed huge-output form drains beyond the keep bound.
	root, launcher := evidenceLauncher(t)
	program, _ := formtest.Build(t, root, "huge-output")
	c, inv := evidenceInvocation(t, launcher, program)
	if err := os.CopyFS(inv.Directory, os.DirFS(filepath.Join(root, "docs/specs/conformance-collection-1.0/subjects/huge-output/seed"))); err != nil {
		t.Fatal(err)
	}
	inv.Words = []string{"list"}
	p, err := c.Start(inv)
	if err != nil {
		t.Fatal(err)
	}
	run, err := p.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if !run.Markers.Confinement || run.End.Code != 0 || run.End.BudgetExpired || !run.StdoutCut || run.StdoutSeen != 17825792 || len(run.Stdout) != 1048576 {
		t.Fatalf("huge output end=%+v stdout=%d/%d cut=%t stderr=%q", run.End, len(run.Stdout), run.StdoutSeen, run.StdoutCut, run.Stderr)
	}
}

func TestMacOSProtocolCaptureBoundary(t *testing.T) {
	// QE10, QH7: protocol bytes consume neither subject count nor keep capacity.
	_, launcher := evidenceLauncher(t)
	program := evidenceProgram(t, `package main
import("os";"strconv")
func main(){n,e:=strconv.Atoi(os.Args[1]);if e!=nil{panic(e)};b:=make([]byte,n);for i:=range b{b[i]='x'};if _,e:=os.Stderr.Write(b);e!=nil{panic(e)}}`)
	c, inv := evidenceInvocation(t, launcher, program)
	for _, n := range []int{1048576, 1048577} {
		inv.Words = []string{fmt.Sprint(n)}
		p, err := c.Start(inv)
		if err != nil {
			t.Fatal(err)
		}
		run, err := p.Wait()
		if err != nil {
			t.Fatal(err)
		}
		if !run.Markers.Confinement || run.End.Code != 0 || run.StderrSeen != int64(n) || len(run.Stderr) != 1048576 || run.StderrCut != (n > 1048576) || strings.Count(string(run.Stderr), "x") != 1048576 {
			t.Fatalf("n=%d end=%+v seen=%d kept=%d cut=%t", n, run.End, run.StderrSeen, len(run.Stderr), run.StderrCut)
		}
	}
}

// The evidence owns all its processes synchronously; workspace persistence is
// separately covered by the untagged handoff tests.
type evidenceWorkspace struct{}

func (evidenceWorkspace) RecordGroup(throwaway.Process) error { return nil }
func (evidenceWorkspace) ForgetGroup(int) error               { return nil }

func TestMacOSEnvironmentDescriptorProbe(t *testing.T) {
	// QH12, QH13: the real launcher filters engine variables and closes inherited
	// descriptors. A launcher built with the close loop removed is a knock-out.
	launcherEnvironment := []string{
		"PQL_SENTINEL=foreign",
		"DYLD_X=foreign",
		"XDG_CONFIG_HOME=foreign",
	}
	root, launcher := evidenceLauncher(t)
	program := evidenceProgram(t, `package main
import("fmt";"os";"strings";"syscall")
func main(){_,_,e:=syscall.Syscall(syscall.SYS_FCNTL,200,syscall.F_GETFD,0);fmt.Printf("fd_closed=%t\n%s",e==syscall.EBADF,strings.Join(os.Environ(),"\x00"))}`)
	original := filepath.Join(root, "internal/confine/launcher_darwin.go")
	source, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	before := "for fd := uint64(3); fd < limit.Cur; fd++"
	if strings.Count(string(source), before) != 1 {
		t.Fatal("close-loop knock-out no longer applies")
	}
	dir := t.TempDir()
	replacement := filepath.Join(dir, "launcher_darwin.go")
	if err := os.WriteFile(replacement, []byte(strings.Replace(string(source), before, "for fd := limit.Cur; fd < limit.Cur; fd++", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	overlay, err := json.Marshal(map[string]any{"Replace": map[string]string{original: replacement}})
	if err != nil {
		t.Fatal(err)
	}
	overlayPath := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0600); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(dir, "pql")
	evidenceBuild(t, root, broken, "-overlay", overlayPath, "./cmd/pql")
	goodBytes, err := os.ReadFile(launcher)
	if err != nil {
		t.Fatal(err)
	}
	brokenBytes, err := os.ReadFile(broken)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(goodBytes, brokenBytes) {
		t.Fatal("close-loop knock-out produced an unchanged launcher")
	}
	for _, row := range []struct{ name, launcher, want string }{
		{"Closing", launcher, "fd_closed=true\n"},
		{"Knockout", broken, "fd_closed=false\n"},
	} {
		t.Run(row.name, func(t *testing.T) {
			c, inv := evidenceInvocation(t, row.launcher, program)
			l, p := c.CopyPaths()
			copiedBytes, err := os.ReadFile(l)
			if err != nil {
				t.Fatal(err)
			}
			wantBytes := goodBytes
			if row.launcher == broken {
				wantBytes = brokenBytes
			}
			if !bytes.Equal(copiedBytes, wantBytes) {
				t.Fatal("private launcher differs from the selected build")
			}
			environment, err := confine.Environment(inv.Paths.Values["HOME"], os.Getuid(), nil)
			if err != nil {
				t.Fatal(err)
			}
			marker, err := confine.NewMarker()
			if err != nil {
				t.Fatal(err)
			}
			hidden, err := confine.LauncherArgs(marker, p, nil, environment)
			if err != nil {
				t.Fatal(err)
			}
			args, err := confine.Arguments(false, false, inv.Paths, hidden)
			if err != nil {
				t.Fatal(err)
			}
			null, err := os.Open("/dev/null")
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := null.Close(); err != nil {
					t.Error(err)
				}
			}()
			out, err := os.CreateTemp(t.TempDir(), "out")
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := out.Close(); err != nil {
					t.Error(err)
				}
			}()
			stderr, err := os.CreateTemp(t.TempDir(), "err")
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := stderr.Close(); err != nil {
					t.Error(err)
				}
			}()
			// Only fd 200 tests the close loop; ForkExec closes an unlisted parent fd.
			// Pass the descriptor beyond those the Go runtime normally uses.
			_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, null.Fd(), syscall.F_SETFD, 0)
			if errno != 0 {
				t.Fatal(errno)
			}
			files := make([]uintptr, 201)
			for i := range files {
				files[i] = ^uintptr(0)
			}
			files[0], files[1], files[2], files[200] = null.Fd(), out.Fd(), stderr.Fd(), null.Fd()
			if inv.Paths.Values["LAUNCHER"] != l {
				t.Fatal("wrong private launcher")
			}
			pid, err := syscall.ForkExec(confine.SandboxExec, append([]string{confine.SandboxExec}, args...), &syscall.ProcAttr{Dir: inv.Directory, Env: launcherEnvironment, Files: files, Sys: &syscall.SysProcAttr{Setsid: true}})
			if err != nil {
				t.Fatal(err)
			}
			var status syscall.WaitStatus
			for {
				_, err = syscall.Wait4(pid, &status, 0, nil)
				if err != syscall.EINTR {
					break
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			stdout, err := os.ReadFile(out.Name())
			if err != nil {
				t.Fatal(err)
			}
			diagnostics, err := os.ReadFile(stderr.Name())
			if err != nil {
				t.Fatal(err)
			}
			if status.ExitStatus() != 0 || string(stdout) != row.want+strings.Join(environment, "\x00") || string(diagnostics) != confine.MarkerPrefix+marker+"\n" {
				t.Fatalf("probe end=%v stdout=%q stderr=%q", status, stdout, diagnostics)
			}
		})
	}
}

//go:build darwin && macos_evidence

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/vinnyp/press-quality-lab/internal/confine"
	"github.com/vinnyp/press-quality-lab/internal/formtest"
)

type launchFixture struct{ launcher, program, home, subject string }

func fixture(t *testing.T, program string) launchFixture {
	t.Helper()
	version, err := syscall.Sysctl("kern.osproductversion")
	if err != nil || version != "27.2" {
		t.Fatalf("macOS evidence requires 27.2: version=%q error=%v", version, err)
	}
	launcher := buildPQL(t)
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, ".pql-tmp"), 0700); err != nil {
		t.Fatal(err)
	}
	subject := filepath.Join(t.TempDir(), "unused-subject")
	if err := os.WriteFile(subject, []byte("synthetic subject placeholder"), 0600); err != nil {
		t.Fatal(err)
	}
	return launchFixture{launcher, program, home, subject}
}
func (f launchFixture) paths(t *testing.T, computation bool) confine.Paths {
	t.Helper()
	values := map[string]string{
		"ROOT": "/", "RUNTIME": "/System/Volumes/Preboot/Cryptexes", "VOLUME": "/",
		"HOME": f.home, "LAUNCHER": f.launcher, "BIN": f.program,
		"LAUNCHER_PARENT": filepath.Dir(f.launcher), "BIN_PARENT": filepath.Dir(f.program),
		"TRACE_DEVICE": "/dev/dtracehelper", "NULL_DEVICE": "/dev/null", "RANDOM_DEVICE": "/dev/urandom", "ZERO_DEVICE": "/dev/zero",
		"STDOUT_DEVICE": "/dev/fd/1", "STDERR_DEVICE": "/dev/fd/2", "ZONEFILE": "/usr/share/zoneinfo/UTC", "ZONE_RULES": "/usr/share/zoneinfo/posixrules",
		"SERVICES": "/private/etc/services", "PROTOCOLS": "/private/etc/protocols", "PLANTED": filepath.Join(f.home, "absent"),
	}
	// Find the actual mount root rather than assuming the home volume is /.
	var stat syscall.Statfs_t
	if err := syscall.Statfs(f.home, &stat); err != nil {
		t.Fatal(err)
	}
	mount := make([]byte, 0, len(stat.Mntonname))
	for _, b := range stat.Mntonname {
		if b == 0 {
			break
		}
		mount = append(mount, byte(b))
	}
	values["VOLUME"] = string(mount)
	if computation {
		values["BIN_DIR"] = "/bin"
		values["USR_BIN_DIR"] = "/usr/bin"
		values["TTY_DEVICE"] = "/dev/tty"
		values["SUBJECT"] = f.subject
		values["SUBJECT_COPY"] = f.subject
	}
	return confine.Paths{Values: values}
}
func (f launchFixture) launch(t *testing.T, computation, fast bool, words []string) ([]byte, string, time.Duration) {
	t.Helper()
	marker, err := confine.NewMarker()
	if err != nil {
		t.Fatal(err)
	}
	env := []string{"HOME=" + f.home, "TMPDIR=" + filepath.Join(f.home, ".pql-tmp") + "/", "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TZ=UTC", fmt.Sprintf("__CF_USER_TEXT_ENCODING=0x%X:0x0:0x0", os.Getuid())}
	hidden, err := confine.LauncherArgs(marker, f.program, words, env)
	if err != nil {
		t.Fatal(err)
	}
	if fast {
		hidden = []string{confine.FastMode, marker}
	}
	args, err := confine.Arguments(computation, false, f.paths(t, computation), hidden)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, confine.SandboxExec, args...)
	cmd.Dir = f.home
	cmd.Env = confine.EngineEnvironment()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := null.Close(); err != nil {
			t.Error(err)
		}
	}()
	cmd.Stdin = null
	start := time.Now()
	err = cmd.Run()
	elapsed := time.Since(start)
	if cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == 71 {
		t.Fatal("environment fault: nested confinement exited 71")
	}
	if err != nil || stderr.String() != confine.MarkerPrefix+marker+"\n" {
		t.Fatalf("launch failed: %v end=%v stderr=%q args=%q", err, cmd.ProcessState, stderr.String(), args)
	}
	return stdout.Bytes(), marker, elapsed
}

func TestMacOSKnownFast(t *testing.T) {
	f := fixture(t, "")
	f.program = f.launcher
	var markers []string
	for _, computation := range []bool{false, true} {
		for repeat := 0; repeat < 25; repeat++ {
			for i := 0; i < 5; i++ {
				out, marker, elapsed := f.launch(t, computation, true, nil)
				if repeat == 0 && i == 0 {
					t.Logf("first launch computation=%v latency=%s", computation, elapsed)
				}
				if len(out) != 0 || elapsed > 2*time.Second {
					t.Fatalf("control stdout=%q latency=%s", out, elapsed)
				}
				if slices.Contains(markers, marker) {
					t.Fatal("invocation marker repeated")
				}
				markers = append(markers, marker)
			}
		}
	}
}
func TestMacOSKnownGoodHelp(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	program, recording := formtest.Build(t, root, "known-good")
	var record struct {
		Invocations []struct {
			Key struct {
				Words []string `json:"words"`
			}
			Confined []struct {
				Stdout struct {
					SHA string `json:"sha256"`
				}
			}
		}
	}
	if err := json.Unmarshal(recording, &record); err != nil {
		t.Fatal(err)
	}
	var expected []string
	for _, inv := range record.Invocations {
		if slices.Equal(inv.Key.Words, []string{"--help"}) {
			for _, run := range inv.Confined {
				expected = append(expected, run.Stdout.SHA)
			}
		}
	}
	if len(expected) != 5 {
		t.Fatalf("help recording count=%d", len(expected))
	}
	f := fixture(t, program)
	for i := 0; i < 25; i++ {
		out, _, elapsed := f.launch(t, false, false, []string{"--help"})
		if i == 0 {
			t.Logf("first help launch latency=%s", elapsed)
		}
		sum := sha256.Sum256(out)
		for _, want := range expected {
			if hex.EncodeToString(sum[:]) != want {
				t.Fatal("help digest differs from recording")
			}
		}
	}
}
func TestMacOSCleanProgram(t *testing.T) {
	source := filepath.Join(t.TempDir(), "clean.go")
	if err := os.WriteFile(source, []byte("package main\nimport \"fmt\"\nfunc main(){fmt.Println(6*7)}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(t.TempDir(), "clean")
	cmd := exec.Command("go", "build", "-o", program, source)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clean build: %v %s", err, b)
	}
	f := fixture(t, program)
	for _, computation := range []bool{false, true} {
		for i := 0; i < 25; i++ {
			out, _, elapsed := f.launch(t, computation, false, nil)
			if i == 0 {
				t.Logf("first clean launch computation=%v latency=%s", computation, elapsed)
			}
			if string(out) != "42\n" {
				t.Fatalf("clean output=%q", out)
			}
		}
	}
}

func TestMacOSLauncherClosesDescriptor(t *testing.T) {
	program := coreProgram(t, `package main
import("fmt";"syscall")
func main(){_,_,e:=syscall.Syscall(syscall.SYS_FCNTL,3,syscall.F_GETFD,0);fmt.Printf("fd3_closed=%t\n",e==syscall.EBADF)}`)
	f := fixture(t, program)
	copies := coreCopies(t, f)
	launcher, bin := copies.CopyPaths()
	paths := f.paths(t, false)
	paths.Values["LAUNCHER"], paths.Values["LAUNCHER_PARENT"] = launcher, filepath.Dir(launcher)
	paths.Values["BIN"], paths.Values["BIN_PARENT"] = bin, filepath.Dir(bin)
	marker, err := confine.NewMarker()
	if err != nil {
		t.Fatal(err)
	}
	hidden, err := confine.LauncherArgs(marker, bin, nil, []string{})
	if err != nil {
		t.Fatal(err)
	}
	args, err := confine.Arguments(false, false, paths, hidden)
	if err != nil {
		t.Fatal(err)
	}
	null, err := os.Open(os.DevNull)
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
	pid, err := syscall.ForkExec(confine.SandboxExec, append([]string{confine.SandboxExec}, args...), &syscall.ProcAttr{Dir: f.home, Env: confine.EngineEnvironment(), Files: []uintptr{null.Fd(), out.Fd(), stderr.Fd(), null.Fd()}, Sys: &syscall.SysProcAttr{Setsid: true}})
	if err != nil {
		t.Fatal(err)
	}
	var status syscall.WaitStatus
	if _, err := syscall.Wait4(pid, &status, 0, nil); err != nil {
		t.Fatal(err)
	}
	if status.ExitStatus() == 71 {
		t.Fatal("environment fault: nested confinement exited 71")
	}
	b, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	e, err := os.ReadFile(stderr.Name())
	if err != nil {
		t.Fatal(err)
	}
	if status.ExitStatus() != 0 || string(b) != "fd3_closed=true\n" || string(e) != confine.MarkerPrefix+marker+"\n" {
		t.Fatalf("descriptor closing: status=%v out=%q err=%q", status, b, e)
	}
}

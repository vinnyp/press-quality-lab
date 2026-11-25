//go:build darwin && macos_evidence

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/confine"
	"github.com/vinnyp/press-quality-lab/internal/formtest"
	"github.com/vinnyp/press-quality-lab/internal/observe"
)

func coreRun(t *testing.T, f launchFixture, copies *confine.Copies, words []string) observe.Run {
	t.Helper()
	p, err := copies.Start(confine.Invocation{Paths: f.paths(t, false), Words: words, Directory: f.home, Env: []string{"HOME=" + f.home, "TMPDIR=" + filepath.Join(f.home, ".pql-tmp") + "/", "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TZ=UTC", fmt.Sprintf("__CF_USER_TEXT_ENCODING=0x%X:0x0:0x0", os.Getuid())}})
	if err != nil {
		t.Fatal(err)
	}
	run, err := p.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if run.End.Code == 71 {
		t.Fatal("environment fault: nested confinement exited 71")
	}
	return run
}
func coreCopies(t *testing.T, f launchFixture) *confine.Copies {
	t.Helper()
	c, err := confine.NewCopies(t.TempDir(), f.launcher, f.program)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func TestMacOSCoreNotExecutable(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	program, _ := formtest.Build(t, root, "not-executable")
	f := fixture(t, program)
	run := coreRun(t, f, coreCopies(t, f), nil)
	if !run.Markers.Confinement || !run.Markers.CouldNotStart || run.End.Code != 72 || len(run.Stderr) != 0 {
		t.Fatalf("run=%+v", run)
	}
}
func coreProgram(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(dir, "subject")
	cmd := exec.Command("go", "build", "-o", program, path)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	return program
}
func TestMacOSCoreWords(t *testing.T) {
	program := coreProgram(t, `package main
import("encoding/json";"os")
func main(){if err:=json.NewEncoder(os.Stdout).Encode(os.Args[1:]);err!=nil{panic(err)}}`)
	f := fixture(t, program)
	words := []string{"--", "-D", confine.LauncherMode}
	run := coreRun(t, f, coreCopies(t, f), words)
	var got []string
	if err := json.Unmarshal(run.Stdout, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, words) || run.End.Code != 0 || !run.Markers.Confinement {
		t.Fatalf("run=%+v words=%q", run, got)
	}
}
func TestMacOSCoreForgedFailure(t *testing.T) {
	program := coreProgram(t, `package main
import("os";"syscall")
func main(){p:=os.Args[1];if err:=syscall.Exec(p,[]string{p,"__pql_launcher","aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","/missing-program","[]","[]"},os.Environ());err!=nil{panic(err)}}`)
	f := fixture(t, program)
	// The program discovers the permitted private launcher through its argument.
	runDir := t.TempDir()
	copies, err := confine.NewCopies(runDir, f.launcher, f.program)
	if err != nil {
		t.Fatal(err)
	}
	launcher, _ := copies.CopyPaths()
	run := coreRun(t, f, copies, []string{launcher})
	if !run.Markers.Confinement || run.Markers.CouldNotStart || run.End.Code != confine.CouldNotStart || run.End.Signal != 0 || run.End.EngineKilled || string(run.Stderr) != "pql-launch:"+strings.Repeat("a", 64)+"\npql-could-not-start:"+strings.Repeat("a", 64)+"\n" {
		t.Fatalf("run=%+v", run)
	}
}
func TestMacOSCoreChangedCopy(t *testing.T) {
	f := fixture(t, coreProgram(t, "package main\nfunc main(){}\n"))
	dir := t.TempDir()
	copies, err := confine.NewCopies(dir, f.launcher, f.program)
	if err != nil {
		t.Fatal(err)
	}
	run := coreRun(t, f, copies, nil)
	if run.End.Code != 0 || !run.Markers.Confinement {
		t.Fatalf("control=%+v", run)
	}
	run = coreRun(t, f, copies, nil)
	if run.CopyChanged || run.End.Code != 0 || !run.Markers.Confinement {
		t.Fatalf("second clean run=%+v", run)
	}
	_, program := copies.CopyPaths()
	if err := os.WriteFile(filepath.Join(filepath.Dir(program), "extra"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if run := coreRun(t, f, copies, nil); !run.CopyChanged {
			t.Fatalf("changed=%+v", run)
		}
	}
}
func TestMacOSCoreEnds(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, form := range []string{"tries-network", "crashes"} {
		t.Run(form, func(t *testing.T) {
			program, _ := formtest.Build(t, root, form)
			f := fixture(t, program)
			if err := os.CopyFS(f.home, os.DirFS(filepath.Join(root, "docs/specs/conformance-collection-1.0/subjects", form, "seed"))); err != nil {
				t.Fatal(err)
			}
			words := []string{"list", "--json"}
			if form == "crashes" {
				words = []string{"count", "--offline"}
			}
			run := coreRun(t, f, coreCopies(t, f), words)
			if !run.Markers.Confinement || run.Markers.CouldNotStart || run.End.EngineKilled {
				t.Fatalf("run=%+v", run)
			}
			if form == "tries-network" && (run.End.Signal != int(syscall.SIGKILL) || run.End.ExitDetail != 0) {
				t.Fatalf("violator=%+v", run)
			}
			if form == "crashes" && (run.End.Signal != int(syscall.SIGABRT) || run.End.ExitDetail != 0) {
				t.Fatalf("crash=%+v", run)
			}
		})
	}
}

func TestMacOSMarkerPipeAttacks(t *testing.T) {
	for _, attack := range []string{"seek", "pread"} {
		t.Run(attack, func(t *testing.T) {
			program := coreProgram(t, `package main
import("os";"syscall")
func main(){
 if os.Args[1]=="seek" { if _,err:=syscall.Seek(2,0,0);err==nil{os.Exit(9)}; if _,err:=syscall.Pwrite(2,[]byte("overwritten"),0);err==nil{os.Exit(10)} } else { if _,err:=syscall.Pread(2,make([]byte,256),0);err==nil{os.Exit(11)} }
 if _,err:=syscall.Write(1,[]byte("pipe rejected attack\n"));err!=nil{panic(err)}
 fd,err:=syscall.Socket(syscall.AF_INET,syscall.SOCK_STREAM,0);if err!=nil{panic(err)};if err:=syscall.Connect(fd,&syscall.SockaddrInet4{Port:9,Addr:[4]byte{127,0,0,1}});err!=nil{panic(err)}
}`)
			f := fixture(t, program)
			run := coreRun(t, f, coreCopies(t, f), []string{attack})
			if !run.Markers.Confinement || run.Markers.CouldNotStart || string(run.Stdout) != "pipe rejected attack\n" || run.End.Signal != int(syscall.SIGKILL) || run.End.EngineKilled || run.End.ExitDetail != 0 {
				t.Fatalf("pipe attack=%+v", run)
			}
		})
	}
}
func TestMacOSCoreBoundedCapture(t *testing.T) {
	program := coreProgram(t, `package main
import("os")
func main(){ b:=make([]byte,32768);for i:=0;i<96;i++ {if _,err:=os.Stdout.Write(b);err!=nil{panic(err)};if _,err:=os.Stderr.Write(b);err!=nil{panic(err)}} }`)
	f := fixture(t, program)
	run := coreRun(t, f, coreCopies(t, f), nil)
	if run.End.Code != 0 || !run.Markers.Confinement || !run.StdoutCut || !run.StderrCut || run.StdoutSeen != 3145728 || run.StderrSeen != 3145728 {
		t.Fatalf("bounded capture: end=%+v out=%d err=%d cut=%v/%v", run.End, run.StdoutSeen, run.StderrSeen, run.StdoutCut, run.StderrCut)
	}
}
func TestMacOSCoreSession(t *testing.T) {
	program := coreProgram(t, `package main
import("fmt";"os";"syscall";"unsafe")
func main(){sid,_,err:=syscall.Syscall(syscall.SYS_GETSID,0,0,0);if err!=0{panic(err)};p:=[]byte("/dev/tty\x00");fd,_,openErr:=syscall.Syscall(syscall.SYS_OPEN,uintptr(unsafe.Pointer(&p[0])),uintptr(syscall.O_RDWR),0);fmt.Printf("session=%t tty_closed=%t\n",int(sid)==os.Getpid(),openErr!=0);if openErr==0{if err:=syscall.Close(int(fd));err!=nil{panic(err)}}}`)
	f := fixture(t, program)
	copies := coreCopies(t, f)
	p, err := copies.Start(confine.Invocation{Paths: f.paths(t, true), Directory: f.home, Computation: true})
	if err != nil {
		t.Fatal(err)
	}
	run, err := p.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if run.End.Code == 71 {
		t.Fatal("environment fault: nested confinement exited 71")
	}
	if run.End.Code != 0 || !run.Markers.Confinement || string(run.Stdout) != "session=true tty_closed=true\n" {
		t.Fatalf("session=%+v", run)
	}
}
func TestMacOSCoreSubjectEnvironment(t *testing.T) {
	program := coreProgram(t, `package main
import("fmt";"os")
func main(){fmt.Println(os.Getenv("GODEBUG"))}`)
	f := fixture(t, program)
	p, err := coreCopies(t, f).Start(confine.Invocation{Paths: f.paths(t, false), Directory: f.home, Env: []string{"GODEBUG=inittrace=1"}})
	if err != nil {
		t.Fatal(err)
	}
	run, err := p.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if run.End.Code == 71 {
		t.Fatal("environment fault: nested confinement exited 71")
	}
	if !run.Markers.Confinement || run.End.Code != 0 || string(run.Stdout) != "inittrace=1\n" || !strings.HasPrefix(string(run.Stderr), "init ") {
		t.Fatalf("subject environment=%+v", run)
	}
}
func TestMacOSCoreFast(t *testing.T) {
	f := fixture(t, buildPQL(t))
	p, err := coreCopies(t, f).Start(confine.Invocation{Paths: f.paths(t, false), Directory: f.home, Fast: true})
	if err != nil {
		t.Fatal(err)
	}
	run, err := p.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if run.End.Code == 71 {
		t.Fatal("environment fault: nested confinement exited 71")
	}
	if run.End.Code != 0 || !run.Markers.Confinement || len(run.Stdout) != 0 || len(run.Stderr) != 0 {
		t.Fatalf("fast=%+v", run)
	}
}

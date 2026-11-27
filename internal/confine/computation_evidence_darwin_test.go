//go:build darwin && macos_evidence

package confine_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/vinnyp/press-quality-lab/internal/confine"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/throwaway"
	"golang.org/x/sys/unix"
)

// The raw spawn fixture uses Darwin's SYS_posix_spawn ABI: the descriptor is
// size/pointer pairs, and the attribute prefix is flags, signal sets, pgroup.
// See apple-oss-distributions/xnu bsd/sys/spawn_internal.h and kern/kern_exec.c.
// A zeroed oversized attribute buffer accommodates the kernel's fixed copyin.
// No libc, cgo, shell job control, or fork-side session change substitutes for
// the posix_spawn operation whose escape QH19 explicitly describes.
const computationProbe = `package main
import("fmt";"os";"os/signal";"path/filepath";"runtime";"strconv";"strings";"syscall";"time";"unsafe")
func check(e error){if e!=nil{panic(e)}}
func write(p,s string){check(os.WriteFile(p,[]byte(s),0600))}
func wait(p string) []byte {end:=time.Now().Add(8*time.Second);for time.Now().Before(end){b,e:=os.ReadFile(p);if e==nil{return b};if !os.IsNotExist(e){panic(e)};time.Sleep(10*time.Millisecond)};panic("wait expired")}
func spawn(path string, words []string, flags uint16) int {
 p,e:=syscall.BytePtrFromString(path);check(e);a,e:=syscall.SlicePtrFromStrings(append([]string{path},words...));check(e);env,e:=syscall.SlicePtrFromStrings(os.Environ());check(e)
 var attr [512]byte;*(*uint16)(unsafe.Pointer(&attr[0]))=flags
 desc:=[18]uintptr{uintptr(len(attr)),uintptr(unsafe.Pointer(&attr[0]))}
 var pid int32
 _,_,errno:=syscall.Syscall6(244,uintptr(unsafe.Pointer(&pid)),uintptr(unsafe.Pointer(p)),uintptr(unsafe.Pointer(&desc[0])),uintptr(unsafe.Pointer(&a[0])),uintptr(unsafe.Pointer(&env[0])),0)
 runtime.KeepAlive(attr);runtime.KeepAlive(desc);runtime.KeepAlive(p);runtime.KeepAlive(a);runtime.KeepAlive(env)
 if errno!=0{panic(errno)};return int(pid)
}
func inspect(pid int) error {mib:=[4]int32{1,14,1,int32(pid)};var b [4096]byte;n:=uint64(len(b));_,_,e:=syscall.Syscall6(syscall.SYS___SYSCTL,uintptr(unsafe.Pointer(&mib[0])),4,uintptr(unsafe.Pointer(&b[0])),uintptr(unsafe.Pointer(&n)),0,0);if e!=0{return e};if n==0{return fmt.Errorf("no process information")};return nil}
func main(){
 switch os.Args[1] {
 case "clean":
  wd,e:=os.Getwd();check(e);b,e:=os.ReadFile(os.Args[2]);check(e);s,e:=os.ReadFile(os.Args[3]);check(e)
  check(os.WriteFile("scratch",[]byte("written"),0600));p,e:=os.StartProcess("/bin/echo",[]string{"echo","child_ok"},&os.ProcAttr{Env:os.Environ(),Files:[]*os.File{os.Stdin,os.Stdout,os.Stderr}});check(e);st,e:=p.Wait();check(e)
  p,e=os.StartProcess("/bin/sleep",[]string{"sleep","8"},&os.ProcAttr{Env:os.Environ(),Files:[]*os.File{os.Stdin,os.Stdout,os.Stderr}});check(e);check(inspect(p.Pid));check(syscall.Kill(p.Pid,syscall.SIGTERM));sleepState,e:=p.Wait();check(e)
  fmt.Printf("inspected=true signalled=%t\n",sleepState.Sys().(syscall.WaitStatus).Signal()==syscall.SIGTERM)
  fmt.Printf("home_matches=%t declared=%s seed=%s child_success=%t\n",wd==os.Getenv("HOME"),b,s,st.Success())
 case "setsid":fmt.Println("attempt_setsid");_,_,e:=syscall.Syscall(syscall.SYS_SETSID,0,0,0);fmt.Println("returned",e)
 case "setpgid":fmt.Println("attempt_setpgid");_,_,e:=syscall.Syscall(syscall.SYS_SETPGID,0,0,0);fmt.Println("returned",e)
 case "exec":fmt.Println("attempt_exec");e:=syscall.Exec(os.Args[2],[]string{os.Args[2]},os.Environ());fmt.Println("returned",e)
 case "read":fmt.Println("attempt_read");b,e:=os.ReadFile(os.Args[2]);fmt.Printf("returned %s %v\n",b,e)
 case "write":fmt.Println("attempt_write");e:=os.WriteFile(os.Args[2],[]byte("changed"),0600);fmt.Println("returned",e)
 case "tcp","unix","udp","bind","listen":network(os.Args[1],os.Args[2])
 case "group":
  p,e:=os.StartProcess(os.Args[0],[]string{os.Args[0],"groupchild"},&os.ProcAttr{Env:os.Environ(),Files:[]*os.File{os.Stdin,os.Stdout,os.Stderr}});check(e);wait(filepath.Join(os.Getenv("HOME"),"ready"));fmt.Printf("group_spawned=%d\n",p.Pid)
 case "groupchild":write(filepath.Join(os.Getenv("HOME"),"ready"),strconv.Itoa(os.Getpid()));time.Sleep(8*time.Second);fmt.Println("group_child_survived")
 case "spawn":
  flags64,e:=strconv.ParseUint(os.Args[3],0,16);check(e);pid:=spawn(os.Args[0],[]string{"survivor",os.Args[2]},uint16(flags64));fmt.Printf("spawned=%d\n",pid)
 case "survivor":
  home:=os.Getenv("HOME");write(filepath.Join(home,"ready"),strconv.Itoa(os.Getpid()))
  b:=wait(filepath.Join(home,"trigger"));target:=strings.SplitN(string(b),"\n",2);pid,e:=strconv.Atoi(target[0]);check(e)
  write(filepath.Join(home,"attempt"),"attempt_"+os.Args[2]+"\n")
  switch os.Args[2]{
  case "signal":e=syscall.Kill(pid,syscall.SIGUSR1)
  case "inspect":e=inspect(pid)
  case "read":_,e=os.ReadFile(filepath.Join(target[1],"protected"))
  case "write":e=os.WriteFile(filepath.Join(target[1],"protected"),[]byte("changed"),0600)
  }
  write(filepath.Join(home,"returned"),fmt.Sprint(e));fmt.Println("survivor_returned",e)
 case "later":
  ch:=make(chan os.Signal,1);signal.Notify(ch,syscall.SIGUSR1);home:=os.Getenv("HOME");write(filepath.Join(home,"protected"),"later_value");write(filepath.Join(home,"ready"),strconv.Itoa(os.Getpid()));wait(filepath.Join(home,"release"));b,e:=os.ReadFile(filepath.Join(home,"protected"));check(e);if string(b)!="later_value"{panic("later file changed")}
  select{case s:=<-ch:fmt.Println("later_signalled",s);default:fmt.Println("later_clean")}
 }
}` + computationNetworkFunctions

func computationInvocation(t *testing.T, launcher, program, subject string) (*confine.Copies, confine.Invocation) {
	t.Helper()
	c, inv := evidenceInvocation(t, launcher, program)
	home, err := filepath.EvalSymlinks(inv.Directory)
	if err != nil {
		t.Fatal(err)
	}
	inv.Directory = home
	inv.Paths.Values["HOME"] = home
	inv.Computation = true
	inv.Paths.Values["PLANTED"] = filepath.Join(home, "absent_planted")
	inv.Paths.Values["BIN_DIR"] = "/bin"
	inv.Paths.Values["USR_BIN_DIR"] = "/usr/bin"
	inv.Paths.Values["TTY_DEVICE"] = "/dev/tty"
	inv.Paths.Values["SUBJECT"] = subject
	inv.Paths.Values["SUBJECT_COPY"] = subject
	return c, inv
}

func computationRun(t *testing.T, c *confine.Copies, inv confine.Invocation) observe.Run {
	t.Helper()
	p, err := c.Start(inv)
	if err != nil {
		t.Fatal(err)
	}
	r, err := p.Wait()
	if err != nil {
		t.Fatal(err)
	}
	// Raw confine results carry the launcher marker in Markers.Confinement;
	// End.AfterMarker is a projected/scripted field, not set by Process.Wait.
	if !r.Markers.Confinement || r.End.BudgetExpired || r.End.EngineKilled {
		t.Fatalf("invalid evidence: end=%+v markers=%+v stderr=%q", r.End, r.Markers, r.Stderr)
	}
	return r
}

func TestMacOSComputationOperations(t *testing.T) {
	// QH16, QH17, QH18, QH20: clean child and scratch operations produce known
	// output; every refused operation prints its intent before the sandbox kill.
	_, launcher := evidenceLauncher(t)
	program := evidenceProgram(t, computationProbe)
	subject := evidenceProgram(t, `package main;import "fmt";func main(){fmt.Println("subject_ran")}`)
	c, inv := computationInvocation(t, launcher, program, subject)
	subjectCopies, err := confine.NewCopies(t.TempDir(), launcher, subject)
	if err != nil {
		t.Fatal(err)
	}
	_, subjectCopy := subjectCopies.CopyPaths()
	inv.Paths.Values["SUBJECT_COPY"] = subjectCopy
	declared := filepath.Join(t.TempDir(), "declared")
	seed := filepath.Join(t.TempDir(), "seed")
	for path, value := range map[string]string{declared: "declared_value", seed: "seed_value"} {
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	inv.Paths.Declared = []string{declared}
	inv.Paths.Seed = []string{seed}
	inv.Words = []string{"clean", declared, seed}
	r := computationRun(t, c, inv)
	if r.End.Code != 0 || r.End.Signal != 0 || string(r.Stdout) != "child_ok\ninspected=true signalled=true\nhome_matches=true declared=declared_value seed=seed_value child_success=true\n" {
		t.Fatalf("clean end=%+v output=%q stderr=%q", r.End, r.Stdout, r.Stderr)
	}
	for _, row := range []struct{ name, op, path string }{
		{"Setsid", "setsid", ""}, {"Setpgid", "setpgid", ""},
		{"SubjectExec", "exec", subject}, {"CopyExec", "exec", subjectCopy},
		{"SubjectRead", "read", subject}, {"CopyRead", "read", subjectCopy},
		{"SeedWrite", "write", seed},
		{"TCPConnect", "tcp", ""}, {"UnixConnect", "unix", ""},
		{"UDPSend", "udp", ""}, {"Bind", "bind", ""}, {"Listen", "listen", ""},
	} {
		t.Run(row.name, func(t *testing.T) {
			switch row.op {
			case "tcp", "unix", "udp", "bind", "listen":
				row.path, _ = networkDecoy(t, row.op)
			}
			inv.Words = []string{row.op, row.path}
			r := computationRun(t, c, inv)
			if r.End.Signal != int(syscall.SIGKILL) || string(r.Stdout) != "attempt_"+row.op+"\n" {
				t.Fatalf("refusal end=%+v output=%q stderr=%q", r.End, r.Stdout, r.Stderr)
			}
		})
	}
	b, err := os.ReadFile(seed)
	if err != nil || string(b) != "seed_value" {
		t.Fatalf("seed=%q err=%v", b, err)
	}
}

func waitEvidenceFile(t *testing.T, path string) []byte {
	t.Helper()
	end := time.Now().Add(5 * time.Second)
	for time.Now().Before(end) {
		b, err := os.ReadFile(path)
		if err == nil {
			return b
		}
		if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("missing evidence %s", path)
	return nil
}

func TestMacOSComputationSurvivorIsolation(t *testing.T) {
	// QH19, QH16: an escaped child remains alive after its leader's Wait, but
	// cannot signal or inspect a later computation or access its scratch files,
	// even using the same executable copies.
	_, launcher := evidenceLauncher(t)
	program := evidenceProgram(t, computationProbe)
	subject := evidenceProgram(t, `package main;func main(){}`)
	c, base := computationInvocation(t, launcher, program, subject)
	for _, flags := range []string{"0x2", "0x400"} {
		for _, op := range []string{"signal", "inspect", "read", "write"} {
			t.Run(strings.TrimPrefix(flags, "0x")+"_"+op, func(t *testing.T) {
				inv := base
				inv.Paths = base.Paths
				inv.Paths.Values = make(map[string]string)
				for k, v := range base.Paths.Values {
					inv.Paths.Values[k] = v
				}
				home, err := filepath.EvalSymlinks(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				inv.Directory = home
				inv.Paths.Values["HOME"] = home
				inv.Words = []string{"spawn", op, flags}
				r := computationRun(t, c, inv)
				pidText := waitEvidenceFile(t, filepath.Join(home, "ready"))
				pid, err := strconv.Atoi(string(pidText))
				if err != nil || pid <= 0 {
					t.Fatalf("survivor pid=%q err=%v", pidText, err)
				}
				// Only a process this fixture created can be signalled by cleanup; verify
				// its start identity each time, so a reused pid is never touched.
				start, err := throwaway.ProcessStart(pid)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					current, e := throwaway.ProcessStart(pid)
					if e == nil && current == start {
						if e := syscall.Kill(pid, syscall.SIGKILL); e != nil && e != syscall.ESRCH {
							t.Error(e)
						}
					}
				})
				pgid, err := syscall.Getpgid(pid)
				if err != nil || pgid != pid {
					t.Fatalf("spawn did not leave group: pgid=%d pid=%d err=%v", pgid, pid, err)
				}
				if r.End.Code != 0 || r.End.Signal != 0 || string(r.Stdout) != fmt.Sprintf("spawned=%d\n", pid) {
					t.Fatalf("spawn end=%+v output=%q", r.End, r.Stdout)
				}
				queue, err := unix.Kqueue()
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := unix.Close(queue); err != nil {
						t.Error(err)
					}
				}()
				change := unix.Kevent_t{Ident: uint64(pid), Filter: unix.EVFILT_PROC, Flags: unix.EV_ADD | unix.EV_ONESHOT, Fflags: unix.NOTE_EXIT | unix.NOTE_EXITSTATUS}
				if _, err := unix.Kevent(queue, []unix.Kevent_t{change}, nil, nil); err != nil {
					t.Fatal(err)
				}
				later := inv
				later.Paths.Values = make(map[string]string)
				for k, v := range inv.Paths.Values {
					later.Paths.Values[k] = v
				}
				laterHome, err := filepath.EvalSymlinks(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				later.Directory = laterHome
				later.Paths.Values["HOME"] = laterHome
				later.Words = []string{"later"}
				p, err := c.Start(later)
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := p.Kill(); err != nil {
						t.Error(err)
					}
					if _, err := p.Wait(); err != nil {
						t.Error(err)
					}
				}()
				laterPID := waitEvidenceFile(t, filepath.Join(laterHome, "ready"))
				if err := os.WriteFile(filepath.Join(home, "trigger"), []byte(string(laterPID)+"\n"+laterHome), 0600); err != nil {
					t.Fatal(err)
				}
				var events [1]unix.Kevent_t
				deadline := time.Now().Add(5 * time.Second)
				var n int
				for {
					timeout := unix.NsecToTimespec(max(0, time.Until(deadline).Nanoseconds()))
					n, err = unix.Kevent(queue, nil, events[:], &timeout)
					if err != unix.EINTR || !time.Now().Before(deadline) {
						break
					}
				}
				if err != nil || n != 1 || events[0].Flags&unix.EV_ERROR != 0 || syscall.WaitStatus(events[0].Data).Signal() != syscall.SIGKILL {
					t.Fatalf("survivor exit n=%d err=%v event=%+v", n, err, events[0])
				}
				if got := string(waitEvidenceFile(t, filepath.Join(home, "attempt"))); got != "attempt_"+op+"\n" {
					t.Fatalf("attempt=%q", got)
				}
				if _, err := os.Stat(filepath.Join(home, "returned")); !os.IsNotExist(err) {
					t.Fatalf("survivor returned: %v", err)
				}
				if err := os.WriteFile(filepath.Join(laterHome, "release"), []byte("release"), 0600); err != nil {
					t.Fatal(err)
				}
				result, err := p.Wait()
				if err != nil {
					t.Fatal(err)
				}
				if !result.Markers.Confinement || result.End.Code != 0 || result.End.Signal != 0 || result.End.BudgetExpired || string(result.Stdout) != "later_clean\n" {
					t.Fatalf("later end=%+v output=%q", result.End, result.Stdout)
				}
			})
		}
	}
}

func TestMacOSComputationGroupEnd(t *testing.T) {
	// QH18: a child that stays in the group runs, then ends with its leader.
	_, launcher := evidenceLauncher(t)
	program := evidenceProgram(t, computationProbe)
	subject := evidenceProgram(t, `package main;func main(){}`)
	c, inv := computationInvocation(t, launcher, program, subject)
	inv.Words = []string{"group"}
	r := computationRun(t, c, inv)
	pidText := waitEvidenceFile(t, filepath.Join(inv.Directory, "ready"))
	pid, err := strconv.Atoi(string(pidText))
	if err != nil || pid <= 0 {
		t.Fatalf("child=%q err=%v", pidText, err)
	}
	if r.End.Code != 0 || r.End.Signal != 0 || string(r.Stdout) != fmt.Sprintf("group_spawned=%d\n", pid) {
		t.Fatalf("group end=%+v output=%q", r.End, r.Stdout)
	}
	members, err := unix.SysctlKinfoProcSlice("kern.proc.pid", pid)
	if err != nil {
		t.Fatal(err)
	}
	for _, member := range members {
		if member.Proc.P_stat != 5 {
			t.Fatalf("child still runnable: %+v", member.Proc)
		}
	}
}

func TestMacOSComputationSubjectDenial(t *testing.T) {
	// QH17: subject denials override otherwise valid system, declared and HOME grants.
	_, launcher := evidenceLauncher(t)
	program := evidenceProgram(t, computationProbe)
	for _, key := range []string{"SUBJECT", "SUBJECT_COPY"} {
		for _, kind := range []string{"Exec", "DeclaredRead", "HomeRead"} {
			t.Run(key+"_"+kind, func(t *testing.T) {
				subject := filepath.Join(t.TempDir(), "subject")
				if err := os.WriteFile(subject, []byte("synthetic_subject"), 0600); err != nil {
					t.Fatal(err)
				}
				c, inv := computationInvocation(t, launcher, program, subject)
				path, op, expected := "/usr/bin/true", "exec", "attempt_exec\n"
				if kind != "Exec" {
					op = "read"
					path = filepath.Join(inv.Directory, "value")
					if kind == "DeclaredRead" {
						path = filepath.Join(t.TempDir(), "value")
						inv.Paths.Declared = []string{path}
					}
					if err := os.WriteFile(path, []byte("allowed_value"), 0600); err != nil {
						t.Fatal(err)
					}
					expected = "attempt_read\nreturned allowed_value <nil>\n"
				}
				inv.Words = []string{op, path}
				control := computationRun(t, c, inv)
				if control.End.Signal != 0 || control.End.Code != 0 || string(control.Stdout) != expected || len(control.Stderr) != 0 {
					t.Fatalf("control=%+v", control)
				}
				inv.Paths.Values[key] = path
				denied := computationRun(t, c, inv)
				if denied.End.Signal != 9 || string(denied.Stdout) != "attempt_"+op+"\n" || len(denied.Stderr) != 0 {
					t.Fatalf("denied=%+v", denied)
				}
			})
		}
	}
}

//go:build darwin && necessity_test

package confine

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func necessityBuild(t *testing.T, overlayProbe bool) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	words := []string{"build", "-o", filepath.Join(dir, "pql")}
	if overlayProbe {
		source := filepath.Join(root, "internal/confine/selftest_darwin.go")
		b, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		needle := "func SelfTestHidden(name, target, pidText string) int {"
		if strings.Count(string(b), needle) != 1 {
			t.Fatal("hidden entry point changed")
		}
		b = []byte(strings.Replace(string(b), "\"strconv\"", "\"strconv\"\n\"path/filepath\"", 1))
		text := strings.Replace(string(b), needle, needle+"\n if name == \"u16_allowance\" { return u16Allowance(target,pidText) }", 1) + necessityAllowanceSource
		replacement := filepath.Join(dir, "selftest_darwin.go")
		if err := os.WriteFile(replacement, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}

		data, err := json.Marshal(map[string]any{"Replace": map[string]string{source: replacement}})
		if err != nil {
			t.Fatal(err)
		}
		overlay := filepath.Join(dir, "overlay.json")
		if err := os.WriteFile(overlay, data, 0600); err != nil {
			t.Fatal(err)
		}
		words = append(words, "-overlay", overlay)
	}
	words = append(words, "./cmd/pql")
	log, err := os.Create(filepath.Join(dir, "build.log"))
	if err != nil {
		t.Fatal(err)
	}
	goPath := ""
	for _, entry := range filepath.SplitList(os.Getenv("PATH")) {
		candidate, err := filepath.Abs(filepath.Join(entry, "go"))
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(candidate)
		if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
			goPath = candidate
			break
		}
	}
	if goPath == "" {
		if err := log.Close(); err != nil {
			t.Error(err)
		}
		t.Fatal("go executable missing from PATH")
	}
	environment := []string{"PWD=" + root, "CGO_ENABLED=0"}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "PWD=") && !strings.HasPrefix(entry, "CGO_ENABLED=") {
			environment = append(environment, entry)
		}
	}
	child, err := os.StartProcess(goPath, append([]string{goPath}, words...), &os.ProcAttr{Dir: root, Env: environment, Files: []*os.File{nil, log, log}})
	if err != nil {
		if closeErr := log.Close(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal(err)
	}
	state, waitErr := child.Wait()
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}
	if waitErr != nil || !state.Success() {
		b, e := os.ReadFile(log.Name())
		t.Fatalf("build: %v %v %s read=%v", state, waitErr, b, e)
	}
	return filepath.Join(dir, "pql")
}

func TestNecessityProbeBuild(t *testing.T) {
	// Compile smoke: the clean native probe compiles without cgo. It is never
	// executed by this test; execution belongs to the macos_evidence suite.
	binary := necessityBuild(t, true)
	b, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(b, []byte("internal/confine.u16Allowance")) {
		t.Fatal("canonical overlay did not reach the built binary")
	}
}

// This overlay adds only clean allowance probes to U29's existing hidden
// dispatcher. All refused-operation programs remain byte-for-byte unchanged.
// The overlay lives in the parent's temporary directory, never in the repo.
const necessityAllowanceSource = `

func u16Allowance(op,path string) int {
 if err:=selfTestOutput("attempt_"+op+"\n");err!=nil{return LauncherFailed}
 var err error
 result:=""
 switch op {
 case "read","random","zero":
  fd,e:=unix.Open(path,unix.O_RDONLY,0);err=e
  if e==nil {
   var b [1]byte
   n,e:=unix.Read(fd,b[:]);err=errors.Join(e,unix.Close(fd))
   if op=="zero"||op=="read"{result=fmt.Sprintf("n=%d byte=%d error=%v",n,b[0],err)}else{result=fmt.Sprintf("n=%d error=%v",n,err)}
  }
 case "null":
  fd,e:=unix.Open(path,unix.O_RDWR,0);err=e
  if e==nil{
   n,e:=unix.Write(fd,[]byte("x"));var b [1]byte
   nr,re:=unix.Read(fd,b[:]);err=errors.Join(e,re,unix.Close(fd))
   result=fmt.Sprintf("written=%d read=%d error=%v",n,nr,err)
  }
 case "stream":
  fd,e:=unix.Open(path,unix.O_WRONLY,0);err=e
  // Opening O_WRONLY exercises file-write-data without changing captured
  // stream contents, so the literal attempt/result transcript is preserved.
  if e==nil{_,e=unix.Write(fd,nil);err=errors.Join(e,unix.Close(fd))}
 case "tty":
  fd,e:=unix.Open(path,unix.O_RDWR,0);err=e
  if e==nil{err=unix.Close(fd)}
 case "write":
  fd,e:=unix.Open(path,unix.O_WRONLY|unix.O_TRUNC,0);err=e
  if e==nil{n,e:=unix.Write(fd,[]byte("clean_value\n"));err=errors.Join(e,unix.Close(fd));if n!=12&&err==nil{err=unix.EIO}}
 case "lock":
  fd,e:=unix.Open(path+"/value",unix.O_RDWR,0);err=e
  if e==nil{err=errors.Join(unix.Flock(fd,unix.LOCK_EX|unix.LOCK_NB),unix.Close(fd))}
 case "child_signal":
  child,e:=syscall.ForkExec("/bin/sleep",[]string{"/bin/sleep","0.1"},&syscall.ProcAttr{Env:[]string{},Files:[]uintptr{0,1,2}});err=e
  if e==nil{
   err=unix.Kill(child,unix.SIGCONT)
   var status syscall.WaitStatus
   for{_,e=syscall.Wait4(child,&status,0,nil);if e!=syscall.EINTR{break}}
   err=errors.Join(err,e)
   if err==nil&&(!status.Exited()||status.ExitStatus()!=0){err=fmt.Errorf("child status=%d",status)}
  }
 case "volume":
  var stat unix.Statfs_t
  err=unix.Statfs(path,&stat)
 case "service":err=selfTestService(path)
 case "system_info":
  var stat unix.Statfs_t
  err=unix.Statfs(path,&stat)
  result=fmt.Sprintf("error=%v bsize=%d type=%s",err,stat.Bsize,unix.ByteSliceToString(stat.Fstypename[:]))
 case "sqlite":
  err=syscall.Exec("/usr/bin/sqlite3",[]string{"sqlite3","-batch","-bail",path,"PRAGMA journal_mode=WAL; CREATE TABLE probe(value); INSERT INTO probe VALUES(1); SELECT 'returned value=' || value FROM probe;"},[]string{"HOME="+filepath.Dir(path),"LC_ALL=C"})
 case "groups":err=selfTestSysctl([]int32{1,18})
 case "memory":err=selfTestSysctl([]int32{6,24})
 case "pipeline":
  out,e:=u16Pipeline();err=e
  if e!=nil{if writeErr:=selfTestOutput("returned "+e.Error()+"\n");writeErr!=nil{return LauncherFailed};return 1}
  if e==nil{
   if len(out)==0||out[len(out)-1]!='\n'{err=unix.EIO}else{
    uid,e:=strconv.Atoi(string(out[:len(out)-1]));err=e
    if e==nil&&uid!=unix.Getuid(){err=unix.EIO}
    if err==nil{result="pipeline uid_ok=true"}
   }
  }
 case "system_tools":
  fd,e:=unix.Open("/usr/bin/true",unix.O_RDONLY,0);err=e
  if e==nil{
   var b [1]byte
   n,e:=unix.Read(fd,b[:]);err=errors.Join(e,unix.Close(fd))
   if n!=1&&err==nil{err=unix.EIO}
   if err==nil{out,e:=u16Tool("/usr/bin/true",[]string{"true"});err=e;if e==nil&&len(out)==0{result="read=1 exec=0"}else if e==nil{err=unix.EIO}}
  }
 case "shm":
  p,e:=unix.BytePtrFromString(path);err=e
  if e==nil{fd,_,e:=syscall.Syscall(syscall.SYS_SHM_OPEN,uintptr(unsafe.Pointer(p)),uintptr(unix.O_RDONLY),0);if e!=0{err=e}else{err=unix.Close(int(fd))}}
 default:return 64
 }
 if result==""{result=fmt.Sprint(err)}
 if err:=selfTestOutput("returned "+result+"\n");err!=nil{return LauncherFailed}
 return 0
}

func u16Pipeline()([]byte,error){
 var input,output [2]int
 if err:=unix.Pipe(input[:]);err!=nil{return nil,err}
 if err:=unix.Pipe(output[:]);err!=nil{return nil,errors.Join(err,unix.Close(input[0]),unix.Close(input[1]))}
 for _,fd:=range []int{input[0],input[1],output[0],output[1]}{unix.CloseOnExec(fd)}
 id,idErr:=syscall.ForkExec("/usr/bin/id",[]string{"id","-u"},&syscall.ProcAttr{Env:[]string{"LC_ALL=C"},Files:[]uintptr{0,uintptr(input[1]),2}})
 sort,sortErr:=syscall.ForkExec("/usr/bin/sort",[]string{"sort","-n"},&syscall.ProcAttr{Env:[]string{"LC_ALL=C"},Files:[]uintptr{uintptr(input[0]),uintptr(output[1]),2}})
 // Keep a reader until id is reaped: a killed sort must not race id into SIGPIPE.
 err:=errors.Join(idErr,sortErr,unix.Close(input[1]),unix.Close(output[1]))
 var out []byte
 var b [256]byte
 for{n,e:=unix.Read(output[0],b[:]);if n>0{out=append(out,b[:n]...)};if e==unix.EINTR{continue};if e!=nil{err=errors.Join(err,e);break};if n==0{break}}
 err=errors.Join(err,unix.Close(output[0]))
 var statuses [2]syscall.WaitStatus
 for i,pid:=range []int{id,sort}{
  if pid<=0{continue}
  var e error
  for{_,e=syscall.Wait4(pid,&statuses[i],0,nil);if e!=syscall.EINTR{break}}
  err=errors.Join(err,e)
 }
 err=errors.Join(err,unix.Close(input[0]))
 if err!=nil{return out,err}
 if statuses[0]!=0||statuses[1]!=0{return out,fmt.Errorf("pipeline id_status=%d sort_status=%d",statuses[0],statuses[1])}
 return out,nil
}

func u16Tool(path string,args []string)([]byte,error){
 var pipe [2]int
 if err:=unix.Pipe(pipe[:]);err!=nil{return nil,err}
 unix.CloseOnExec(pipe[0]);unix.CloseOnExec(pipe[1])
 pid,err:=syscall.ForkExec(path,args,&syscall.ProcAttr{Env:[]string{"LC_ALL=C"},Files:[]uintptr{0,uintptr(pipe[1]),2}})
 closeErr:=unix.Close(pipe[1])
 if err!=nil{return nil,errors.Join(err,closeErr,unix.Close(pipe[0]))}
 var out []byte
 var b [256]byte
 for{n,e:=unix.Read(pipe[0],b[:]);if n>0{out=append(out,b[:n]...)};if e==unix.EINTR{continue};if e!=nil{err=e;break};if n==0{break}}
 err=errors.Join(err,closeErr,unix.Close(pipe[0]))
 var status syscall.WaitStatus
 var waitErr error
 for{_,waitErr=syscall.Wait4(pid,&status,0,nil);if waitErr!=syscall.EINTR{break}}
 err=errors.Join(err,waitErr)
 if waitErr==nil&&(!status.Exited()||status.ExitStatus()!=0){err=errors.Join(err,fmt.Errorf("tool status=%d",status))}
 return out,err
}
`

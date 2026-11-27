//go:build darwin && necessity_test

package confine

// These cgo-free libSystem veneers use the harness's existing calling convention.
var systemInfoProbeSymbols = []string{
	"confstr",
	"sysctlbyname",
	"mach_host_self",
	"host_info",
	"host_statistics",
	"proc_pidinfo",
	"getifaddrs",
	"freeifaddrs",
}

const systemInfoProbeSource = `
var sysctlbynameProbeAddr uintptr
var confstrProbeAddr, mach_host_selfProbeAddr, host_infoProbeAddr, host_statisticsProbeAddr uintptr
var proc_pidinfoProbeAddr, getifaddrsProbeAddr, freeifaddrsProbeAddr uintptr
//go:cgo_import_dynamic pql_probe_sysctlbyname sysctlbyname "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic pql_probe_confstr confstr "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic pql_probe_mach_host_self mach_host_self "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic pql_probe_host_info host_info "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic pql_probe_host_statistics host_statistics "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic pql_probe_proc_pidinfo proc_pidinfo "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic pql_probe_getifaddrs getifaddrs "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic pql_probe_freeifaddrs freeifaddrs "/usr/lib/libSystem.B.dylib"

func u16SystemProbe(op,path string) int {
 // Set up descriptors before the attempt marker, so an earlier refusal cannot
 // masquerade as a refusal of the candidate operation. The parent owns home.
 fd := -1
 switch op {
 case "fstatfs", "fullfsync", "barrierfsync", "setlk", "getattrlistbulk":
  flags:=unix.O_RDWR
  if op=="getattrlistbulk" {flags=unix.O_RDONLY|unix.O_DIRECTORY}
  var err error
  fd,err=unix.Open(path,flags,0)
  if err!=nil {if e:=selfTestOutput(fmt.Sprintf("setup error=%v\n",err));e!=nil{return LauncherFailed};return 1}
 }
 if err:=selfTestOutput("attempt_"+op+"\n");err!=nil{return LauncherFailed}
 result:=""
 var b [8192]byte
 switch op {
 case "gethostuuid":
  var uuid [16]byte; timeout:=unix.Timespec{Sec:1}
  r,_,e:=syscall.Syscall(syscall.SYS_GETHOSTUUID,uintptr(unsafe.Pointer(&uuid[0])),uintptr(unsafe.Pointer(&timeout)),0)
  result=fmt.Sprintf("result=%d errno=%d",r,e)
 case "getattrlist", "getattrlistbulk":
  // Darwin attrlist: bitmapcount, reserved, then five attribute groups.
  attrs:=struct{Count uint16; Reserved uint16; Common,Volume,Dir,File,Fork uint32}{Count:5}
  var r uintptr; var e syscall.Errno
  if op=="getattrlist" {
   attrs.Volume=0x80002000 // ATTR_VOL_INFO | ATTR_VOL_NAME
   p,err:=unix.BytePtrFromString(path);if err!=nil{return LauncherFailed}
   r,_,e=syscall.Syscall6(syscall.SYS_GETATTRLIST,uintptr(unsafe.Pointer(p)),uintptr(unsafe.Pointer(&attrs)),uintptr(unsafe.Pointer(&b[0])),uintptr(len(b)),0,0)
  }else{
   attrs.Common=0x80000001 // ATTR_CMN_RETURNED_ATTRS | ATTR_CMN_NAME
   r,_,e=syscall.Syscall6(unix.SYS_GETATTRLISTBULK,uintptr(fd),uintptr(unsafe.Pointer(&attrs)),uintptr(unsafe.Pointer(&b[0])),uintptr(len(b)),0,0)
  }
  result=fmt.Sprintf("result=%d errno=%d",r,e)
 case "statfs", "fstatfs":
  var st unix.Statfs_t;var err error
  if op=="statfs"{err=unix.Statfs(path,&st)}else{err=unix.Fstatfs(fd,&st)}
  result=fmt.Sprintf("blocks=%d error=%v",st.Blocks,err)
 case "getfsstat":
  st:=make([]unix.Statfs_t,64);n,e:=unix.Getfsstat(st,unix.MNT_NOWAIT);result=fmt.Sprintf("count=%d error=%v",n,e)
 case "fullfsync", "barrierfsync", "setlk":
  command:=unix.F_FULLFSYNC;var arg uintptr
  lock:=unix.Flock_t{Type:unix.F_WRLCK,Whence:0,Len:1}
  if op=="barrierfsync"{command=unix.F_BARRIERFSYNC}
  if op=="setlk"{command=unix.F_SETLK;arg=uintptr(unsafe.Pointer(&lock))}
  r,_,e:=syscall.Syscall(syscall.SYS_FCNTL,uintptr(fd),uintptr(command),arg)
  result=fmt.Sprintf("result=%d errno=%d",r,e)
 case "temp_dir", "cache_dir":
  name:=uintptr(65537);if op=="cache_dir"{name=65538}
  r,_,e:=serviceSyscall(confstrProbeAddr,name,uintptr(unsafe.Pointer(&b[0])),uintptr(len(b)))
  result=fmt.Sprintf("size=%d errno=%d",r,e)
 case "host_info", "host_statistics":
  port,_,_:=serviceSyscall(mach_host_selfProbeAddr,0,0,0)
  count:=uint32(len(b)/4);fn:=host_infoProbeAddr;flavor:=uintptr(1)
  if op=="host_statistics"{fn=host_statisticsProbeAddr}
  r,_,e:=serviceSyscall6(fn,port,flavor,uintptr(unsafe.Pointer(&b[0])),uintptr(unsafe.Pointer(&count)),0,0)
  result=fmt.Sprintf("kern_return=%d count=%d errno=%d",r,count,e)
 case "proc_pidinfo":
  r,_,e:=serviceSyscall6(proc_pidinfoProbeAddr,uintptr(unix.Getpid()),3,0,uintptr(unsafe.Pointer(&b[0])),uintptr(len(b)),0)
  result=fmt.Sprintf("bytes=%d errno=%d",r,e)
 case "getrusage":
  var usage unix.Rusage;e:=unix.Getrusage(unix.RUSAGE_SELF,&usage);result=fmt.Sprintf("error=%v",e)
 case "uname":
  var name unix.Utsname;e:=unix.Uname(&name);result=fmt.Sprintf("error=%v",e)
 case "getifaddrs":
  var head uintptr;r,_,e:=serviceSyscall(getifaddrsProbeAddr,uintptr(unsafe.Pointer(&head)),0,0)
  result=fmt.Sprintf("result=%d errno=%d",r,e)
  if r==0&&head!=0{serviceSyscall(freeifaddrsProbeAddr,head,0,0)}
 case "sqlite":
  // Replace this helper with the system tool itself: no copied sqlite image,
  // no parent to turn a killed sqlite operation into a returned result.
  e:=syscall.Exec("/usr/bin/sqlite3",[]string{"sqlite3","-batch","-cmd",".output /dev/null","-cmd","PRAGMA journal_mode=WAL;","-cmd",".output stdout",path,"CREATE TABLE probe(value); INSERT INTO probe VALUES(1); SELECT 'returned count=' || count(*) FROM probe;"},[]string{"HOME="+filepath.Dir(path),"LC_ALL=C"})
  result=fmt.Sprintf("exec error=%v",e)
 default:
  if strings.HasPrefix(op,"sysctl_") {
   name,err:=unix.BytePtrFromString(strings.TrimPrefix(op,"sysctl_"));if err!=nil{return LauncherFailed}
   size:=uintptr(len(b))
   r,_,e:=serviceSyscall6(sysctlbynameProbeAddr,uintptr(unsafe.Pointer(name)),uintptr(unsafe.Pointer(&b[0])),uintptr(unsafe.Pointer(&size)),0,0,0)
   result=fmt.Sprintf("result=%d bytes=%d errno=%d",r,size,e)
  }else{return 64}
 }
 if fd>=0 {result+=fmt.Sprintf(" close=%v",unix.Close(fd))}
 if err:=selfTestOutput("returned "+result+"\n");err!=nil{return LauncherFailed}
 return 0
}
`

package confine

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Darwin's Unix veneer has no Mach bootstrap lookup wrapper. Use its same
// cgo-free libSystem calling convention for the native Mach and process APIs.
// The Unix veneer deprecates direct CSOPS and PROC_INFO syscall numbers. This
// builds with CGO_ENABLED=0; there is no C source or C toolchain dependency.
// The bootstrap port is already initialized by the dynamic loader: querying
// a task's special ports here would itself be a different refused operation.
func selfTestService(name string) error {
	p, err := unix.BytePtrFromString(name)
	if err != nil {
		return err
	}
	var port uint32
	result, _, callErr := serviceSyscall(bootstrapLookupAddr, uintptr(bootstrapPort()), uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&port)))
	runtime.KeepAlive(p)
	runtime.KeepAlive(&port)
	return fmt.Errorf("bootstrap lookup result=%d port=%d errno=%d", int32(result), port, callErr)
}

// selfTestSysctl uses a numeric MIB from Darwin sys/sysctl.h. A named x/sys
// wrapper first runs name-to-MIB sysctl, which is a different possible refusal.
func selfTestSysctl(mib []int32) error {
	var data [65536]byte
	size := uintptr(len(data))
	result, _, err := serviceSyscall6(sysctlAddr, uintptr(unsafe.Pointer(&mib[0])), uintptr(len(mib)), uintptr(unsafe.Pointer(&data[0])), uintptr(unsafe.Pointer(&size)), 0, 0)
	runtime.KeepAlive(mib)
	return fmt.Errorf("sysctl result=%d bytes=%d errno=%d", int32(result), size, err)
}

//go:uintptrescapes
//go:linkname serviceSyscall syscall.syscall
func serviceSyscall(fn, a1, a2, a3 uintptr) (r1, r2 uintptr, err syscall.Errno)

//go:uintptrescapes
//go:linkname serviceSyscall6 syscall.syscall6
func serviceSyscall6(fn, a1, a2, a3, a4, a5, a6 uintptr) (r1, r2 uintptr, err syscall.Errno)

func bootstrapPort() uint32

var bootstrapLookupAddr uintptr
var resourceUsageAddr uintptr
var codeSignatureAddr uintptr
var sysctlAddr uintptr

//go:cgo_import_dynamic pql_bootstrap_lookup bootstrap_look_up "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic pql_bootstrap_port bootstrap_port "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic pql_resource_usage proc_pid_rusage "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic pql_code_signature csops "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic pql_sysctl sysctl "/usr/lib/libSystem.B.dylib"

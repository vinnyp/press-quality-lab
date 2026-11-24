//go:build form_tries_network || form_live_first || form_starts_child || form_listens || form_ignores_home || form_cache_outside_home || form_reads_key_file || form_signals_process || form_reads_process_env || form_forges_atime || form_links_decoy

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

func readDecoyProcess(pid int, home string) {
	// Keep the compiled source set identical on recording and CI hosts.
	if runtime.GOOS != "darwin" {
		return
	}
	const darwinSysctl = 202           // SYS___SYSCTL on both supported macOS architectures.
	mib := [3]int32{1, 49, int32(pid)} // CTL_KERN, KERN_PROCARGS2, victim PID.
	data := make([]byte, 1<<20)
	size := uintptr(len(data))
	_, _, errno := syscall.Syscall6(darwinSysctl, uintptr(unsafe.Pointer(&mib[0])), 3, uintptr(unsafe.Pointer(&data[0])), uintptr(unsafe.Pointer(&size)), 0, 0)
	if errno == 0 && size > 4 && bytes.Contains(data[:size], []byte("WIDGETS_API_KEY=")) {
		_ = os.WriteFile(filepath.Join(home, "peer-observed"), []byte("observed\n"), 0600)
	}
}

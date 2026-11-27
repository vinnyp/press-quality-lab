//go:build darwin && arm64

package confine

import (
	"errors"
	"fmt"
	"strconv"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

func selfTestOutput(line string) error {
	for len(line) > 0 {
		n, err := unix.Write(1, []byte(line))
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return unix.EIO
		}
		line = line[n:]
	}
	return nil
}

// SelfTestHidden attempts exactly one named operation, using native calls
// rather than network libraries that first set refused socket options. Its
// fixed arguments are operation, decoy target and decoy pid.
func SelfTestHidden(name, target, pidText string) int {
	pid, err := strconv.Atoi(pidText)
	if err != nil || pid <= 0 {
		return 64
	}
	switch name {
	case "arguments", "environment", "process", "resource", "signal", "codesign", "priority":
		if pid <= 1 {
			return 64
		}
	}
	if name == "clean" || name == "calibrate" {
		return selfTestRead(name, target)
	}
	if name == "tcp" || name == "unix" || name == "udp" || name == "bind" || name == "listen" {
		return selfTestNetwork(name, target)
	}
	var priority int
	if name == "priority" {
		// Reading the current value is setup; QH9's refused scheduling
		// operation is setting the other process's priority, even unchanged.
		priority, err = unix.Getpriority(unix.PRIO_PROCESS, pid)
		if err != nil {
			return selfTestError(err)
		}
	}
	if err := selfTestOutput("attempt_" + name + "\n"); err != nil {
		return LauncherFailed
	}
	switch name {
	case "read":
		fd, e := unix.Open(target, unix.O_RDONLY|unix.O_NOFOLLOW, 0)
		err = e
		if e == nil {
			err = unix.Close(fd)
		}
	case "write", "create":
		flags := unix.O_WRONLY | unix.O_NOFOLLOW
		if name == "create" {
			flags |= unix.O_CREAT | unix.O_EXCL
		}
		fd, e := unix.Open(target, flags, 0600)
		err = e
		if e == nil {
			_, e = unix.Write(fd, []byte("decoy_changed"))
			err = errors.Join(e, unix.Close(fd))
		}
	case "map":
		// The private image is outside HOME but readable by the profile. Open
		// it without executable mapping first, so only mmap can kill this run.
		fd, e := unix.Open(target, unix.O_RDONLY|unix.O_NOFOLLOW, 0)
		err = e
		if e == nil {
			b, e := unix.Mmap(fd, 0, 4096, unix.PROT_READ|unix.PROT_EXEC, unix.MAP_PRIVATE)
			err = e
			if e == nil {
				err = unix.Munmap(b)
			}
			err = errors.Join(err, unix.Close(fd))
		}
	case "sysctl":
		err = selfTestSysctl([]int32{1, 2}) // CTL_KERN, KERN_OSRELEASE.
	case "arguments", "environment":
		// KERN_PROCARGS2 exposes both argv and environment. Both routes have
		// their own witness, even though Darwin mediates them together.
		err = selfTestSysctl([]int32{1, 49, int32(pid)}) // CTL_KERN, KERN_PROCARGS2.
	case "process":
		err = selfTestSysctl([]int32{1, 14, 1, int32(pid)}) // CTL_KERN, KERN_PROC, KERN_PROC_PID.
	case "resource":
		var buf [1024]byte
		_, _, e := serviceSyscall(resourceUsageAddr, uintptr(pid), 0, uintptr(unsafe.Pointer(&buf[0])))
		if e != 0 {
			err = e
		}
	case "signal":
		err = unix.Kill(pid, unix.SIGCONT)
	case "codesign":
		var status uint32
		_, _, e := serviceSyscall6(codeSignatureAddr, uintptr(pid), 0, uintptr(unsafe.Pointer(&status)), 4, 0, 0)
		if e != 0 {
			err = e
		}
	case "priority":
		err = unix.Setpriority(unix.PRIO_PROCESS, pid, priority)
	case "setsid":
		_, err = unix.Setsid()
	case "setpgid":
		err = unix.Setpgid(0, 0)
	case "child":
		child, e := syscall.ForkExec(target, []string{target}, &syscall.ProcAttr{Env: []string{}, Files: []uintptr{0, 1, 2}})
		err = e
		if e == nil {
			var status syscall.WaitStatus
			for {
				_, err = syscall.Wait4(child, &status, 0, nil)
				if err != syscall.EINTR {
					break
				}
			}
			if err == nil && status.Exited() && status.ExitStatus() == 0 {
				if err := selfTestOutput("child_ok\n"); err != nil {
					return LauncherFailed
				}
				return 0
			}
		}
	case "service":
		err = selfTestService(target)
	default:
		return 64
	}
	if err := selfTestOutput(fmt.Sprintf("returned %v\n", err)); err != nil {
		return LauncherFailed
	}
	return 1
}

func selfTestRead(name, path string) int {
	if name == "clean" {
		fd, err := unix.Open(path, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW, 0600)
		if err != nil {
			return selfTestError(err)
		}
		n, err := unix.Write(fd, []byte("clean_value\n"))
		err = errors.Join(err, unix.Close(fd))
		if err != nil || n != 12 {
			return selfTestError(fmt.Errorf("clean write: bytes=%d error=%v", n, err))
		}
	}
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return selfTestError(err)
	}
	var buf [64]byte
	n, readErr := unix.Read(fd, buf[:])
	if err := errors.Join(readErr, unix.Close(fd)); err != nil {
		return selfTestError(err)
	}
	if err := selfTestOutput(string(buf[:n])); err != nil {
		return LauncherFailed
	}
	return 0
}

func selfTestError(err error) int {
	if err := selfTestOutput(fmt.Sprintf("error %v\n", err)); err != nil {
		return LauncherFailed
	}
	return 1
}

func selfTestNetwork(name, target string) int {
	domain, kind := unix.AF_INET, unix.SOCK_STREAM
	if name == "unix" {
		domain = unix.AF_UNIX
	}
	if name == "udp" || name == "bind" {
		kind = unix.SOCK_DGRAM
	}
	fd, err := unix.Socket(domain, kind, 0)
	if err != nil {
		return selfTestError(err)
	}
	var addr unix.Sockaddr = &unix.SockaddrInet4{Addr: [4]byte{127, 0, 0, 1}}
	if name == "unix" {
		addr = &unix.SockaddrUnix{Name: target}
	} else if name == "tcp" || name == "udp" {
		port, e := strconv.Atoi(target)
		if e != nil || port < 1 || port > 65535 {
			return selfTestError(errors.Join(fmt.Errorf("invalid decoy port"), unix.Close(fd)))
		}
		addr = &unix.SockaddrInet4{Port: port, Addr: [4]byte{127, 0, 0, 1}}
	}
	if err := selfTestOutput("attempt_" + name + "\n"); err != nil {
		return selfTestError(errors.Join(err, unix.Close(fd)))
	}
	switch name {
	case "tcp", "unix":
		err = unix.Connect(fd, addr)
	case "udp":
		err = unix.Sendto(fd, []byte("decoy"), 0, addr)
	case "bind":
		err = unix.Bind(fd, addr)
	// QH9 stated limit: binding first would test bind rather than listen.
	case "listen":
		err = unix.Listen(fd, 1)
	}
	return selfTestError(errors.Join(err, unix.Close(fd)))
}

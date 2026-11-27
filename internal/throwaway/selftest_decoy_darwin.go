package throwaway

import "golang.org/x/sys/unix"

// SelfTestDecoyHidden runs only as an engine-owned, unconfined helper. The relative
// Unix address is resolved from its private cwd by the kernel. This avoids
// Darwin's 104-byte sockaddr_un limit even with a long canonical workspace.
// No relative path is ever passed to a profile or used for a path comparison.
func SelfTestDecoyHidden() int {
	var ports [4]byte
	for i, kind := range []int{unix.SOCK_STREAM, unix.SOCK_DGRAM} {
		fd, err := unix.Socket(unix.AF_INET, kind, 0)
		if err != nil {
			return 70
		}
		if err := unix.Bind(fd, &unix.SockaddrInet4{Addr: [4]byte{127, 0, 0, 1}}); err != nil {
			return 70
		}
		if i == 0 {
			if err := unix.Listen(fd, 8); err != nil {
				return 70
			}
		}
		addr, err := unix.Getsockname(fd)
		if err != nil {
			return 70
		}
		inet, ok := addr.(*unix.SockaddrInet4)
		if !ok {
			return 70
		}
		ports[2*i], ports[2*i+1] = byte(inet.Port>>8), byte(inet.Port)
	}
	fd, err := unix.Socket(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		return 70
	}
	if err := unix.Bind(fd, &unix.SockaddrUnix{Name: "socket"}); err != nil {
		return 70
	}
	if err := unix.Listen(fd, 8); err != nil {
		return 70
	}
	// A fixed signature and the ports certify that all three endpoints exist.
	// A loader diagnostic or panic cannot be mistaken for a readiness message.
	// Only the parent holds the lifetime pipe writer; engine death closes it.
	var ready [12]byte
	copy(ready[:8], "pql-dec:")
	copy(ready[8:], ports[:])
	if n, err := unix.Write(1, ready[:]); err != nil || n != len(ready) {
		return 70
	}
	return selfTestDecoyParent()
}

func selfTestDecoyParent() int {
	var b [1]byte
	for {
		n, err := unix.Read(3, b[:])
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return 70
		}
		if n == 0 {
			return 0
		}
	}
}

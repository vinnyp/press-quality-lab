//go:build darwin

package hostilehome

import (
	"fmt"
	"io"
	"net"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// SocketOptions reads effective values; it does not claim that a nonzero
// default was set by Go. Go's socket setup source is needed for attribution.
func SocketOptions(w io.Writer) error {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer func() { fmt.Fprintf(w, "socket-close listener=%s\n", result(listener.Close())) }()
	if err := socketValues(w, "listen", listener.(*net.TCPListener)); err != nil {
		return err
	}
	conn, err := net.DialTimeout("tcp4", listener.Addr().String(), 2*time.Second)
	if err != nil {
		return err
	}
	defer func() { fmt.Fprintf(w, "socket-close dial=%s\n", result(conn.Close())) }()
	return socketValues(w, "dial", conn.(*net.TCPConn))
}

func socketValues(w io.Writer, kind string, socket syscall.Conn) error {
	raw, err := socket.SyscallConn()
	if err != nil {
		return err
	}
	return raw.Control(func(fd uintptr) {
		for _, option := range []struct {
			name          string
			level, number int
		}{
			{"SO_REUSEADDR", unix.SOL_SOCKET, unix.SO_REUSEADDR},
			{"SO_REUSEPORT", unix.SOL_SOCKET, unix.SO_REUSEPORT},
			{"SO_KEEPALIVE", unix.SOL_SOCKET, unix.SO_KEEPALIVE},
			{"SO_NOSIGPIPE", unix.SOL_SOCKET, unix.SO_NOSIGPIPE},
			{"SO_BROADCAST", unix.SOL_SOCKET, unix.SO_BROADCAST},
			{"SO_LINGER", unix.SOL_SOCKET, unix.SO_LINGER},
			{"TCP_NODELAY", unix.IPPROTO_TCP, unix.TCP_NODELAY},
			{"TCP_KEEPALIVE", unix.IPPROTO_TCP, unix.TCP_KEEPALIVE},
			{"TCP_KEEPINTVL", unix.IPPROTO_TCP, unix.TCP_KEEPINTVL},
			{"TCP_KEEPCNT", unix.IPPROTO_TCP, unix.TCP_KEEPCNT},
		} {
			if option.number == unix.SO_LINGER && option.level == unix.SOL_SOCKET {
				value, e := unix.GetsockoptLinger(int(fd), option.level, option.number)
				fmt.Fprintf(w, "socket %s %s value=%+v result=%s\n", kind, option.name, value, result(e))
			} else {
				value, e := unix.GetsockoptInt(int(fd), option.level, option.number)
				fmt.Fprintf(w, "socket %s %s value=%d result=%s\n", kind, option.name, value, result(e))
			}
		}
	})
}

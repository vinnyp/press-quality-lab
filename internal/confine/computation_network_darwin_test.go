//go:build darwin && macos_evidence

package confine_test

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Keep the exact operation bodies shared by the controls and confined probes.
// The unconfined listen control binds loopback to keep its endpoint local.
// The confined witness cannot bind first: the engine isolates the listen operation.
const computationNetworkFunctions = `
func network(op, target string) {
 kind:=syscall.SOCK_STREAM;domain:=syscall.AF_INET
 if op=="udp" || op=="bind" {kind=syscall.SOCK_DGRAM};if op=="unix" {domain=syscall.AF_UNIX}
 fd,e:=syscall.Socket(domain,kind,0);check(e);defer func(){check(syscall.Close(fd))}()
 fmt.Println("attempt_"+op)
 switch op {
 case "tcp","unix":
  var addr syscall.Sockaddr
  if op=="unix" {addr=&syscall.SockaddrUnix{Name:target}} else {port,e:=strconv.Atoi(target);check(e);addr=&syscall.SockaddrInet4{Port:port,Addr:[4]byte{127,0,0,1}}}
  check(syscall.Connect(fd,addr));n,e:=syscall.Write(fd,[]byte("network_control"));check(e);if n!=15{panic("short write")}
 case "udp":
  port,e:=strconv.Atoi(target);check(e);check(syscall.Sendto(fd,[]byte("network_control"),0,&syscall.SockaddrInet4{Port:port,Addr:[4]byte{127,0,0,1}}))
 case "bind","listen":
  if op=="bind" {check(syscall.Bind(fd,&syscall.SockaddrInet4{Addr:[4]byte{127,0,0,1}}))} else {if target=="control"{check(syscall.Bind(fd,&syscall.SockaddrInet4{Addr:[4]byte{127,0,0,1}}))};check(syscall.Listen(fd,1))}
  addr,e:=syscall.Getsockname(fd);check(e)
  if target=="control" {a:=addr.(*syscall.SockaddrInet4).Addr;fmt.Printf("address %d.%d.%d.%d\n",a[0],a[1],a[2],a[3])}
  fmt.Printf("ready %d\n",addr.(*syscall.SockaddrInet4).Port)
  readFD:=fd
  if op=="listen" {readFD,_,e=syscall.Accept(fd);check(e);defer func(){check(syscall.Close(readFD))}()}
  b:=make([]byte,32);n,_,e:=syscall.Recvfrom(readFD,b,0);check(e);if string(b[:n])!="network_control"{panic("wrong payload")}
 }
 fmt.Println("returned network_control")
}
`

func networkDecoy(t *testing.T, op string) (string, func()) {
	t.Helper()
	if op == "bind" || op == "listen" {
		return "", func() {}
	}
	if op == "udp" {
		c, e := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() {
			if e := c.Close(); e != nil {
				t.Error(e)
			}
		})
		addr, ok := c.LocalAddr().(*net.UDPAddr)
		if !ok {
			t.Fatalf("UDP address type %T", c.LocalAddr())
		}
		return strconv.Itoa(addr.Port), func() {
			if e := c.SetReadDeadline(time.Now().Add(3 * time.Second)); e != nil {
				t.Fatal(e)
			}
			b := make([]byte, 32)
			n, _, e := c.ReadFromUDP(b)
			if e != nil || string(b[:n]) != "network_control" {
				t.Fatalf("UDP decoy payload=%q err=%v", b[:n], e)
			}
		}
	}
	address := "127.0.0.1:0"
	kind := "tcp4"
	if op == "unix" {
		// Darwin limits Unix socket paths to 104 bytes; the default test
		// temporary root plus the row name can exceed that limit.
		t.Setenv("TMPDIR", "/tmp")
		kind = "unix"
		address = filepath.Join(t.TempDir(), "socket")
	}
	l, e := net.Listen(kind, address)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := l.Close(); e != nil {
			t.Error(e)
		}
	})
	target := address
	if op == "tcp" {
		addr, ok := l.Addr().(*net.TCPAddr)
		if !ok {
			t.Fatalf("TCP address type %T", l.Addr())
		}
		target = strconv.Itoa(addr.Port)
	}
	return target, func() {
		deadline := time.Now().Add(3 * time.Second)
		switch l := l.(type) {
		case *net.TCPListener:
			if e := l.SetDeadline(deadline); e != nil {
				t.Fatal(e)
			}
		case *net.UnixListener:
			if e := l.SetDeadline(deadline); e != nil {
				t.Fatal(e)
			}
		}
		c, e := l.Accept()
		if e != nil {
			t.Fatal(e)
		}
		defer func() {
			if e := c.Close(); e != nil {
				t.Error(e)
			}
		}()
		if e := c.SetReadDeadline(deadline); e != nil {
			t.Fatal(e)
		}
		b, e := io.ReadAll(c)
		if e != nil || string(b) != "network_control" {
			t.Fatalf("decoy payload=%q err=%v", b, e)
		}
	}
}

func TestComputationNetworkControls(t *testing.T) {
	// QH9, QH20: each unrestricted operation changes socket state and transfers
	// a known payload; an unreachable decoy or a no-op cannot satisfy the control.
	source := `package main
import("fmt";"os";"strconv";"syscall")
func check(e error){if e!=nil{panic(e)}}
func main(){network(os.Args[1],os.Args[2])}
` + computationNetworkFunctions
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	program := filepath.Join(dir, "program")
	if e := os.WriteFile(path, []byte(source), 0600); e != nil {
		t.Fatal(e)
	}
	goPath, e := exec.LookPath("go")
	if e != nil {
		t.Fatal(e)
	}
	p, e := os.StartProcess(goPath, []string{goPath, "build", "-o", program, path}, &os.ProcAttr{Env: append(os.Environ(), "CGO_ENABLED=0"), Files: []*os.File{nil, os.Stdout, os.Stderr}})
	if e != nil {
		t.Fatal(e)
	}
	st, e := p.Wait()
	if e != nil || !st.Success() {
		t.Fatalf("build=%v err=%v", st, e)
	}
	for _, op := range []string{"tcp", "unix", "udp", "bind", "listen"} {
		t.Run(op, func(t *testing.T) {
			target, received := networkDecoy(t, op)
			if op == "listen" {
				target = "control"
			}
			r, w, e := os.Pipe()
			if e != nil {
				t.Fatal(e)
			}
			defer func() {
				if e := r.Close(); e != nil {
					t.Error(e)
				}
			}()
			p, e := os.StartProcess(program, []string{program, op, target}, &os.ProcAttr{Files: []*os.File{nil, w, w}})
			if closeErr := w.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
			if e != nil {
				t.Fatal(e)
			}
			// All receives have a deadline, and the parent always reaps its own child.
			defer func() {
				st, e := p.Wait()
				if e != nil || !st.Success() {
					t.Errorf("control=%v err=%v", st, e)
				}
			}()
			if e := r.SetReadDeadline(time.Now().Add(5 * time.Second)); e != nil {
				t.Fatal(e)
			}
			s := bufio.NewScanner(r)
			if !s.Scan() || s.Text() != "attempt_"+op {
				t.Fatalf("attempt=%q err=%v", s.Text(), s.Err())
			}
			if op == "listen" {
				if !s.Scan() || s.Text() != "address 127.0.0.1" {
					t.Fatalf("listen address=%q err=%v", s.Text(), s.Err())
				}
			}
			if op == "bind" || op == "listen" {
				if !s.Scan() {
					t.Fatalf("missing ready: %v", s.Err())
				}
				port, e := strconv.Atoi(strings.TrimPrefix(s.Text(), "ready "))
				if e != nil || port <= 0 {
					t.Fatalf("ready=%q err=%v", s.Text(), e)
				}
				kind := "udp4"
				if op == "listen" {
					kind = "tcp4"
				}
				c, e := net.DialTimeout(kind, fmt.Sprintf("127.0.0.1:%d", port), time.Second)
				if e != nil {
					t.Fatal(e)
				}
				n, e := c.Write([]byte("network_control"))
				closeErr := c.Close()
				if e != nil || n != 15 || closeErr != nil {
					t.Fatalf("send=%d err=%v close=%v", n, e, closeErr)
				}
			}
			if !s.Scan() || s.Text() != "returned network_control" {
				t.Fatalf("result=%q err=%v", s.Text(), s.Err())
			}
			if s.Scan() || s.Err() != nil {
				t.Fatalf("extra output=%q err=%v", s.Text(), s.Err())
			}
			received()
		})
	}
}

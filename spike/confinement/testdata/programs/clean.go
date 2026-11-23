package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

var workResult uint64

func preemption() bool {
	runtime.GOMAXPROCS(1)
	started, finished := make(chan struct{}), make(chan struct{})
	var ended atomic.Bool
	timer := time.NewTimer(time.Millisecond)
	defer timer.Stop()
	go func() {
		close(started)
		var value uint64 = 1
		for i := uint64(0); i < 1<<27; i++ {
			value = (value^i)*6364136223846793005 + 1
		}
		workResult = value
		ended.Store(true)
		close(finished)
	}()
	<-started
	<-timer.C
	preempted := !ended.Load()
	<-finished
	return preempted
}

func main() {
	if !preemption() {
		os.Exit(8)
	}
	if len(os.Args) != 2 {
		os.Exit(64)
	}
	var workers sync.WaitGroup
	for range 4 {
		workers.Go(func() {
			for range 10000 {
				runtime.Gosched()
			}
		})
	}
	runtime.GC()
	workers.Wait()
	switch os.Args[1] {
	case "ordinary":
		data := make([]byte, 32)
		if _, err := rand.Read(data); err != nil {
			os.Exit(20)
		}
		executable, err := os.Executable()
		if err != nil {
			os.Exit(21)
		}
		entries, err := os.ReadDir(filepath.Dir(executable))
		if err != nil || len(entries) != 1 {
			os.Exit(22)
		}
		cwd, err := os.Getwd()
		if err != nil || cwd != os.Getenv("HOME") {
			os.Exit(23)
		}
		host, err := os.Hostname()
		if err != nil || host == "" {
			os.Exit(24)
		}
		if _, err := time.LoadLocation("America/New_York"); err != nil {
			os.Exit(25)
		}
		if _, err := io.ReadAll(os.Stdin); err != nil {
			os.Exit(26)
		}
		dir, err := os.MkdirTemp("", "ordinary-")
		if err != nil {
			os.Exit(27)
		}
		path := filepath.Join(dir, "file")
		if err := os.WriteFile(path, []byte("value"), 0600); err != nil {
			os.Exit(28)
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != "value" {
			os.Exit(29)
		}
		if os.Rename(path, path+"-next") != nil || os.Chmod(path+"-next", 0400) != nil {
			os.Exit(30)
		}
		if os.RemoveAll(dir) != nil {
			os.Exit(31)
		}
		fmt.Println("ordinary ok")
	case "protocol-name":
		address, err := net.ResolveIPAddr("ip:icmp", "127.0.0.1")
		if err != nil {
			os.Exit(33)
		}
		fmt.Println(address.String())
	case "service-port":
		// Exercise Go's service-file resolver rather than libc delegation.
		net.DefaultResolver.PreferGo = true
		port, err := net.LookupPort("tcp", "https")
		if err != nil {
			os.Exit(32)
		}
		fmt.Println(port)
	case "resolver":
		if _, err := fmt.Fprint(os.Stderr, "operation-ready\n"); err != nil {
			os.Exit(70)
		}
		if _, err := net.LookupHost("decoy.invalid."); err == nil {
			os.Exit(9)
		}
		fmt.Println("lookup failed")
	case "help":
		fmt.Println("usage: synthetic [help|user|exit]")
	case "user":
		u, err := user.Current()
		if err != nil || u.Uid == "" || u.Username == "" {
			os.Exit(7)
		}
		fmt.Println("user ok")
	case "exit":
		fmt.Println("exit three")
		os.Exit(3)
	default:
		os.Exit(64)
	}
}

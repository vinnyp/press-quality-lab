package main

import (
	"fmt"
	"net"
	"os"
	"os/user"
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
	case "resolver":
		if _, err := net.LookupHost("decoy.invalid"); err == nil {
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

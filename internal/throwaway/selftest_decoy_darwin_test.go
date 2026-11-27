package throwaway

import (
	"io"
	"os"
	"testing"
	"time"
)

func TestSelfTestDecoyHelper(t *testing.T) {
	// QH9: native helper lifecycle is exercised only in the parent's owned cwd.
	if os.Getenv("PQL_DECOY_LIFETIME_HELPER") != "1" {
		return
	}
	// Exercise the native lifetime read without creating network endpoints.
	if _, err := os.Stdout.Write([]byte("pql-dec:fake!")); err != nil {
		os.Exit(70)
	}
	os.Exit(selfTestDecoyParent())
}

func TestSelfTestDecoyParentEOF(t *testing.T) {
	// QH9, QE20: the engine's lifetime pipe closes on engine death.
	readyR, readyW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	lifeR, lifeW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := readyR.Close(); err != nil {
			t.Error(err)
		}
	}()
	dir := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	p, err := os.StartProcess(executable, []string{executable, "-test.run=^TestSelfTestDecoyHelper$"}, &os.ProcAttr{Dir: dir, Env: []string{"PQL_DECOY_LIFETIME_HELPER=1"}, Files: []*os.File{nil, readyW, readyW, lifeR}})
	if err != nil {
		t.Fatal(err)
	}
	if err := readyW.Close(); err != nil {
		t.Fatal(err)
	}
	if err := lifeR.Close(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		state, err := p.Wait()
		if err == nil && !state.Success() {
			err = &decoyExitError{}
		}
		done <- err
	}()
	if err := readyR.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var b [12]byte
	_, readErr := io.ReadFull(readyR, b[:])
	if err := lifeW.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		if err := p.Kill(); err != nil {
			t.Error(err)
		}
		if err := <-done; err == nil {
			t.Error("killed helper succeeded")
		}
		t.Fatal("decoy did not exit on parent EOF")
	}
	if readErr != nil || string(b[:8]) != "pql-dec:" {
		t.Fatalf("readiness=%q err=%v", b, readErr)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("helper left entries=%v err=%v", entries, err)
	}
}

type decoyExitError struct{}

func (*decoyExitError) Error() string { return "decoy helper failed" }

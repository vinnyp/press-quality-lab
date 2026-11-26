package grade

import (
	"bytes"
	"errors"
	"github.com/vinnyp/press-quality-lab/internal/collect"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/vinnyp/press-quality-lab/internal/throwaway"
)

func TestFunnelEndFailureKeepsPublication(t *testing.T) {
	// QH23, QE20, E-11: ending failures preserve and name the directory.
	for _, mode := range []string{"Control", "Error", "Panic"} {
		t.Run(mode, func(t *testing.T) {
			o, d, _ := fixture(t)
			var w *throwaway.Workspace
			begin := d.Workspace
			var original []byte
			d.Workspace = func() (*throwaway.Workspace, error) {
				var err error
				w, err = begin()
				if err == nil {
					original, err = os.ReadFile(filepath.Join(w.Path(), "process.json"))
				}
				return w, err
			}
			called := false
			d.EndGroups = func(got *throwaway.Workspace, report io.Writer) error {
				called = true
				if got != w {
					t.Error("wrong workspace")
				}
				if _, err := os.Stat(filepath.Join(w.Path(), "process.json")); err != nil {
					t.Error("removed before end", err)
				}
				if mode == "Control" {
					return got.EndGroups(report)
				}
				if mode == "Panic" {
					panic("ending panic")
				}
				return errors.New("ending failed")
			}
			var output bytes.Buffer
			code, err := Run(o, d, &output)
			if mode == "Control" {
				if code != 4 || err != nil || !called {
					t.Fatalf("control: %d %v called=%v", code, err, called)
				}
				if _, err := os.Stat(w.Path()); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("control directory remains: %v", err)
				}
				return
			}
			var classified *Error
			if code != 70 || !errors.As(err, &classified) || classified.Rule != "QH23" || !called {
				t.Fatalf("result: %d %v called=%v", code, err, called)
			}
			if !strings.Contains(output.String(), "cleanup failed; directory left behind: "+Escape(w.Path())+"\n") {
				t.Fatalf("directory unnamed: %q", output.String())
			}
			data, err := os.ReadFile(filepath.Join(w.Path(), "process.json"))
			if err != nil || !bytes.Equal(data, original) {
				t.Fatalf("publication lost: %q %v", data, err)
			}
		})
	}
}

func TestFunnelRemovePanicAndPriorCause(t *testing.T) {
	// QE16, QE20: cleanup panics are engine faults and preserve earlier causes.
	o, d, _ := fixture(t)
	d.Open = func(string) (io.ReadCloser, error) { return nil, errors.New("original read failure") }
	d.Remove = func(*throwaway.Workspace) error { panic("remove panic") }
	var output bytes.Buffer
	code, err := Run(o, d, &output)
	if code != 70 || err == nil || !strings.Contains(err.Error(), "remove panic") || !strings.Contains(err.Error(), "original read failure") {
		t.Fatalf("lost cause: %d %v", code, err)
	}
	if !strings.Contains(output.String(), "cleanup failed; directory left behind:") {
		t.Fatalf("unnamed: %q", output.String())
	}
}

func TestSignalCollectorWaitBound(t *testing.T) {
	// KTD17, QH23, QE20: a stuck collector leaves its workspace for a later sweep.
	o, d, _ := fixture(t)
	signals := make(chan os.Signal, 1)
	d.Signals = signals
	release := make(chan struct{})
	done := make(chan struct{})
	d.Collect = func(plan.Plan, observe.Source) ([]collect.Check, error) {
		defer close(done)
		signals <- syscall.SIGTERM
		<-release
		return nil, nil
	}
	var output bytes.Buffer
	type result struct {
		code int
		err  error
	}
	returned := make(chan result, 1)
	go func() { code, err := Run(o, d, &output); returned <- result{code, err} }()
	var r result
	select {
	case r = <-returned:
	case <-time.After(6 * time.Second):
		close(release)
		<-done
		<-returned
		t.Fatal("collector wait was not bounded")
	}
	code, err := r.code, r.err
	close(release)
	<-done
	if code != 70 || err == nil || !strings.Contains(err.Error(), "collector did not stop after signal") {
		t.Fatalf("unbounded collector: %d %v", code, err)
	}
	if !strings.Contains(output.String(), "cleanup failed; directory left behind:") {
		t.Fatalf("workspace unnamed: %q", output.String())
	}
}

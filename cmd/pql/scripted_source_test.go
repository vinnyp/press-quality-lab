//go:build scripted

package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/scripted"
)

func TestScriptedFileInputs(t *testing.T) {
	// QT1, KTD16: the caller reads files and the byte parser rejects oversized JSON.
	host := []byte(`{"confinement":{"applied":true,"sandbox_exec_exit":null},"self_test":"passed","known_fast":"within","read_calibration":"moved","store_control":"detected","read_control":"killed","private_copy":"unchanged","kill_attribution":[]}`)
	recording, err := os.ReadFile("../../docs/specs/conformance-collection-1.0/examples/behaviour-02.json")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	hp, bp, tp := filepath.Join(dir, "host"), filepath.Join(dir, "recording"), filepath.Join(dir, "trace")
	write := func(path string, data []byte) {
		t.Helper()
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(hp, host)
	write(bp, recording)
	write(tp, []byte("prefix\n"))
	options := commandOptions{Host: hp, Behaviour: bp}
	t.Setenv(scripted.TraceEnv, tp)
	source, closeTrace, err := buildScriptedSource(options)
	if err != nil {
		t.Fatal(err)
	}
	writeErr := source.Required([]any{})
	if err := errors.Join(writeErr, closeTrace()); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(tp)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "prefix\n{\"kind\":\"required\",\"value\":[]}\n" {
		t.Fatalf("trace %q", got)
	}
	for _, row := range []struct {
		name            string
		host, behaviour []byte
	}{
		{"DuplicateHost", []byte(`{"x":1,"x":2}`), recording},
		{"DuplicateRecording", host, []byte(`{"x":1,"x":2}`)},
		{"LargeHost", append(host, bytes.Repeat([]byte(" "), (16<<20)-len(host)+1)...), recording},
		{"LargeRecording", host, append(recording, bytes.Repeat([]byte(" "), (16<<20)-len(recording)+1)...)},
	} {
		t.Run(row.name, func(t *testing.T) {
			write(hp, row.host)
			write(bp, row.behaviour)
			_, _, err := buildScriptedSource(options)
			var refusal *scripted.Error
			if !errors.As(err, &refusal) || refusal.Code != 2 || refusal.Rule != "QT1" {
				t.Fatalf("refusal %v", err)
			}
		})
	}
	write(hp, host)
	write(bp, recording)
	for _, options := range []commandOptions{{Host: filepath.Join(dir, "absent"), Behaviour: bp}, {Host: hp, Behaviour: filepath.Join(dir, "absent")}, {Host: dir, Behaviour: bp}} {
		_, _, err := buildScriptedSource(options)
		var refusal *scripted.Error
		if !errors.As(err, &refusal) || refusal.Rule != "QT1" {
			t.Fatalf("I/O refusal %v", err)
		}
	}
	t.Setenv(scripted.TraceEnv, dir)
	if _, _, err := buildScriptedSource(options); err == nil {
		t.Fatal("trace open failure ignored")
	}
	t.Setenv(scripted.TraceEnv, "")
	_, closeTrace, err = buildScriptedSource(options)
	if err != nil {
		t.Fatal(err)
	}
	if err := closeTrace(); err != nil {
		t.Fatal(err)
	}
}

type scriptedReadCloser struct {
	io.Reader
	closeErr error
	closed   bool
}

func (r *scriptedReadCloser) Close() error { r.closed = true; return r.closeErr }

type scriptedReadFailure struct{ err error }

func (r scriptedReadFailure) Read([]byte) (int, error) { return 0, r.err }

func TestScriptedReadErrorsAndBound(t *testing.T) {
	// QT1, KTD16: retain read and close failures and read at most the limit plus one.
	readErr, closeErr := errors.New("read failure"), errors.New("close failure")
	for _, row := range []struct {
		name     string
		reader   io.Reader
		closeErr error
		wantRead bool
	}{
		{"Read", scriptedReadFailure{readErr}, nil, true},
		{"Close", bytes.NewReader(nil), closeErr, false},
		{"Both", scriptedReadFailure{readErr}, closeErr, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			file := &scriptedReadCloser{Reader: row.reader, closeErr: row.closeErr}
			_, err := readScriptedInput(file)
			if !file.closed || errors.Is(err, readErr) != row.wantRead || (row.closeErr != nil && !errors.Is(err, closeErr)) {
				t.Fatalf("closed=%t error=%v", file.closed, err)
			}
		})
	}
	input := bytes.NewReader(bytes.Repeat([]byte(" "), (16<<20)+2))
	file := &scriptedReadCloser{Reader: input}
	data, err := readScriptedInput(file)
	if err != nil || !file.closed || len(data) != (16<<20)+1 || input.Len() != 1 {
		t.Fatalf("bytes=%d remaining=%d closed=%t error=%v", len(data), input.Len(), file.closed, err)
	}
}

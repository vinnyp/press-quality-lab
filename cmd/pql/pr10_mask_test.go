package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestPR10ProcessMasking(t *testing.T) {
	// QE18, KTD16: both command streams and the error funnel mask before escaping.
	original := runGrade
	t.Cleanup(func() { runGrade = original })
	value := "abc\tdefgh"
	runGrade = func(o commandOptions) error {
		o.Masker.AddDeclared(value)
		for _, w := range []io.Writer{o.Output, o.Diagnostic} {
			if _, err := io.WriteString(w, value[:4]); err != nil {
				return err
			}
			if _, err := io.WriteString(w, value[4:]+"\n"); err != nil {
				return err
			}
		}
		return errors.New(value)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"grade", "synthetic.json"}, strings.NewReader(""), &stdout, &stderr)
	if code != 70 || stdout.String() != "*********\n" || stderr.String() != "*********\npql: *********\n" {
		t.Fatalf("code %d stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
}

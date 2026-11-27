package grade

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"io"
	"testing"
)

func TestScopeRefusals(t *testing.T) {
	// E-7, X1: invalid scope lists reach the pre-check and never reach collection.
	for _, row := range []struct {
		name  string
		scope []string
	}{
		{"DestructiveAlone", []string{"destructive"}},
		{"Unknown", []string{"conformance", "unknown"}},
		{"Repeated", []string{"conformance", "conformance"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			o, d, _ := fixture(t)
			o.Scope = row.scope
			d.Source = func() (observe.Source, func() error, error) { t.Fatal("source reached"); return nil, nil, nil }
			code, err := Run(o, d, io.Discard)
			assertFailure(t, code, err, 2, "QE6", "X1: user caused pre-check rejection")
		})
	}
}

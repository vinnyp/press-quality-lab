//go:build scripted

package grade

import (
	"bytes"
	"io"
	"os"
	"testing"

	"encoding/json"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/scripted"
	"reflect"
	"sort"
)

type failingTrace struct{}

func (failingTrace) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestScriptedRequiredFailure(t *testing.T) {
	// QT5, QE16: a real scripted source propagates a required-trace write failure.
	recording, err := os.ReadFile("../../docs/specs/conformance-collection-1.0/examples/behaviour-01.json")
	if err != nil {
		t.Fatal(err)
	}
	host := []byte(`{"confinement":{"applied":true,"sandbox_exec_exit":null},"self_test":"failed","known_fast":"within","read_calibration":"moved","store_control":"detected","read_control":"killed","private_copy":"unchanged","kill_attribution":[]}`)
	for _, name := range []string{"Failure", "Control"} {
		t.Run(name, func(t *testing.T) {
			var trace bytes.Buffer
			var writer io.Writer = &trace
			if name == "Failure" {
				writer = failingTrace{}
			}
			src, err := scripted.Read(host, recording, writer)
			if err != nil {
				t.Fatal(err)
			}
			o, d, saved := fixture(t)
			o.Scripted = true
			d.Source = func() (observe.Source, func() error, error) { return src, func() error { return nil }, nil }
			code, err := Run(o, d, io.Discard)
			if name == "Failure" {
				assertFailure(t, code, err, 70, "QE16", "io: read/write on closed pipe")
				if saved.JudgeInput != nil {
					t.Fatal("stored after trace failure")
				}
			} else {
				if code != 4 || err != nil {
					t.Fatalf("control %d %v", code, err)
				}
				var event struct {
					Kind  string
					Value []struct {
						Dimension string
						Crediting bool
					}
				}
				if err := json.Unmarshal(trace.Bytes(), &event); err != nil {
					t.Fatal(err)
				}
				var dimensions []string
				for _, item := range event.Value {
					if !item.Crediting {
						t.Fatal("control lost crediting")
					}
					dimensions = append(dimensions, item.Dimension)
				}
				sort.Strings(dimensions)
				want := []string{"availability", "bounding", "discoverability", "execution", "pii-secret-hygiene", "side-effect-integrity"}
				if event.Kind != "required" || !reflect.DeepEqual(dimensions, want) || saved.JudgeInput == nil {
					t.Fatalf("control: %+v", event)
				}

			}
		})
	}
}

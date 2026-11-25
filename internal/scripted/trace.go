//go:build scripted

package scripted

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

// TraceEnv names the corpus runner's append-only JSON event file.
const TraceEnv = "PQL_SCRIPTED_TRACE"

// Trace writes the required set before any served invocation or created home.
// A nil writer disables output, but retains sequencing checks.
type Trace struct {
	writer   io.Writer
	required bool
}

func NewTrace(writer io.Writer) *Trace { return &Trace{writer: writer} }
func wire(v any) any {
	switch x := v.(type) {
	case map[string]any:
		o := object{}
		for k, v := range x {
			o[k] = wire(v)
		}
		return o
	case object:
		o := object{}
		for k, v := range x {
			o[k] = wire(v)
		}
		return o
	case []string:
		a := array{}
		for _, v := range x {
			a = append(a, v)
		}
		return a
	case []any:
		a := array{}
		for _, v := range x {
			a = append(a, wire(v))
		}
		return a
	case array:
		a := array{}
		for _, v := range x {
			a = append(a, wire(v))
		}
		return a
	case json.Number:
		return number(x)
	default:
		return v
	}
}
func externalKeyID(key observe.Key) (string, error) { return keyID(wire(key)), nil }
func (t *Trace) append(kind string, value any) error {
	if t.writer == nil {
		return nil
	}
	data := strictjson.Write(object{"kind": kind, "value": wire(value)})
	n, err := t.writer.Write(data)
	if err == nil && n != len(data) {
		return io.ErrShortWrite
	}
	return err
}
func (t *Trace) Required(value any) error {
	if t.required {
		return fmt.Errorf("QT5: required trace already fixed")
	}
	if _, ok := wire(value).(array); !ok {
		return fmt.Errorf("QT5: required trace must be an array")
	}
	if err := t.append("required", value); err != nil {
		return err
	}
	t.required = true
	return nil
}
func (t *Trace) Served(key observe.Key) error {
	if !t.required && t.writer != nil {
		return fmt.Errorf("QT5: invocation before required trace")
	}
	return t.append("served", key)
}
func (t *Trace) Home(home string) error {
	if !t.required {
		return fmt.Errorf("QT5: home before required trace")
	}
	if home == "" {
		return fmt.Errorf("QT5: empty home trace")
	}
	return t.append("home", home)
}
func (s *Source) Required(value any) error { return s.trace.Required(value) }
func (s *Source) Home(home string) error   { return s.trace.Home(home) }

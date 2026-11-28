//go:build scripted

package scripted

import (
	"errors"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"testing"
)

func TestFix1ReaderIntegers(t *testing.T) {
	// QT5, QE10: validation precedes count conversion; exact judge boundary is accepted.
	base := decode(t, readFile(t, kit+"/examples/behaviour-01.json"))
	for _, row := range []struct {
		name   string
		change func(object)
		code   int
	}{
		{"Boundary", func(b object) {
			inv := obj(arr(b["invocations"])[0])
			for _, v := range arr(inv["confined"]) {
				s := obj(obj(v)["stdout"])
				s["bytes"] = number("9007199254740991")
				s["cut"] = true
				s["kept"] = nil
			}
		}, 0},
		{"DuplicateBeforeCount", func(b object) {
			invs := arr(b["invocations"])
			inv := obj(invs[0])
			for _, v := range arr(inv["confined"]) {
				s := obj(obj(v)["stdout"])
				s["bytes"] = number("9007199254740992")
				s["cut"] = true
				s["kept"] = nil
			}
			b["invocations"] = append(invs, inv)
		}, 2},
		{"AgreementBeforeCount", func(b object) {
			inv := obj(arr(b["invocations"])[0])
			runs := arr(inv["confined"])
			s := obj(obj(runs[0])["stdout"])
			s["bytes"] = number("9007199254740992")
			s["cut"] = true
			s["kept"] = nil
		}, 2},
		{"AttributionBeforeCount", func(b object) {
			inv := obj(arr(b["invocations"])[0])
			for _, v := range arr(inv["confined"]) {
				r := obj(v)
				r["end"] = object{"kind": "signal", "signal": "SIGKILL", "by": "confinement"}
				obj(r["markers"])["confinement"] = false
				s := obj(r["stdout"])
				s["bytes"] = number("9007199254740992")
				s["cut"] = true
				s["kept"] = nil
			}
		}, 2},
		{"RepeatedControlOverflow", func(b object) {
			inv := obj(arr(b["invocations"])[0])
			for _, v := range arr(inv["confined"])[1:2] {
				obj(v)["planted"] = object{"atime_moved": true, "same_file": true, "control_end": object{"kind": "exit", "code": number("9007199254740992")}}
			}
		}, 2},
	} {
		t.Run(row.name, func(t *testing.T) {
			b := decode(t, strictjson.Write(base))
			row.change(b)
			_, err := Read([]byte(passing), strictjson.Write(b), nil)
			var e *Error
			if row.code == 0 {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.As(err, &e) || e.Code != row.code {
				t.Fatalf("error %v want %d", err, row.code)
			}
			if row.name == "RepeatedControlOverflow" && e.Detail != "recording shape refused" {
				t.Fatalf("invalid repeated end reached %v", err)
			}
		})
	}
	for _, value := range []string{"9007199254740992", "9223372036854775807"} {
		h := decode(t, []byte(passing))
		h["confinement"] = object{"applied": false, "sandbox_exec_exit": number(value)}
		s, err := Read(strictjson.Write(h), strictjson.Write(base), nil)
		if err != nil {
			t.Fatal(err)
		}
		c, err := s.Controls()
		if err != nil {
			t.Fatal(err)
		}
		if c.SandboxExecExitExact.String() != value || c.SandboxExecExit == nil || int64(*c.SandboxExecExit) <= observe.MaxCount {
			t.Fatalf("exit lost: %+v", c)
		}
	}
}
func TestFix1NullReadControl(t *testing.T) {
	// QT4, QO7, QP11: a host control is served even when the recorded control is null.
	b := decode(t, readFile(t, kit+"/examples/behaviour-01.json"))
	h := decode(t, []byte(passing))
	h["read_calibration"] = "moved"
	h["read_control"] = "killed"
	inv := obj(arr(b["invocations"])[0])
	for _, v := range arr(inv["confined"]) {
		r := obj(v)
		r["end"] = object{"kind": "exit", "code": number("0")}
		r["markers"] = object{"confinement": true, "could_not_start": false}
		r["planted"] = object{"atime_moved": true, "same_file": true, "control_end": nil}
	}
	s, err := Read(strictjson.Write(h), strictjson.Write(b), nil)
	if err != nil {
		t.Fatal(err)
	}
	key, ok := inv["key"].(object)
	if !ok {
		t.Fatal("missing key")
	}
	r, err := s.Observe(map[string]any(key), observe.Spec{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Details.Planted == nil || r.Details.Planted.ControlEnd != nil {
		t.Fatal("ordinary run control")
	}
	r, err = s.Observe(map[string]any(key), observe.Spec{Control: true})
	if err != nil {
		t.Fatal(err)
	}
	CheckAttribution(t, r, "confinement")
}

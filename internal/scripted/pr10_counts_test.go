//go:build scripted

package scripted

import (
	"errors"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"testing"
)

func TestPR10RecordingCounts(t *testing.T) {
	// QE10: the scripted source fails closed on counts outside the judge range.
	b := decode(t, readFile(t, kit+"/examples/behaviour-01.json"))
	h := decode(t, []byte(passing))
	if _, err := Read(strictjson.Write(h), strictjson.Write(b), nil); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ name, integer string }{{"JudgeOverflow", "9007199254740992"}, {"Int64Overflow", "9223372036854775808"}, {"Exponent", "1e30"}} {
		t.Run(row.name, func(t *testing.T) {
			copy := decode(t, strictjson.Write(b))
			first := obj(arr(obj(arr(copy["invocations"])[0])["confined"])[0])
			// A cut stream permits an arbitrarily large drained byte count.
			stream := obj(first["stdout"])
			stream["bytes"] = number(row.integer)
			stream["cut"] = true
			stream["kept"] = nil
			inv := obj(arr(copy["invocations"])[0])
			inv["confined"] = array{first, first, first, first, first}
			_, err := Read(strictjson.Write(h), strictjson.Write(copy), nil)
			var fault *Error
			if !errors.As(err, &fault) || fault.Code != 70 {
				t.Fatalf("got %v; want engine fault", err)
			}
		})
	}
	h["confinement"] = object{"applied": false, "sandbox_exec_exit": number("9007199254740992")}
	if _, err := Read(strictjson.Write(h), strictjson.Write(b), nil); err == nil {
		t.Fatal("accepted sandbox exit overflow")
	}
}

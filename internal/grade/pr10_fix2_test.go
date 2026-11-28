package grade

import (
	"bytes"
	"encoding/json"
	"github.com/vinnyp/press-quality-lab/internal/judge"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"strings"
	"testing"
)

func TestFix2TerminalJSON(t *testing.T) {
	// KTD16, QE17: stdout escapes controls, format characters, separators and tags; storage is exact.
	for _, row := range []struct{ name, value, escape string }{
		{"Control", "\u0085", `\u0085`},
		{"Format", "\u202e", `\u202e`},
		{"Line", "\u2028", `\u2028`},
		{"Paragraph", "\u2029", `\u2029`},
		{"Tag", "\U000e0001", `\udb40\udc01`},
	} {
		t.Run(row.name, func(t *testing.T) {
			o, d, saved := fixture(t)
			o.JSON = true
			calls := 0
			raw := []byte(`{"exit_code":0,"outcome":"graded","marker":"` + row.value + `"}`)
			d.Judge = func(b []byte) ([]byte, int) {
				calls++
				if calls == 1 {
					return judge.Judge(b)
				}
				return raw, 0
			}
			var out bytes.Buffer
			if code, err := Run(o, d, &out); err != nil || code != 0 {
				t.Fatalf("code %d error %v", code, err)
			}
			if !bytes.Contains(out.Bytes(), []byte(`"marker":"`+row.escape+`"`)) || bytes.Contains(out.Bytes(), []byte(row.value)) {
				t.Fatalf("stdout %q", out.Bytes())
			}
			var decoded map[string]any
			if err := json.Unmarshal(out.Bytes(), &decoded); err != nil || decoded["marker"] != row.value {
				t.Fatalf("decoded %v %v", decoded, err)
			}
			if !strings.Contains(string(strictjson.Write(saved.JudgeOutput)), `"marker":"`+row.value+`"`) {
				t.Fatalf("stored bytes %q", strictjson.Write(saved.JudgeOutput))
			}
		})
	}
}

package record

import (
	"encoding/base64"
	"encoding/hex"
	"io"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

// Form names one row of credit-and-routes section 4.
type Form struct{ Name, Text string }

// EncodeForms is the shared leak-scan and display-masking encoder.
func EncodeForms(value string) []Form {
	b := []byte(value)
	lower := hex.EncodeToString(b)
	upper := strings.ToUpper(lower)
	percent := func(h string) string {
		var s strings.Builder
		for i := 0; i < len(h); i += 2 {
			s.WriteByte('%')
			s.WriteString(h[i : i+2])
		}
		return s.String()
	}
	return []Form{
		{"plain", value},
		{"base64", base64.StdEncoding.EncodeToString(b)},
		{"base64-nopad", base64.RawStdEncoding.EncodeToString(b)},
		{"base64url", base64.URLEncoding.EncodeToString(b)},
		{"base64url-nopad", base64.RawURLEncoding.EncodeToString(b)},
		{"hex-lower", lower},
		{"hex-upper", upper},
		{"percent-upper", percent(upper)},
		{"percent-lower", percent(lower)},
	}
}

type replacement struct{ from, to string }

// Masker can learn invocation canaries while its writers remain installed.
// It never changes its caller's raw evidence.
type Masker struct {
	mu    sync.RWMutex
	forms []replacement
}

func NewMasker(canaries, declared []string) *Masker {
	m := &Masker{}
	for _, v := range canaries {
		m.AddCanary(v)
	}
	for _, v := range declared {
		m.AddDeclared(v)
	}
	return m
}
func (m *Masker) add(value string, canary bool) {
	if value == "" {
		return
	}
	forms := EncodeForms(value)
	zero := EncodeForms("pqlcanary" + strings.Repeat("0", 32))
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, f := range forms {
		to := strings.Repeat("*", utf8.RuneCountInString(f.Text))
		if canary {
			to = zero[i].Text
		}
		m.forms = append(m.forms, replacement{f.Text, to})
	}
	sort.SliceStable(m.forms, func(i, j int) bool { return len(m.forms[i].from) > len(m.forms[j].from) })
}
func (m *Masker) AddCanary(value string) { m.add(value, true) }
func (m *Masker) AddDeclared(value string) {
	if len(value) >= 8 {
		m.add(value, false)
	}
}

// transform retains a trailing possible match for the next Write.
func (m *Masker) transform(s string, final bool) (string, string) {
	if m == nil {
		return s, ""
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out strings.Builder
	for len(s) > 0 {
		matched := false
		for _, r := range m.forms {
			if !final && len(s) < len(r.from) && strings.HasPrefix(r.from, s) {
				return out.String(), s
			}
			if strings.HasPrefix(s, r.from) {
				out.WriteString(r.to)
				s = s[len(r.from):]
				matched = true
				break
			}
		}
		if !matched {
			out.WriteByte(s[0])
			s = s[1:]
		}
	}
	return out.String(), ""
}
func (m *Masker) Mask(s string) string { out, _ := m.transform(s, true); return out }

// MaskingWriter buffers only a possible trailing match, across write boundaries.
// A caller must flush at the process's exit funnel.
type MaskingWriter struct {
	mu      sync.Mutex
	masker  *Masker
	out     io.Writer
	pending string
	err     error
}

func (m *Masker) Writer(out io.Writer) *MaskingWriter {
	if existing, ok := out.(*MaskingWriter); ok && existing.masker == m {
		return existing
	}
	return &MaskingWriter{masker: m, out: out}
}
func (w *MaskingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return 0, w.err
	}
	text, tail := w.masker.transform(w.pending+string(p), false)
	w.pending = tail
	w.err = writeAll(w.out, text)
	if w.err != nil {
		return 0, w.err
	}
	return len(p), nil
}
func (w *MaskingWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return w.err
	}
	w.err = writeAll(w.out, w.masker.Mask(w.pending))
	w.pending = ""
	return w.err
}
func writeAll(w io.Writer, s string) error {
	n, err := io.WriteString(w, s)
	if err == nil && n != len(s) {
		return io.ErrShortWrite
	}
	return err
}

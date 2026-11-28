package record

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"io"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

// Form names one row of credit-and-routes section 4.
type Form = observe.Form

// EncodeForms shares the collection encoder without a second reading path.
func EncodeForms(value string) []Form { return observe.EncodeForms(value) }

type replacement struct {
	from, to string
	canary   bool
}

// Masker can learn invocation canaries while its writers remain installed.
// It never changes its caller's raw evidence.
type Masker struct {
	mu         sync.RWMutex
	forms      []replacement
	index      [256][]replacement
	comparison func()
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
		m.forms = append(m.forms, replacement{f.Text, to, canary})
	}
	sort.SliceStable(m.forms, func(i, j int) bool { return len(m.forms[i].from) > len(m.forms[j].from) })
	m.index = [256][]replacement{}
	for _, r := range m.forms {
		m.index[r.from[0]] = append(m.index[r.from[0]], r)
	}
}
func (m *Masker) AddCanary(value string) { m.add(value, true) }
func (m *Masker) AddDeclared(value string) {
	if len(value) >= 8 {
		m.add(value, false)
	}
}

// coverage finds matches in start order and merges declared intervals once.
// Previously covered tail bytes survive when a match starts in an earlier write.
func (m *Masker) coverage(s string, covered, replacementBytes []byte, final bool) (int, []byte, []byte) {
	if m == nil {
		return len(s), make([]byte, len(s)), make([]byte, len(s))
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	covered = append(covered, make([]byte, len(s)-len(covered))...)
	replacementBytes = append(replacementBytes, make([]byte, len(s)-len(replacementBytes))...)
	type interval struct {
		start, end int
		r          replacement
	}
	matches := []interval{}
	end := len(s)
	for i := 0; i < len(s); i++ {
		for _, r := range m.index[s[i]] {
			if m.comparison != nil {
				m.comparison()
			}
			if strings.HasPrefix(s[i:], r.from) {
				matches = append(matches, interval{i, i + len(r.from), r})
			} else if !final && len(s)-i < len(r.from) && strings.HasPrefix(r.from, s[i:]) {
				end = min(end, i)
			}
		}
	}
	mark := func(start, stop int) {
		for j := start; j < stop; j++ {
			if covered[j] != 2 {
				covered[j] = 1
				if utf8.RuneStart(s[j]) {
					replacementBytes[j] = '*'
				}
			}
		}
	}
	start, stop := -1, 0
	for _, match := range matches {
		if match.r.canary {
			continue
		}
		if start < 0 {
			start, stop = match.start, match.end
		} else if match.start <= stop {
			stop = max(stop, match.end)
		} else {
			mark(start, stop)
			start, stop = match.start, match.end
		}
	}
	if start >= 0 {
		mark(start, stop)
	}
	for _, match := range matches {
		if !match.r.canary {
			continue
		}
		for j := match.start; j < match.end; j++ {
			if covered[j] != 2 {
				covered[j] = 2
				replacementBytes[j] = match.r.to[j-match.start]
			}
		}
	}
	return end, covered, replacementBytes
}
func coveredText(s string, covered, replacementBytes []byte) string {
	var out strings.Builder
	for i := range len(s) {
		if covered[i] == 0 {
			out.WriteByte(s[i])
		} else if replacementBytes[i] != 0 {
			out.WriteByte(replacementBytes[i])
		}
	}
	return out.String()
}

// transform masks a complete value or the safe prefix of a recorded head.
func (m *Masker) transform(s string, final bool) (string, string) {
	end, covered, replacementBytes := m.coverage(s, nil, nil, final)
	// A cut head has no continuation to inspect. Keep already masked bytes,
	// even when a partial periodic match starts inside the covered run.
	for end < len(s) && covered[end] != 0 {
		end++
	}
	return coveredText(s[:end], covered, replacementBytes), s[end:]
}
func (m *Masker) Mask(s string) string { out, _ := m.transform(s, true); return out }

// MaskingWriter buffers only a possible trailing match, across write boundaries.
// A caller must flush at the process's exit funnel.
type MaskingWriter struct {
	mu               sync.Mutex
	masker           *Masker
	out              io.Writer
	pending          string
	covered          []byte
	replacementBytes []byte
	err              error
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
	s := w.pending + string(p)
	end, covered, replacementBytes := w.masker.coverage(s, w.covered, w.replacementBytes, false)
	text := coveredText(s[:end], covered, replacementBytes)
	w.pending = s[end:]
	w.covered = append([]byte(nil), covered[end:]...)
	w.replacementBytes = append([]byte(nil), replacementBytes[end:]...)
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
	_, covered, replacementBytes := w.masker.coverage(w.pending, w.covered, w.replacementBytes, true)
	w.err = writeAll(w.out, coveredText(w.pending, covered, replacementBytes))
	w.covered = nil
	w.replacementBytes = nil
	w.pending = ""
	return w.err
}
func writeAll(w io.Writer, s string) error {
	if s == "" {
		return nil
	}
	n, err := io.WriteString(w, s)
	if err == nil && n != len(s) {
		return io.ErrShortWrite
	}
	return err
}

// RawWriter returns the destination for already masked structured documents.
func RawWriter(w io.Writer) io.Writer {
	for {
		masked, ok := w.(*MaskingWriter)
		if !ok {
			return w
		}
		if err := masked.Flush(); err != nil {
			return failedWriter{err}
		}
		w = masked.out
	}
}

type failedWriter struct{ err error }

func (w failedWriter) Write([]byte) (int, error) { return 0, w.err }

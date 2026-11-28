package decide

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"strings"
)

// ComputedValueDepth and ComputedValueBytes retain QA6's shared bounds.
const ComputedValueDepth = observe.ComputedValueDepth
const ComputedValueBytes = observe.ComputedValueBytes

// ParseComputed delegates to the sole QA6 reader used by the producer too.
func ParseComputed(data []byte, cut bool, count int64) (any, bool) {
	return observe.ParseComputed(data, cut, count)
}

// Substitute replaces QA4 tokens after argv[0], without rescanning inserted paths.
func Substitute(argv []string, seed, subjectDir string, captures map[string]string) []string {
	result := append([]string(nil), argv...)
	for i := 1; i < len(result); i++ {
		s := result[i]
		var b strings.Builder
		for j := 0; j < len(s); {
			switch {
			case strings.HasPrefix(s[j:], "{SEED}"):
				b.WriteString(seed)
				j += 6
			case strings.HasPrefix(s[j:], "{SUBJECT_DIR}"):
				b.WriteString(subjectDir)
				j += 13
			case strings.HasPrefix(s[j:], "{CAPTURE:"):
				end := strings.IndexByte(s[j+9:], '}')
				if end >= 0 {
					name := s[j+9 : j+9+end]
					if validCaptureName(name) {
						p, ok := captures[name]
						if !ok {
							panic("QA4: missing capture path")
						}
						b.WriteString(p)
						j += 10 + end
						continue
					}
				}
				b.WriteByte(s[j])
				j++
			default:
				b.WriteByte(s[j])
				j++
			}
		}
		result[i] = b.String()
	}
	return result
}
func validCaptureName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < 32 || r == '{' || r == '}' || r == ':' {
			return false
		}
	}
	return true
}

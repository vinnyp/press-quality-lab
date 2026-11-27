package record

import (
	"crypto/sha256"
	"fmt"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"strings"
	"unicode/utf8"
)

// Stream builds QE10's four fields from raw capture facts. Masking precedes
// both display cuts; the digest and byte count always describe unmasked bytes.
func Stream(raw []byte, seen int64, cut bool, details observe.StreamDetails, masker *Masker) Object {
	kept := raw
	if len(kept) > 1<<20 {
		kept = kept[:1<<20]
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(kept))
	display := string(raw)
	if details.Count != "" && !details.Kept {
		// A sealed cut recording supplies its digest and decoded display head,
		// rather than kept bytes. Real captures always supply raw bytes here.
		return Object{"bytes": seen, "sha256": details.Digest, "cut": cut, "head": masker.Mask(details.Head)}
	}
	display = masker.Mask(display)
	if len(display) > 64<<10 {
		display = display[:64<<10]
	}
	var head strings.Builder
	for len(display) > 0 {
		r, n := utf8.DecodeRuneInString(display)
		head.WriteRune(r)
		display = display[n:]
	}
	return Object{"bytes": seen, "sha256": digest, "cut": cut, "head": head.String()}
}

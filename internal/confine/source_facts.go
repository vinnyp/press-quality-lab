package confine

import (
	"crypto/sha256"
	"fmt"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"unicode/utf8"
)

func sourceStream(raw []byte, cut bool) observe.StreamDetails {
	head := raw[:min(len(raw), 64<<10)]
	runes := make([]rune, 0, len(head))
	for len(head) > 0 {
		r, n := utf8.DecodeRune(head)
		runes = append(runes, r)
		head = head[n:]
	}
	return observe.StreamDetails{Digest: fmt.Sprintf("%x", sha256.Sum256(raw)), Head: string(runes), Kept: !cut}
}

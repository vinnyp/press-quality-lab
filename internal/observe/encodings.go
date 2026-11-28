package observe

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
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

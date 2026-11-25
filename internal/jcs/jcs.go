// Package jcs canonicalises strictjson values according to RFC 8785.
package jcs

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

// Canonical returns UTF-8 canonical JSON without a newline. Invalid values,
// cycles and numbers that lose decimal value through binary64 return an error.
func Canonical(v any) ([]byte, error) {
	return appendValue(nil, v, make(map[identity]bool))
}

// Digest returns the lowercase hexadecimal SHA-256 of Canonical(v).
func Digest(v any) (string, error) {
	b, err := Canonical(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func quote(dst []byte, s string) ([]byte, error) {
	if !utf8.ValidString(s) {
		return nil, fmt.Errorf("invalid UTF-8 string")
	}
	dst = append(dst, '"')
	for _, c := range s {
		switch c {
		case '"', '\\':
			dst = append(dst, '\\', byte(c))
		case '\b':
			dst = append(dst, '\\', 'b')
		case '\f':
			dst = append(dst, '\\', 'f')
		case '\n':
			dst = append(dst, '\\', 'n')
		case '\r':
			dst = append(dst, '\\', 'r')
		case '\t':
			dst = append(dst, '\\', 't')
		default:
			if c < 32 {
				const digits = "0123456789abcdef"
				dst = append(dst, '\\', 'u', '0', '0', digits[c>>4], digits[c&15])
			} else {
				dst = utf8.AppendRune(dst, c)
			}
		}
	}
	return append(dst, '"'), nil
}

func less(a, b string) bool {
	x, y := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	for i := 0; i < len(x) && i < len(y); i++ {
		if x[i] != y[i] {
			return x[i] < y[i]
		}
	}
	return len(x) < len(y)
}

type identity struct {
	kind    reflect.Kind
	pointer uintptr
}

func appendValue(dst []byte, v any, active map[identity]bool) ([]byte, error) {
	switch v.(type) {
	case strictjson.Object, strictjson.Array:
		key := identity{reflect.TypeOf(v).Kind(), uintptr(reflect.ValueOf(v).UnsafePointer())}
		if active[key] {
			return nil, fmt.Errorf("cyclic JSON value")
		}
		active[key] = true
		defer delete(active, key)
	}
	switch x := v.(type) {
	case nil:
		return append(dst, "null"...), nil
	case bool:
		return strconv.AppendBool(dst, x), nil
	case string:
		return quote(dst, x)
	case strictjson.Number:
		s, err := number(x)
		if err != nil {
			return nil, err
		}
		return append(dst, s...), nil
	case strictjson.Array:
		dst = append(dst, '[')
		for i, child := range x {
			if i > 0 {
				dst = append(dst, ',')
			}
			var err error
			dst, err = appendValue(dst, child, active)
			if err != nil {
				return nil, err
			}
		}
		return append(dst, ']'), nil
	case strictjson.Object:
		keys := make([]string, 0, len(x))
		for k := range x {
			if !utf8.ValidString(k) {
				return nil, fmt.Errorf("invalid UTF-8 key")
			}
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return less(keys[i], keys[j]) })
		dst = append(dst, '{')
		for i, k := range keys {
			if i > 0 {
				dst = append(dst, ',')
			}
			var err error
			dst, err = quote(dst, k)
			if err != nil {
				return nil, err
			}
			dst = append(dst, ':')
			dst, err = appendValue(dst, x[k], active)
			if err != nil {
				return nil, err
			}
		}
		return append(dst, '}'), nil
	default:
		return nil, fmt.Errorf("unsupported JSON value %T", v)
	}
}

// decimalKey compares decimal values without allocating powers of ten. The
// exponent remains arbitrary precision even for manually constructed Numbers.
func decimalKey(s string) string {
	negative := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	exponent := new(big.Int)
	if p := strings.IndexAny(s, "eE"); p >= 0 {
		exponent.SetString(strings.TrimPrefix(s[p+1:], "+"), 10)
		s = s[:p]
	}
	if p := strings.IndexByte(s, '.'); p >= 0 {
		exponent.Sub(exponent, big.NewInt(int64(len(s)-p-1)))
		s = s[:p] + s[p+1:]
	}
	s = strings.TrimLeft(s, "0")
	if s == "" {
		return "0"
	}
	digits := strings.TrimRight(s, "0")
	exponent.Add(exponent, big.NewInt(int64(len(s)-len(digits))))
	if negative {
		digits = "-" + digits
	}
	return digits + "e" + exponent.String()
}

func number(n strictjson.Number) (string, error) {
	raw := string(n)
	// Read validates the grammar, including manually constructed Number values.
	v, ok := strictjson.Read([]byte(raw), strictjson.Bounds{Bytes: len(raw), Depth: 1, NumberChars: len(raw), ExponentDigits: len(raw)}, nil)
	if _, numeric := v.(strictjson.Number); !ok || !numeric {
		return "", fmt.Errorf("invalid JSON number %q", raw)
	}
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return "", fmt.Errorf("number outside binary64: %q", raw)
	}
	result := "0"
	if f != 0 {
		format := byte('e')
		if math.Abs(f) >= 1e-6 && math.Abs(f) < 1e21 {
			format = 'f'
		}
		result = strconv.FormatFloat(f, format, -1, 64)
		if p := strings.IndexByte(result, 'e'); p >= 0 {
			power, parseErr := strconv.Atoi(result[p+1:])
			if parseErr != nil {
				return "", parseErr
			}
			sign := ""
			if power >= 0 {
				sign = "+"
			}
			result = result[:p+1] + sign + strconv.Itoa(power)
		}
	}
	if decimalKey(raw) != decimalKey(result) {
		return "", fmt.Errorf("number does not round-trip through binary64: %q", raw)
	}
	return result, nil
}

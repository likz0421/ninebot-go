package crypto

import (
	"encoding/base64"
	"strings"
)

// AndroidB64 mimics android.util.Base64.NO_WRAP: standard alphabet,
// no line breaks. (DecodeAndroidB64 tolerates both standard and
// URL-safe input on the way back.)
func AndroidB64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

// DecodeAndroidB64 is tolerant of both standard and URL-safe input.
func DecodeAndroidB64(s string) ([]byte, error) {
	s = stripAllWhitespace(s)
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.URLEncoding.DecodeString(s)
}

func stripAllWhitespace(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r':
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// passportPublicKeyPEM is the hardcoded RSA-1024 public key embedded in
// the Ninebot app (Passport password encryption). Extracted verbatim
// from the reference binary.
const passportPublicKeyPEM = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQC6maFY3dEhgav1147RW2gVWzCv
agkiRySnCDRSTM67YhHvLcrUSMnngxJl0A2liFLJydpn65E58oh0Phtu+t4Kkkfe
GIHsr931wRMRtkila4F/RF3U5pqSt42k/10U087QEhGMGvdOzF/5ziGXJod6ovBx
yk6pJlzNhxLTVJSzkQIDAQAB
-----END PUBLIC KEY-----`

var passportPubKey *rsa.PublicKey

func init() {
	block, _ := pem.Decode([]byte(passportPublicKeyPEM))
	if block == nil {
		panic("crypto: bad embedded passport public key")
	}
	k, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		panic("crypto: parse passport public key: " + err.Error())
	}
	pub, ok := k.(*rsa.PublicKey)
	if !ok {
		panic("crypto: embedded passport key is not RSA")
	}
	passportPubKey = pub
}

// PassportEncryptRSA encrypts short payloads (login password) with the
// embedded app public key, returning Android-style base64.
func PassportEncryptRSA(plain []byte) (string, error) {
	out, err := rsa.EncryptPKCS1v15(rand.Reader, passportPubKey, plain)
	if err != nil {
		return "", err
	}
	return AndroidB64(out), nil
}

// Passport protocol constants recovered from the reference binary
// (gitea.com/ninebot/cli) and verified against a live capture
// (POST /v5/user reproduced byte-for-byte).
const (
	// PassportClientID is the fixed clientId sent on every Passport request.
	PassportClientID = "vehicle_app_prod"
	// PassportClientKey is the per-build signing key embedded in the app.
	PassportClientKey = "e177176a-3b3e-1513-e26e-d1123034cb66"
	// PassportAppVersion / OS / OSLanguage / OSVersion are the four fixed
	// fields mixed into both the canonical string and the header set.
	PassportAppVersion = "610103831"
	PassportOS         = "ios"
	PassportOSLanguage = "zh-hans-cn"
	PassportOSVersion  = "27.0"
)

// PassportCanonicalString builds the deterministic canonical string that
// the Passport "sign" header covers. It is the sorted, "&"-joined list of
// "k=v" pairs over: clientKey, url, timestamp, the four fixed fields
// (app_version/os/os_language/os_version) and every request param.
//
// Verified against FOUR independent captures:
//   - PyPI reference (Android): url=/v5/user, params={},
//     ts=1789965521546 -> sign f559cb8d...e4769
//   - iOS app (3 distinct requests): url=/v5/user,
//     ts=1789786169488 -> sign 5be65140...6d4e,
//     ts=1789786159887 -> sign 2fba2b15...7871,
//     ts=1789786083735 -> sign c1fe0d30...3735.
//     iOS uses the SAME clientKey (e177176a-...) with platform values
//     swapped (app_version=610103831, os=ios, os_language=zh,
//     os_version=27.0)
//     — the canonical structure is platform-invariant.
func PassportCanonicalString(url string, params map[string]string, timestamp string) string {
	m := map[string]string{
		"clientKey":   PassportClientKey,
		"url":         url,
		"timestamp":   timestamp,
		"app_version": PassportAppVersion,
		"os":          PassportOS,
		"os_language": PassportOSLanguage,
		"os_version":  PassportOSVersion,
	}
	for k, v := range params {
		m[k] = v
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte('&')
		}
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(m[k])
	}
	return sb.String()
}

// PassportSign returns the "sign" header value: the lowercase hex SHA-256
// digest of the canonical string (plain hash, no HMAC).
func PassportSign(canonical string) string {
	sum := sha256.Sum256([]byte(canonical))
	return hexEncode(sum[:])
}

// EncodeOrderedJSON serializes v preserving struct field order (signing
// requires byte-stable output, unlike encoding/json's map sorting).
func EncodeOrderedJSON(v any) ([]byte, error) {
	var sb strings.Builder
	if err := writeJSONValue(&sb, v, false); err != nil {
		return nil, err
	}
	return []byte(sb.String()), nil
}

// EncodeOrderedJSONOpen is EncodeOrderedJSON without the closing brace,
// letting callers append extra fields (BuildInner appends checkcode).
func EncodeOrderedJSONOpen(v any) ([]byte, error) {
	var sb strings.Builder
	if om, ok := v.(*OrderedMap); ok {
		sb.WriteByte('{')
		for i, kv := range om.Entries {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(jsonEscapeString(kv.Key, false))
			sb.WriteByte(':')
			if err := writeJSONValue(&sb, kv.Value, false); err != nil {
				return nil, err
			}
		}
		return []byte(sb.String()), nil
	}
	return EncodeOrderedJSON(v)
}

func writeJSONValue(sb *strings.Builder, v any, escape bool) error {
	switch t := v.(type) {
	case nil:
		sb.WriteString("null")
	case bool:
		if t {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}
	case string:
		sb.WriteString(jsonEscapeString(t, escape))
	case int:
		sb.WriteString(strconv.Itoa(t))
	case int64:
		sb.WriteString(strconv.FormatInt(t, 10))
	case float64:
		sb.WriteString(strconv.FormatFloat(t, 'f', -1, 64))
	case []any:
		sb.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				sb.WriteByte(',')
			}
			if err := writeJSONValue(sb, e, escape); err != nil {
				return err
			}
		}
		sb.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		sb.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(jsonEscapeString(k, escape))
			sb.WriteByte(':')
			if err := writeJSONValue(sb, t[k], escape); err != nil {
				return err
			}
		}
		sb.WriteByte('}')
	case OrderedMap:
		sb.WriteByte('{')
		for i, kv := range t.Entries {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(jsonEscapeString(kv.Key, escape))
			sb.WriteByte(':')
			if err := writeJSONValue(sb, kv.Value, escape); err != nil {
				return err
			}
		}
		sb.WriteByte('}')
	default:
		return fmt.Errorf("encode: unsupported type %T", v)
	}
	return nil
}

// KV is one ordered key/value entry.
type KV struct {
	Key   string
	Value any
}

// OrderedMap preserves insertion order for JSON signing.
type OrderedMap struct {
	Entries []KV
}

// Set appends (or replaces) a key.
func (m *OrderedMap) Set(k string, v any) {
	for i := range m.Entries {
		if m.Entries[i].Key == k {
			m.Entries[i].Value = v
			return
		}
	}
	m.Entries = append(m.Entries, KV{Key: k, Value: v})
}

// Get returns a value by key.
func (m *OrderedMap) Get(k string) (any, bool) {
	for _, e := range m.Entries {
		if e.Key == k {
			return e.Value, true
		}
	}
	return nil, false
}

func jsonEscapeString(s string, escape bool) string {
	var sb strings.Builder
	sb.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		default:
			if r < 0x20 {
				sb.WriteString(`\u`)
				var b [4]byte
				b[0] = lowerHexNibble(byte(r >> 4))
				b[1] = lowerHexNibble(byte(r))
				sb.WriteString("00")
				sb.WriteByte(b[0])
				sb.WriteByte(b[1])
			} else if escape && (r == '<' || r == '>' || r == '&') {
				sb.WriteString(`\u`)
				sb.WriteByte(upperHexNibble(byte(r >> 12)))
				sb.WriteByte(lowerHexNibble(byte(r >> 8)))
				sb.WriteByte(lowerHexNibble(byte(r >> 4)))
				sb.WriteByte(lowerHexNibble(byte(r)))
			} else {
				sb.WriteRune(r)
			}
		}
	}
	sb.WriteByte('"')
	return sb.String()
}


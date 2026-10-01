package crypto

import (
	"crypto/md5"
	"encoding/hex"
	"strings"
)

// ComputeCheckcode returns the inner-request "checkcode" field:
// UPPERCASE hex of the standard MD5 over the ordered-JSON payload
// (reference binary routes md5.Sum through strings.ToUpper).
func ComputeCheckcode(orderedJSON []byte) string {
	sum := md5.Sum(orderedJSON)
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

// MD5HexLower is the envelope "h" flavor: lowercase hex md5, computed
// inline by crypto.EncryptRequest in the reference binary.
func MD5HexLower(data []byte) string {
	sum := md5.Sum(data)
	return hex.EncodeToString(sum[:])
}

package crypto

import (
	"encoding/hex"
)

func hexEncode(b []byte) string { return hex.EncodeToString(b) }

func rotl32(v uint32, n uint) uint32 {
	n &= 31
	if n == 0 {
		return v
	}
	return v<<n | v>>(32-n)
}

func putLE32(dst []byte, v uint32) {
	dst[0] = byte(v)
	dst[1] = byte(v >> 8)
	dst[2] = byte(v >> 16)
	dst[3] = byte(v >> 24)
}

func lowerHexNibble(v byte) byte {
	v &= 0x0f
	if v < 10 {
		return '0' + v
	}
	return 'a' + v - 10
}

func upperHexNibble(v byte) byte {
	v &= 0x0f
	if v < 10 {
		return '0' + v
	}
	return 'A' + v - 10
}

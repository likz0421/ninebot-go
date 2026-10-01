package crypto

// DeriveKey folds four keyData strings into a 16-byte AES-128 key.
//
// Translated instruction-by-instruction from the reference binary
// (internal/crypto.DeriveKey @ .text). Each string folds into a 32-bit
// word via acc = rol32(acc, 8) ^ byte; the four words are then mixed
// with the T(x) = x ^ rol32(x,8) ^ rol32(x,24) primitive.
func DeriveKey(s1, s2, s3, s4 string) [16]byte {
	w1 := foldString(s1)
	w2 := foldString(s2)
	w3 := foldString(s3)
	w4 := foldString(s4)

	v := w1 ^ w3
	tv := v ^ rotl32(v, 8) ^ rotl32(v, 24)
	a := w2 ^ tv
	b := w4 ^ tv

	u := w2 ^ w4
	tu := u ^ rotl32(u, 8) ^ rotl32(u, 24)
	c := w3 ^ tu
	d := w1 ^ tu

	p0 := (c & a) ^ d
	cb := (c | b) ^ a
	q := (c ^ b) ^ cb
	nq := ^q
	p3 := p0 ^ nq
	p2 := (nq | p0) ^ cb
	p1 := (^cb & p3) ^ b

	var out [16]byte
	putLE32(out[0:4], p1)
	putLE32(out[4:8], p2)
	putLE32(out[8:12], p3)
	putLE32(out[12:16], p0)
	return out
}

// foldString folds a string into a 32-bit word: acc = rol32(acc,8) ^ b.
func foldString(s string) uint32 {
	var acc uint32
	for i := 0; i < len(s); i++ {
		acc = rotl32(acc, 8) ^ uint32(s[i])
	}
	return acc
}

package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"math/big"
)

// charset69 is the alphabet for keyDataFour (reference binary constant).
const charset69 = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789;-_.,+/"

// charset94 is the printable-ASCII alphabet used by GenKReq (0x21..0x7E).
const charset94 = "!\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~"

// KeyData carries one request's random key material.
type KeyData struct {
	// KReq is the 16-char printable-ASCII session key: used directly as
	// the AES-128 key and RSA-encrypted into the "k" envelope field.
	KReq string
	// One/Two/Three/Four are the keyData strings embedded in the
	// encrypted JSON; the response is decrypted with DeriveKey(One..Four).
	One   string
	Two   string
	Three string
	Four  string
}

// GenKeyData produces fresh key material: the 16-char KReq session key
// plus three hex strings (11/11/10) and one 8-char string drawn from
// charset69, mirroring the reference binary's call sequence
// (randHex(11), randHex(11), randHex(10), randFromCharset(8, charset69)).
func GenKeyData() *KeyData {
	return &KeyData{
		KReq:  GenKReq(),
		One:   randHex(11),
		Two:   randHex(11),
		Three: randHex(10),
		Four:  randFromCharset(8, charset69),
	}
}

// GenKReq generates the 16-char session key over charset94
// (printable ASCII 0x21..0x7E) using crypto/rand.Int per byte,
// exactly as the reference implementation does.
func GenKReq() string {
	b := make([]byte, 16)
	max := big.NewInt(int64(len(charset94)))
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			// crypto/rand failure is unrecoverable in practice
			n = big.NewInt(0)
		}
		b[i] = charset94[n.Int64()]
	}
	return string(b)
}

// GenNonce returns n random hex chars (api-layer helper mirror).
func GenNonce(n int) string { return randHex(n) }

func randHex(n int) string {
	b := make([]byte, (n+1)/2)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:n]
}

func randFromCharset(n int, charset string) string {
	out := make([]byte, n)
	max := big.NewInt(int64(len(charset)))
	for i := range out {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			idx = big.NewInt(0)
		}
		out[i] = charset[idx.Int64()]
	}
	return string(out)
}

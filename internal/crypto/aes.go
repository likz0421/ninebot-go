package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
)

// zeroIV is the fixed all-zero CBC IV used by the reference binary
// (cipher.NewCBCEncrypter receives a freshly made, never-filled 16-byte
// slice — Go makeslice memory is zeroed).
var zeroIV = make([]byte, aes.BlockSize)

// AESEncrypt encrypts plaintext with AES-128-CBC (zero IV) + PKCS7 and
// returns standard base64, mirroring crypto.AESEncrypt.
func AESEncrypt(plain, key []byte) (string, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return "", fmt.Errorf("aes: bad key len %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	padded := PKCS7Pad(plain, aes.BlockSize)
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, zeroIV).CryptBlocks(out, padded)
	return base64Std.EncodeToString(out), nil
}

// AESDecrypt decrypts base64 AES-CBC (zero IV) ciphertext and strips
// PKCS7 padding.
func AESDecrypt(b64CipherText string, key []byte) ([]byte, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, fmt.Errorf("aes: bad key len %d", len(key))
	}
	raw, err := base64Std.DecodeString(b64CipherText)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || len(raw)%aes.BlockSize != 0 {
		return nil, errors.New("aes: ciphertext not block-aligned")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(raw))
	cipher.NewCBCDecrypter(block, zeroIV).CryptBlocks(out, raw)
	return PKCS7Unpad(out, aes.BlockSize)
}

// PKCS7Pad pads src to blockSize.
func PKCS7Pad(src []byte, blockSize int) []byte {
	pad := blockSize - len(src)%blockSize
	padded := make([]byte, len(src)+pad)
	copy(padded, src)
	for i := len(src); i < len(padded); i++ {
		padded[i] = byte(pad)
	}
	return padded
}

// PKCS7Unpad removes PKCS7 padding.
func PKCS7Unpad(src []byte, blockSize int) ([]byte, error) {
	if len(src) == 0 || len(src)%blockSize != 0 {
		return nil, errors.New("pkcs7: invalid length")
	}
	pad := int(src[len(src)-1])
	if pad == 0 || pad > blockSize || pad > len(src) {
		return nil, errors.New("pkcs7: invalid padding")
	}
	for _, b := range src[len(src)-pad:] {
		if int(b) != pad {
			return nil, errors.New("pkcs7: corrupt padding")
		}
	}
	return src[:len(src)-pad], nil
}

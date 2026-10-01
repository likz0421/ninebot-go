// Package crypto implements the Netease ("NET") encryption layer used
// by the Ninebot Passport + business APIs.
//
// Wire protocol (recovered from live traffic + reference binary):
//
//	request  {"d": b64(AES(keyDataJSON)), "h": hex(md5(keyDataJSON)),
//	          "k": b64(RSA(kreq)), "p": "101", "t": "0"}
//	response {"v": 101, "s": b64(RSA sig), "r": b64(AES(respJSON))}
//
// where keyDataJSON embeds base64(inner) plus four random keyData
// strings, platform and timeStamp; AES is 128-bit CBC with a zero IV
// and PKCS7 padding; response key = DeriveKey(keyData strings).
package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
)

var base64Std = base64.StdEncoding

// neteasePublicKeyPEM is the RSA-1024 public key used to encrypt the
// envelope "k" field (the AES key request) on the NET encrypted channel.
//
// Source: reference binary (ninecli.exe) global variable parsed by
// crypto.init.0 from the embedded DER blob at 0x1408442c0 — this is
// the key EncryptRequest actually uses for kreq, NOT the Passport
// login key. Using the Passport key here yields server error 4102
// ("通讯异常"); verified live: with this key the gateway returns
// HTTP 200 and a decryptable business response.
const neteasePublicKeyPEM = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDT3m0c/8y9c13PzaFbATEg+Zwd
kpPcCy0V21VKBBSx16ckVtLERAQ7EH8d6DqgEbyzayAwlQd1gDhUmx27hDWafXr9
/evUZkkBegcsNnKrIlh93lPKccjk+LDXS1TnDIIFiTlSbNnaYwehI/9pUbKCI3h7
yE0pum6hJh/9QtGPlwIDAQAB
-----END PUBLIC KEY-----`

var neteasePubKey *rsa.PublicKey

func init() {
	block, _ := pem.Decode([]byte(neteasePublicKeyPEM))
	if block == nil {
		panic("crypto: bad embedded netease public key")
	}
	k, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		panic("crypto: parse netease public key: " + err.Error())
	}
	pub, ok := k.(*rsa.PublicKey)
	if !ok {
		panic("crypto: embedded netease key is not RSA")
	}
	neteasePubKey = pub
}

// NeteaseEncryptRSA encrypts the envelope kreq (AES key request) with
// the Netease public key, returning standard base64.
func NeteaseEncryptRSA(plain []byte) (string, error) {
	out, err := rsa.EncryptPKCS1v15(rand.Reader, neteasePubKey, plain)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(out), nil
}

package crypto

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// Envelope is the wire format the reference client emits (verified
// byte-for-byte against its outbound requests):
//
//	{"d": <b64 AES(keyDataJSON)>, "h": <hex md5(keyDataJSON)>,
//	 "k": <b64 RSA(kreq)>, "p": "101", "t": "0"}
//
// Field order and the string-typed p/t mirror the reference; the server
// parses envelope JSON semantically, so this is fidelity rather than a
// correctness requirement.
type Envelope struct {
	D string `json:"d"`
	H string `json:"h"`
	K string `json:"k"`
	P string `json:"p"`
	T string `json:"t"`
}

// ResponseEnvelope is the server's reply format:
//
//	{"v": 101, "s": <b64 RSA signature>, "r": <b64 AES(response)>}
//
// CALIBRATION (open): the "s" signature does NOT verify under the app's
// embedded passport public key with PKCS1v15 over md5/sha1/sha256 of
// either the raw or base64 "r" (0/184 real captures). The server-side
// verification key must be recovered by other means; the field is
// currently ignored.
type ResponseEnvelope struct {
	V int    `json:"v"`
	S string `json:"s"`
	R string `json:"r"`
}

// EncryptRequestOutput bundles the encrypted payload and the key
// material needed to decrypt the response.
type EncryptRequestOutput struct {
	Body    []byte
	KeyData *KeyData
}

// BuildWrapper renders the keyData JSON plaintext that gets encrypted
// into the "d" field. Keys are emitted in sorted order with the exact
// separator style of the reference binary:
//
//	{
//		"data" : "<b64(inner)>",
//		"keyDataFour" : "...",
//		"keyDataOne" : "...",
//		"keyDataThree" : "...",
//		"keyDataTwo" : "...",
//		"platform" : N,
//		"timeStamp" : M
//	}
func BuildWrapper(inner []byte, kd *KeyData, platform, timeStamp int64) string {
	var sb strings.Builder
	sb.WriteString("{\n")
	sb.WriteString("\t\"data\" : ")
	sb.WriteString(jsonEscapeString(AndroidB64(inner), false))
	sb.WriteString(",\n")
	sb.WriteString("\t\"keyDataFour\" : \"")
	sb.WriteString(kd.Four)
	sb.WriteString("\",\n")
	sb.WriteString("\t\"keyDataOne\" : \"")
	sb.WriteString(kd.One)
	sb.WriteString("\",\n")
	sb.WriteString("\t\"keyDataThree\" : \"")
	sb.WriteString(kd.Three)
	sb.WriteString("\",\n")
	sb.WriteString("\t\"keyDataTwo\" : \"")
	sb.WriteString(kd.Two)
	sb.WriteString("\",\n")
	sb.WriteString("\t\"platform\" : ")
	sb.WriteString(strconv.FormatInt(platform, 10))
	sb.WriteString(",\n")
	sb.WriteString("\t\"timeStamp\" : ")
	sb.WriteString(strconv.FormatInt(timeStamp, 10))
	sb.WriteString("\n}\n")
	return sb.String()
}

// EncryptRequest wraps an inner request body into the encrypted envelope,
// generating fresh key material for every call.
//
//	d  = base64( AES-128-CBC(key=kreq, iv=0, PKCS7(keyDataJSON)) )
//	h  = hex( md5(keyDataJSON) )
//	k  = base64( RSA-PKCS1v15(appPublicKey, kreq) )
func EncryptRequest(inner []byte, platform, timeStamp int64) (*EncryptRequestOutput, error) {
	return EncryptRequestWith(inner, GenKeyData(), platform, timeStamp)
}

// EncryptRequestWith is EncryptRequest with caller-supplied key material.
func EncryptRequestWith(inner []byte, kd *KeyData, platform, timeStamp int64) (*EncryptRequestOutput, error) {
	keyDataJSON := BuildWrapper(inner, kd, platform, timeStamp)

	d, err := AESEncrypt([]byte(keyDataJSON), []byte(kd.KReq))
	if err != nil {
		return nil, err
	}
	h := MD5HexLower([]byte(keyDataJSON))

	// kreq is encrypted with the NETEASE public key (not the Passport
	// login key) — recovered from the reference binary's EncryptRequest:
	// the RSA key it uses is a dedicated global parsed from a separate
	// embedded DER blob. Verified live: with this key the gateway
	// returns HTTP 200; with the Passport key it returns 4102.
	k, err := NeteaseEncryptRSA([]byte(kd.KReq))
	if err != nil {
		return nil, err
	}

	env := Envelope{D: d, H: h, K: k, P: "101", T: "0"}
	body, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}
	return &EncryptRequestOutput{Body: body, KeyData: kd}, nil
}

// DecryptResponse unwraps an encrypted response using the request's
// keyData strings (the server derives the same key via DeriveKey).
func DecryptResponse(body []byte, kd *KeyData) ([]byte, error) {
	var env ResponseEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		// Plaintext error envelope — pass through.
		return body, nil
	}
	if env.R == "" {
		return body, nil
	}
	key := DeriveKey(kd.One, kd.Two, kd.Three, kd.Four)
	return AESDecrypt(env.R, key[:])
}

// DefaultTimeStamp returns the unix-seconds stamp used in wrappers.
func DefaultTimeStamp() int64 { return time.Now().Unix() }

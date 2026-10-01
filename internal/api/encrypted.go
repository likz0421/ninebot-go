package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ninecli/internal/crypto"
)

func ioNopCloser(b []byte) io.ReadCloser { return io.NopCloser(bytes.NewReader(b)) }

// wrapperPlatform is the "platform" value inside the encrypted keyData
// JSON; the reference client sends 2 on the Android business channel.
const wrapperPlatform = 2

// EncryptedPost issues one NET-encrypted call and returns the decrypted
// business payload.
//
// Request body:  {"d": b64(AES(keyDataJSON)), "h": md5(keyDataJSON),
// "k": b64(RSA(kreq)), "p": "101", "t": "0"}; response: {"v","s","r"} with
// r decrypted by DeriveKey(keyData strings). Both directions are verified
// byte-for-byte against the reference client's own traffic.
//
// The inner body is flat: CommonParams, then the endpoint's extras, then
// serviceTime/nonce/checkcode. Identity travels inside it — the business
// channel carries no Access-Token header, and its Uid header must hold the
// *business* uid: the passport uuid makes the gateway answer 4103.
func (c *Client) EncryptedPost(url string, extras []crypto.KV, headers map[string]string) (map[string]any, int, error) {
	inner := BuildInner(append(c.commonParams(), extras...))
	kd := crypto.GenKeyData()
	out, err := crypto.EncryptRequestWith(inner, kd, wrapperPlatform, crypto.DefaultTimeStamp())
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Body = ioNopCloser(out.Body)
	req.GetBody = func() (io.ReadCloser, error) { return ioNopCloser(out.Body), nil }
	h := req.Header
	h.Set("Content-Type", "text/html;charset=UTF-8")
	h.Set("User-Agent", userAgentBiz)
	h.Set("Ninebot-Version", "2")
	h.Set("Client-Ver", appVersion)
	h.Set("Business-Type", c.businessTypeFor(url))
	h.Set("Language", c.Cfg.Lang)
	h.Set("Sys-Language", "zh-hans-cn")
	h.Set("Login-Country", "CN")
	h.Set("Platform", "Android")
	h.Set("Platform-Ver", c.Cfg.OSVersion)
	h.Set("Regionx", c.Cfg.Region)
	h.Set("Device-Id", c.Cfg.DeviceID)
	h.Set("Request-Id", crypto.GenNonce(32))
	h.Set("Service-Time", fmt.Sprintf("%d", time.Now().UnixMilli()))
	h.Set("Cache-Control", "no-cache")
	h.Set("Debug", "0")
	h.Set("Need_decrypt", "1")
	if uid := c.BizUID(); uid != "" {
		h.Set("Uid", uid)
	}
	for k, v := range headers {
		h.Set(k, v)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, err
	}
	raw, err := readMaybeGzip(resp)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	plain, perr := crypto.DecryptResponse(raw, kd)
	if captureEnabled() {
		note := ""
		plainStr := string(plain)
		if perr != nil {
			note = "decrypt_response: " + perr.Error()
			plainStr = ""
		}
		capture(captureRecord{
			Channel:        "encrypted",
			URL:            url,
			RequestHeaders: flattenHeader(req.Header),
			RequestBody:    string(out.Body),
			KeyData: &captureKeyData{
				KReq:  kd.KReq,
				One:   kd.One,
				Two:   kd.Two,
				Three: kd.Three,
				Four:  kd.Four,
			},
			InnerPlain:     string(inner),
			ResponseStatus: resp.StatusCode,
			ResponseBody:   string(raw),
			ResponsePlain:  plainStr,
			Note:           note,
		})
	}
	if perr != nil {
		return nil, resp.StatusCode, perr
	}
	var m map[string]any
	if err := json.Unmarshal(plain, &m); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("decode %s: %w (body %.200s)", url, err, string(plain))
	}
	// The gateway double-wraps: the decrypted body is
	// {"data": b64(businessJSON), "timeStamp": N}. Unwrap so callers get
	// the business envelope {"code":1,"data":...} directly.
	if s, ok := m["data"].(string); ok && s != "" {
		if inner, err := crypto.DecodeAndroidB64(s); err == nil {
			var biz map[string]any
			if json.Unmarshal(inner, &biz) == nil {
				m = biz
			}
		}
	}
	return m, resp.StatusCode, nil
}

// BizUID is the identity the business APIs expect: the short uid handed out
// by business_login (e.g. "87517671"), not the 19-digit passport uuid.
func (c *Client) BizUID() string {
	if c.Tokens == nil {
		return ""
	}
	if c.Tokens.BizUID != "" {
		return c.Tokens.BizUID
	}
	return c.Tokens.UID
}

// businessTypeFor maps a gateway host to its Business-Type header value:
// ebike.ninebot.com sends "2", steeldust / cn-cbu / api-jhcx send "1".
func (c *Client) businessTypeFor(url string) string {
	if c.Hosts.EbikeHost != "" && strings.HasPrefix(url, c.Hosts.EbikeHost) {
		return "2"
	}
	return "1"
}

// commonParams returns the reference client's CommonParams table in its
// exact emission order. platform_ver is the OS version and device model
// joined by a space ("13 Xiaomi").
func (c *Client) commonParams() []crypto.KV {
	f := []crypto.KV{
		{Key: "sys_language", Value: "zh-hans-cn"},
		{Key: "client_ver", Value: appVersion},
		{Key: "device_id", Value: c.Cfg.DeviceID},
		{Key: "regionx", Value: c.Cfg.Region},
		{Key: "language", Value: c.Cfg.Lang},
		{Key: "ostype", Value: "and"},
		{Key: "lang", Value: c.Cfg.Lang},
		{Key: "platform_ver", Value: c.Cfg.PlatformVer()},
		{Key: "platform", Value: "android"},
		{Key: "login_country", Value: "CN"},
	}
	if c.Tokens != nil {
		if c.Tokens.AccessToken != "" {
			f = append(f, crypto.KV{Key: "access_token", Value: c.Tokens.AccessToken})
		}
		if uid := c.BizUID(); uid != "" {
			f = append(f, crypto.KV{Key: "uid", Value: uid})
		}
	}
	return f
}

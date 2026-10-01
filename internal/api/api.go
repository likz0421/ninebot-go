// Package api implements the Ninebot Passport + business HTTP clients.
//
// All business endpoints speak the NET encrypted envelope (see
// internal/crypto): requests are {"d","h","k","p","t"} and responses {"v","s","r"}.
package api

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ninecli/internal/config"
	"ninecli/internal/crypto"
)

// Hosts bundles every upstream base URL (overridable via flags).
type Hosts struct {
	PassportBase string // default https://api-passport-bj.ninebot.com
	BizHost      string // default https://api-jhcx-v6-bj.ninebot.com
	EbikeHost    string // default https://ebike.ninebot.com
	MotorHost    string // default https://steeldust.ninebot.com
	TravelHost   string // default https://cn-cbu-gateway.ninebot.com
}

// DefaultHosts returns the production endpoints.
func DefaultHosts() Hosts {
	return Hosts{
		PassportBase: "https://api-passport-bj.ninebot.com",
		BizHost:      "https://api-jhcx-v6-bj.ninebot.com",
		EbikeHost:    "https://ebike.ninebot.com",
		MotorHost:    "https://steeldust.ninebot.com",
		TravelHost:   "https://cn-cbu-gateway.ninebot.com",
	}
}

// Client talks to the Ninebot APIs using the persisted tokens.
type Client struct {
	HTTP   *http.Client
	Hosts  Hosts
	Cfg    *config.Config
	Tokens *config.Tokens
}

// New builds a client.
func New(h Hosts, cfg *config.Config, tok *config.Tokens) *Client {
	return &Client{
		HTTP:   &http.Client{Timeout: 30 * time.Second},
		Hosts:  h,
		Cfg:    cfg,
		Tokens: tok,
	}
}

const (
	appVersion   = "610063322"
	userAgent    = "Ninebot/6.10.10 (cn.ninebot.segway; build:3831; iOS 27.0.0) Alamofire/6.10.10"
	userAgentBiz = "okhttp/4.9.1"
	netVersion   = "1.0.13"
)

// IsBizSuccess reports whether a business envelope succeeded.
func IsBizSuccess(m map[string]any) bool { return IsSuccess(m) }

// IsSuccess inspects the common business envelope and the passport
// {"resultCode":"90000",...} envelope.
//
// The business gateways signal success with code 1 (verified live: the
// battery/status/travel replies all carry "code":1 next to their data) and
// fail with codes like 340000; 0 appears on the passport-adjacent hosts, so
// both are accepted.
func IsSuccess(m map[string]any) bool {
	if m == nil {
		return false
	}
	if rc, ok := m["resultCode"]; ok {
		if s, ok := rc.(string); ok && s == "90000" {
			return true
		}
	}
	for _, k := range []string{"code", "status"} {
		if v, ok := m[k]; ok {
			if n, ok := v.(float64); ok && (n == 0 || n == 1) {
				return true
			}
		}
	}
	return false
}

// BizMessage pulls the human-readable failure text out of a business
// envelope; the gateways use "desc" (not "msg"/"message").
func BizMessage(m map[string]any) string {
	if m == nil {
		return ""
	}
	for _, k := range []string{"desc", "message", "msg", "resultDesc"} {
		if s := strField(m, k); s != "" {
			return s
		}
	}
	return ""
}

// Error is a structured API failure.
type Error struct {
	Code    string
	Message string
	Status  int
}

func (e *Error) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("%s: %s (http %d)", e.Code, e.Message, e.Status)
	}
	return fmt.Sprintf("%s (http %d)", e.Message, e.Status)
}

// readMaybeGzip transparently decompresses gzip bodies.
func readMaybeGzip(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	r := io.Reader(resp.Body)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(r)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		r = gz
	}
	return io.ReadAll(r)
}

// strField pulls a string field from a decoded JSON object.
func strField(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// BuildInner renders the encrypted inner request body.
//
// The body is a *flat* JSON object: the caller's parameters, then
// serviceTime (a number, unix millis), nonce, and finally the checkcode
// computed over everything preceding it. There is no "cmd" field and no
// "params" sub-object — the endpoint in the URL is what routes the call.
// Field order and shapes verified byte-for-byte against the reference
// client's own requests.
func BuildInner(fields []crypto.KV) []byte {
	om := &crypto.OrderedMap{}
	for _, f := range fields {
		om.Set(f.Key, f.Value)
	}
	om.Set("serviceTime", time.Now().UnixMilli())
	om.Set("nonce", crypto.GenNonce(32))

	partial, err := crypto.EncodeOrderedJSONOpen(om)
	if err != nil {
		// OrderedMap always encodes; this cannot fail in practice.
		partial = []byte("{}")
	}
	cc := crypto.ComputeCheckcode(partial)
	out := make([]byte, 0, len(partial)+len(cc)+20)
	out = append(out, partial...)
	out = append(out, `,"checkcode":"`...)
	out = append(out, cc...)
	out = append(out, '"', '}')
	return out
}

// PostJSON posts a JSON body and decodes a JSON response.
func (c *Client) PostJSON(url string, headers map[string]string, body []byte) (map[string]any, int, error) {
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, 0, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, err
	}
	raw, err := readMaybeGzip(resp)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if captureEnabled() {
		capture(captureRecord{
			Channel:        "passport",
			URL:            url,
			RequestHeaders: flattenHeader(req.Header),
			RequestBody:    string(body),
			ResponseStatus: resp.StatusCode,
			ResponseBody:   string(raw),
		})
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("decode %s: %w (body %.200s)", url, err, string(raw))
	}
	return m, resp.StatusCode, nil
}

// errNoTokens is returned when credentials are missing.
var errNoTokens = errors.New("not logged in (run `ninecli login` first)")

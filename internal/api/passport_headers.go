package api

import (
	"strconv"
	"time"

	"ninecli/internal/crypto"
)

func fmtInt64(v int64) string { return strconv.FormatInt(v, 10) }

// passportHeaders builds the exact header set the reference binary sends
// on api-passport-bj.ninebot.com (recovered from Passport.post and
// verified against a live capture of POST /v5/user). Keys use the
// canonical MIME form because the original assigns them directly into
// req.Header without canonicalization.
//
//	sign = lowercase-hex(sha256(canonical))
//	canonical = sorted "k=v" joined by "&" over
//	  {clientKey, url, timestamp, app_version, os, os_language,
//	   os_version} + params
func (c *Client) passportHeaders(path string, params map[string]string, auth string) map[string]string {
	ts := fmtInt64(time.Now().UnixMilli())
	canonical := crypto.PassportCanonicalString(path, params, ts)
	h := map[string]string{
		"Content-Type": "application/json; charset=UTF-8",
		"Clientid":     crypto.PassportClientID,
		"Timestamp":    ts,
		"Sign":         crypto.PassportSign(canonical),
		"App_version":  crypto.PassportAppVersion,
		"Os":           crypto.PassportOS,
		"Os_language":  crypto.PassportOSLanguage,
		"Os_version":   crypto.PassportOSVersion,
	}
	if auth != "" {
		h["Authorization"] = auth
	}
	return h
}

package proxy

import (
	"errors"

	"ninecli/internal/api"
	"ninecli/internal/config"
)

func asAPIError(err error, dst **api.Error) bool {
	var e *api.Error
	if errors.As(err, &e) {
		*dst = e
		return true
	}
	return false
}

func str(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

func applyTokenFields(tok *config.Tokens, tf *api.TokenFields) {
	if tf == nil {
		return
	}
	if tf.AccessToken != "" {
		tok.AccessToken = tf.AccessToken
	}
	if tf.RefreshToken != "" {
		tok.RefreshToken = tf.RefreshToken
	}
	if tf.TokenType != "" {
		tok.TokenType = tf.TokenType
	}
	if tf.UID != "" {
		tok.UID = tf.UID
	}
	if tf.ExpiresIn > 0 {
		tok.ExpiresIn = tf.ExpiresIn
		tok.ExpiresAt = tf.ExpiresAt
	}
}

// businessExchange mirrors cmd.businessAfterLogin for serve mode.
func businessExchange(c *api.Client, d config.Dir, tok *config.Tokens) error {
	m, err := c.BusinessLogin()
	if err != nil {
		return err
	}
	if !api.IsBizSuccess(m) {
		return errors.New("business_login failed")
	}
	data, _ := m["data"].(map[string]any)
	if data == nil {
		data = m
	}
	if v, ok := data["uid"].(string); ok && v != "" {
		tok.BizUID = v
	}
	if v, ok := data["token"].(string); ok && v != "" {
		tok.BizToken = v
	}
	if v, ok := data["access_token"].(string); ok && v != "" && tok.BizToken == "" {
		tok.BizToken = v
	}
	return d.SaveTokens(tok)
}

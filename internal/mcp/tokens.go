package mcp

import (
	"ninecli/internal/api"
	"ninecli/internal/config"
)

var tokenZero config.Tokens

// applyTokens persists credentials after auth tools.
func applyTokens(d config.Dir, tok *config.Tokens, m map[string]any) error {
	tf := api.ExtractTokens(m)
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
	return d.SaveTokens(tok)
}

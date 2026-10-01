package api

import "time"

// TokenFields is the normalized credential set extracted from a
// login/refresh envelope.
type TokenFields struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	UID          string
	ExpiresIn    int64
	ExpiresAt    int64
}

// ExtractTokens pulls credential fields out of a login/refresh
// envelope. The passport response uses "uuid" (not "user_id")
// for the user identifier, verified against a live capture.
func ExtractTokens(m map[string]any) *TokenFields {
	t := &TokenFields{}
	if data, ok := m["data"].(map[string]any); ok {
		t.AccessToken = strField(data, "access_token")
		t.RefreshToken = strField(data, "refresh_token")
		t.TokenType = strField(data, "token_type")
		t.UID = strField(data, "uuid")
		if v, ok := data["expires_in"].(float64); ok {
			t.ExpiresIn = int64(v)
		}
	} else {
		t.AccessToken = strField(m, "access_token")
		t.RefreshToken = strField(m, "refresh_token")
		t.TokenType = strField(m, "token_type")
		t.UID = strField(m, "uuid")
		if v, ok := m["expires_in"].(float64); ok {
			t.ExpiresIn = int64(v)
		}
	}
	if t.ExpiresIn > 0 {
		t.ExpiresAt = time.Now().Unix() + t.ExpiresIn
	}
	return t
}

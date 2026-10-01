package api

import (
	"encoding/json"
)

// Login performs the Passport password login.
// POST /v6/user/login, params {device, areaCode, username, password}.
func (c *Client) Login(areaCode, username, password string) (map[string]any, error) {
	return c.LoginWithDevice("IOS", areaCode, username, password)
}

// LoginWithDevice is Login with an explicit device parameter
// (controls the JWT "audience" claim; cnnx-omsgw requires mobile-class
// tokens, which are only issued for mobile-class devices).
func (c *Client) LoginWithDevice(device, areaCode, username, password string) (map[string]any, error) {
	params := map[string]string{
		"device":   device,
		"areaCode": areaCode,
		"username": username,
		"password": password,
	}
	return c.postPassport("/v6/user/login", params, "")
}

// SendCode sends an SMS verification code.
// POST /v4/code/phone, params {rawTemplate:"login", areaCode, account}.
func (c *Client) SendCode(areaCode, account string) (map[string]any, error) {
	params := map[string]string{
		"rawTemplate": "login",
		"areaCode":    areaCode,
		"account":     account,
	}
	return c.postPassport("/v4/code/phone", params, "")
}

// PhoneCodeLogin consumes an SMS verification code.
// POST /v6/user/phoneCodeLogin,
// params {device, areaCode, phone, code, region}.
func (c *Client) PhoneCodeLogin(areaCode, phone, code, region string) (map[string]any, error) {
	params := map[string]string{
		"device":   "IOS",
		"areaCode": areaCode,
		"phone":    phone,
		"code":     code,
		"region":   region,
	}
	return c.postPassport("/v6/user/phoneCodeLogin", params, "")
}

// Refresh rotates the access token.
// POST /v3/user/refresh, params {device, accessToken, refreshToken}.
func (c *Client) Refresh() (map[string]any, error) {
	if c.Tokens == nil || c.Tokens.RefreshToken == "" {
		return nil, errNoTokens
	}
	params := map[string]string{
		"device":       "IOS",
		"accessToken":  c.Tokens.AccessToken,
		"refreshToken": c.Tokens.RefreshToken,
	}
	return c.postPassport("/v3/user/refresh", params, "")
}

// V5User verifies the token and returns the profile.
// POST /v5/user with an empty JSON body and the JWT as Bearer-less
// Authorization header (capture-verified).
func (c *Client) V5User() (map[string]any, error) {
	if c.Tokens == nil {
		return nil, errNoTokens
	}
	return c.postPassport("/v5/user", map[string]string{}, c.Tokens.AccessToken)
}

// postPassport issues a plaintext Passport request: a JSON body of the
// params plus the signed header set (Content-Type / Clientid / Timestamp
// / Sign / App_version / Os / Os_language / Os_version, and Authorization
// when auth is non-empty). No NET envelope on this channel.
func (c *Client) postPassport(path string, params map[string]string, auth string) (map[string]any, error) {
	body, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	headers := c.passportHeaders(path, params, auth)
	m, status, err := c.PostJSON(c.Hosts.PassportBase+path, headers, body)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, &Error{Status: status, Message: strField(m, "message")}
	}
	return m, nil
}

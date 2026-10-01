package api

// BusinessLogin exchanges the Passport identity for the short business uid
// the vehicle APIs address the caller by. Endpoint and required response
// field verified against the reference client (`data.uid`).
//
// Like the reference, callers treat this as best-effort: the business hosts
// accept the Passport JWT directly, so a failure here only costs the cached
// uid.
func (c *Client) BusinessLogin() (map[string]any, error) {
	if c.Tokens == nil || c.Tokens.AccessToken == "" {
		return nil, errNoTokens
	}
	m, _, err := c.EncryptedPost(c.Hosts.BizHost+"/user/user/login", nil, nil)
	if err != nil {
		return nil, err
	}
	return m, nil
}

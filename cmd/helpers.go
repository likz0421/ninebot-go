package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"ninecli/internal/api"
	"ninecli/internal/config"
)

// envOr returns the env var value or a fallback.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func hostsOrDefaults() api.Hosts {
	h := api.DefaultHosts()
	if flagPassportBase != "" {
		h.PassportBase = flagPassportBase
	}
	if flagBizHost != "" {
		h.BizHost = flagBizHost
	}
	if flagEbikeHost != "" {
		h.EbikeHost = flagEbikeHost
	}
	if flagMotorHost != "" {
		h.MotorHost = flagMotorHost
	}
	if flagTravelHost != "" {
		h.TravelHost = flagTravelHost
	}
	return h
}

func dirOrDefaults() config.Dir {
	return config.ResolveDir(flagConfig)
}

// newBusinessForCmd loads config+tokens and builds a client, requiring
// a business token when needed.
func newBusinessForCmd() (*api.Client, *config.Dir, error) {
	d := dirOrDefaults()
	cfg, err := d.Load()
	if err != nil {
		return nil, nil, err
	}
	tok, err := d.LoadTokens()
	if err != nil {
		return nil, nil, err
	}
	if tok == nil || tok.AccessToken == "" {
		return nil, nil, fmt.Errorf("not logged in: run `ninecli login` first")
	}
	return api.New(hostsOrDefaults(), cfg, tok), &d, nil
}

// ensureBusinessUID makes sure the business_login exchange happened,
// refreshing via business_login when the biz token is missing.
// If business_login fails, we continue with the passport UUID —
// the real iOS app skips business_login entirely and uses the
// passport JWT + a cached business UID directly.
func ensureBusinessUID(c *api.Client, d *config.Dir, tok *config.Tokens) error {
	if tok.BizUID != "" {
		return nil
	}
	m, err := c.BusinessLogin()
	if err != nil || !api.IsBizSuccess(m) {
		// Non-fatal: fall back to passport UUID for uid headers.
		// The vehicle endpoints may accept the passport JWT directly.
		return nil
	}
	data, _ := m["data"].(map[string]any)
	if data == nil {
		data = m
	}
	if v := strField(data, "uid"); v != "" {
		tok.BizUID = v
	}
	if v := strField(data, "token"); v != "" {
		tok.BizToken = v
	}
	if v := strField(data, "access_token"); v != "" && tok.BizToken == "" {
		tok.BizToken = v
	}
	return d.SaveTokens(tok)
}

// finishLogin persists tokens after any successful login path.
func finishLogin(d *config.Dir, tok *config.Tokens, m map[string]any) error {
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

func strField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func strFieldData(m map[string]any, key string) string {
	if data, ok := m["data"].(map[string]any); ok {
		return strField(data, key)
	}
	return strField(m, key)
}

// cachedVehicleBySN finds a cached vehicle.
func cachedVehicleBySN(d *config.Dir, sn string) (*config.CachedVehicle, error) {
	vs, err := d.LoadVehicleCache()
	if err != nil {
		return nil, err
	}
	for i := range vs {
		if vs[i].SN == sn {
			return &vs[i], nil
		}
	}
	return nil, nil
}

// JSONOutput prints raw decrypted JSON when --json is set, otherwise
// returns false so callers render human output.
func JSONOutput(m map[string]any) bool {
	if !flagJSON {
		return false
	}
	b, _ := marshalIndent(m)
	fmt.Println(string(b))
	return true
}

func vehicleChoicePrompt(vs []config.CachedVehicle) *config.CachedVehicle {
	if len(vs) == 0 {
		return nil
	}
	fmt.Fprintln(os.Stderr, "Vehicles:")
	for i, v := range vs {
		shared := ""
		if v.Shared {
			shared = " (shared)"
		}
		fmt.Fprintf(os.Stderr, "  %d) %s  %s%s\n", i+1, v.SN, v.Name, shared)
	}
	fmt.Fprint(os.Stderr, "Which one? [1]: ")
	var line string
	_, _ = fmt.Fscanln(os.Stdin, &line)
	idx := 0
	if line != "" {
		if n := parseIndex(line, len(vs)); n >= 0 {
			idx = n
		}
	}
	return &vs[idx]
}

func parseIndex(s string, max int) int {
	var n int
	if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &n); err != nil {
		return -1
	}
	if n < 1 || n > max {
		return -1
	}
	return n - 1
}

// bizMsg renders the failure text of a business or passport envelope.
func bizMsg(m map[string]any) string { return api.BizMessage(m) }

// printJSON emits an indented JSON document (used by --json output, which
// prints the payload the reference prints: a list for vehicles, an object
// for status/battery/travel).
func printJSON(v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Println("null")
		return
	}
	fmt.Println(string(b))
}

// numField reads a numeric field that the gateways return either as a JSON
// number or as a numeric string.
func numField(m map[string]any, key string) float64 {
	switch t := m[key].(type) {
	case float64:
		return t
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	}
	return 0
}

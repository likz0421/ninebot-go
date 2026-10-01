// Package config resolves the config directory and loads/saves
// config.json, tokens.json and vehicle-cache.json.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Config is the persistent CLI configuration (config.json).
type Config struct {
	AreaCode  string `json:"area_code,omitempty"`
	DeviceID  string `json:"device_id,omitempty"`
	Lang      string `json:"lang,omitempty"`
	CurrentSN string `json:"current_sn,omitempty"`

	// Region, OSVersion and OSModel feed the business channel's
	// regionx / platform_ver fields; the reference client spoofs an
	// Android 13 Xiaomi handset ("platform_ver": "13 Xiaomi").
	Region    string `json:"region,omitempty"`
	OSVersion string `json:"os_version,omitempty"`
	OSModel   string `json:"os_model,omitempty"`
}

// PlatformVer is the platform_ver value sent to the business APIs.
func (c *Config) PlatformVer() string {
	if c.OSModel == "" {
		return c.OSVersion
	}
	if c.OSVersion == "" {
		return c.OSModel
	}
	return c.OSVersion + " " + c.OSModel
}

// Tokens is the persisted credential set (tokens.json).
type Tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
	ExpiresAt    int64  `json:"expires_at,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	UID          string `json:"user_id,omitempty"`

	// business_login result (used for ebike/travel/control APIs)
	BizUID    string `json:"biz_uid,omitempty"`
	BizToken  string `json:"biz_token,omitempty"`
	BizExpIn  int64  `json:"biz_expires_in,omitempty"`
	BizExpAt  int64  `json:"biz_expires_at,omitempty"`
	LoginType string `json:"login_type,omitempty"`
	Phone     string `json:"phone,omitempty"`
}

// CachedVehicle mirrors the vehicle cache used to avoid re-listing.
type CachedVehicle struct {
	SN      string `json:"sn"`
	Name    string `json:"name,omitempty"`
	Shared  bool   `json:"shared,omitempty"`
	Product string `json:"product,omitempty"`
}

// Dir is a resolved config directory.
type Dir string

// ResolveDir returns the config dir: --config flag, $NINEBOT_CONFIG_DIR,
// or ~/.config/ninebot.
func ResolveDir(flagVal string) Dir {
	if flagVal != "" {
		return Dir(flagVal)
	}
	if v := os.Getenv("NINEBOT_CONFIG_DIR"); v != "" {
		return Dir(v)
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return Dir(".ninebot")
	}
	return Dir(filepath.Join(home, ".config", "ninebot"))
}

func (d Dir) ensure() error {
	return os.MkdirAll(string(d), 0o755)
}

// Load reads config.json, generating defaults when absent.
func (d Dir) Load() (*Config, error) {
	if err := d.ensure(); err != nil {
		return nil, err
	}
	cfg := &Config{}
	path := filepath.Join(string(d), "config.json")
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		cfg = d.generate()
		_ = d.Save(cfg) // best-effort
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	d.applyDefaults(cfg)
	return cfg, nil
}

// Save writes config.json.
func (d Dir) Save(cfg *Config) error {
	if err := d.ensure(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(string(d), "config.json"), append(b, '\n'), 0o600)
}

func (d Dir) generate() *Config {
	return &Config{AreaCode: "86", Lang: "zh", DeviceID: randomDeviceID(),
		Region: "bj", OSVersion: "13", OSModel: "Xiaomi"}
}

func (d Dir) applyDefaults(cfg *Config) {
	if cfg.AreaCode == "" {
		cfg.AreaCode = "86"
	}
	if cfg.Lang == "" {
		cfg.Lang = "zh"
	}
	if cfg.Region == "" {
		cfg.Region = "bj"
	}
	if cfg.OSVersion == "" {
		cfg.OSVersion = "13"
	}
	if cfg.OSModel == "" {
		cfg.OSModel = "Xiaomi"
	}
	if cfg.DeviceID == "" {
		cfg.DeviceID = randomDeviceID()
	}
}

// randomDeviceID mimics an android device id (16 hex chars).
func randomDeviceID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// LoadTokens reads tokens.json. Missing file yields (nil, nil).
func (d Dir) LoadTokens() (*Tokens, error) {
	path := filepath.Join(string(d), "tokens.json")
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t := &Tokens{}
	if err := json.Unmarshal(b, t); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return t, nil
}

// SaveTokens writes tokens.json atomically.
func (d Dir) SaveTokens(t *Tokens) error {
	if err := d.ensure(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(string(d), "tokens.json.tmp")
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(string(d), "tokens.json"))
}

// DeleteTokens removes tokens.json (logout). Missing file is not an error.
func (d Dir) DeleteTokens() error {
	err := os.Remove(filepath.Join(string(d), "tokens.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// LoadVehicleCache reads vehicle-cache.json; missing file yields nil.
func (d Dir) LoadVehicleCache() ([]CachedVehicle, error) {
	path := filepath.Join(string(d), "vehicle-cache.json")
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var vs []CachedVehicle
	if err := json.Unmarshal(b, &vs); err != nil {
		return nil, err
	}
	return vs, nil
}

// SaveVehicleCache writes vehicle-cache.json.
func (d Dir) SaveVehicleCache(vs []CachedVehicle) error {
	if err := d.ensure(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(vs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(string(d), "vehicle-cache.json"), append(b, '\n'), 0o600)
}

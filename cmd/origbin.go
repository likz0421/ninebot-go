package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"ninecli/internal/config"
)

// origBinPath returns the path to the original Python ninecli binary.
func origBinPath() (string, error) {
	if p := os.Getenv("NINECLI_ORIG_BIN"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	// Known TeleAgent runtime path
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".local", "share", "TeleAgent", "runtimes", "python",
			"Lib", "site-packages", "ninecli", "bin", "ninecli.exe"),
	}
	if runtime.GOOS == "windows" {
		candidates = append(candidates,
			filepath.Join(os.Getenv("ProgramData"), "ninecli", "ninecli.exe"),
		)
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if p, err := exec.LookPath("ninecli"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("ninecli.exe"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("original ninecli binary not found (set NINECLI_ORIG_BIN to override)")
}

// origConfigDir returns the config directory the original binary uses.
// On Windows the Python appdirs library resolves to %APPDATA%\ninebot.
func origConfigDir() string {
	if v := os.Getenv("NINEBOT_CONFIG_DIR"); v != "" {
		return v
	}
	if runtime.GOOS == "windows" {
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "ninebot")
		}
	}
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		return filepath.Join(home, ".config", "ninebot")
	}
	return ".ninebot"
}

// origTokens mirrors the token JSON format used by the original Python ninecli.
type origTokens struct {
	UUID                string `json:"uuid"`
	Username            string `json:"username,omitempty"`
	Phone               string `json:"phone,omitempty"`
	Region              string `json:"region,omitempty"`
	AreaCode            string `json:"areaCode,omitempty"`
	AccessToken         string `json:"access_token"`
	RefreshToken        string `json:"refresh_token,omitempty"`
	AccessTokenValidity string `json:"accessTokenValidity,omitempty"`
	BusinessUID         string `json:"business_uid,omitempty"`
	SavedAt             int64  `json:"saved_at"`
}

// syncTokensToOrig writes our tokens in the original binary's format
// so that the original binary can use them for encrypted-channel operations.
// It preserves existing fields (like business_uid) that we may not have.
func syncTokensToOrig(tok *config.Tokens) error {
	dir := origConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// Read existing orig tokens to preserve fields we don't have
	var existing origTokens
	if b, err := os.ReadFile(filepath.Join(dir, "tokens.json")); err == nil {
		_ = json.Unmarshal(b, &existing)
	}
	ot := origTokens{
		UUID:         tok.UID,
		Username:     existing.Username,
		Phone:        tok.Phone,
		Region:       "bj",
		AreaCode:     "86",
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		BusinessUID:  tok.BizUID,
		SavedAt:      time.Now().Unix(),
	}
	// Preserve existing business_uid if we don't have one
	if ot.BusinessUID == "" {
		ot.BusinessUID = existing.BusinessUID
	}
	if ot.AccessTokenValidity == "" && existing.AccessTokenValidity != "" {
		ot.AccessTokenValidity = existing.AccessTokenValidity
	}
	if tok.ExpiresAt > 0 {
		ot.AccessTokenValidity = fmt.Sprintf("%d", tok.ExpiresAt*1000)
	}
	b, err := json.MarshalIndent(ot, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "tokens.json"), append(b, '\n'), 0o600)
}

// removeOrigTokens deletes the tokens file from the original binary's config dir.
func removeOrigTokens() {
	_ = os.Remove(filepath.Join(origConfigDir(), "tokens.json"))
}

// runOrig executes the original ninecli binary with --json and the
// original config directory, returning stdout.
func runOrig(args ...string) (string, error) {
	bin, err := origBinPath()
	if err != nil {
		return "", err
	}
	dir := origConfigDir()
	// Always pass explicit host flags so the original binary doesn't
	// need to resolve them from config (which can fail on some systems).
	full := append([]string{
		"--json",
		"--config", dir,
		"--biz-host", "https://api-jhcx-v6-bj.ninebot.com",
		"--ebike-host", "https://ebike.ninebot.com",
		"--motor-host", "https://steeldust.ninebot.com",
		"--travel-host", "https://cn-cbu-gateway.ninebot.com",
		"--passport-base", "https://api-passport-bj.ninebot.com",
	}, args...)
	cmd := exec.Command(bin, full...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = strings.TrimSpace(stdout.String())
		}
		return "", fmt.Errorf("%s: %s", err.Error(), errMsg)
	}
	return stdout.String(), nil
}

// origControlCmd maps our internal op names to the original binary's
// sub-command names.
var origControlCmd = map[string]string{
	"bell":         "bell",
	"engine_start": "engine-start",
	"engine_stop":  "engine-stop",
	"buck":         "buck",
}

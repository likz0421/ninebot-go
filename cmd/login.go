package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"ninecli/internal/api"
	"ninecli/internal/config"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Password login (Passport) — saves tokens.json",
	RunE: func(c *cobra.Command, args []string) error {
		return runLogin(c, args)
	},
}

var loginFlags struct {
	user     string
	password string
	area     string
}

func init() {
	loginCmd.Flags().StringVarP(&loginFlags.user, "user", "u", "", "Passport username (phone or nickname)")
	loginCmd.Flags().StringVarP(&loginFlags.password, "password", "p", "", "Passport password")
	loginCmd.Flags().StringVarP(&loginFlags.area, "area", "a", "", "phone country code (default from config: area_code)")
	_ = loginCmd.MarkFlagRequired("user")
	_ = loginCmd.MarkFlagRequired("password")
	rootCmd.AddCommand(loginCmd)
}

func runLogin(c *cobra.Command, args []string) error {
	d := dirOrDefaults()
	cfg, err := d.Load()
	if err != nil {
		return err
	}
	area := loginFlags.area
	if area == "" {
		area = cfg.AreaCode
	}
	client := api.New(hostsOrDefaults(), cfg, nil)
	m, err := client.Login(area, loginFlags.user, loginFlags.password)
	if err != nil {
		return err
	}
	if !api.IsSuccess(m) {
		return fmt.Errorf("login failed: %s", bizMsg(m))
	}
	tok, _ := d.LoadTokens()
	if tok == nil {
		tok = &apiTokensZero
	}
	tok.LoginType = "password"
	tok.Phone = loginFlags.user
	if err := finishLogin(&d, tok, m); err != nil {
		return err
	}
	// finishLogin refreshed tok in place; make the client use it so the
	// follow-up BusinessLogin sees the fresh access_token (the client was
	// created with nil tokens before login).
	client.Tokens = tok
	if err := businessAfterLogin(client, &d, tok); err != nil {
		return err
	}
	if JSONOutput(m) {
		return nil
	}
	fmt.Println("login ok — tokens.json saved")
	return nil
}

func businessAfterLogin(client *api.Client, d *config.Dir, tok *apiTokens) error {
	m, err := client.BusinessLogin()
	if err != nil {
		return err
	}
	if api.IsBizSuccess(m) {
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
	return fmt.Errorf("business_login failed: %s", bizMsg(m))
}

func marshalIndent(v any) ([]byte, error) { return json.MarshalIndent(v, "", "  ") }

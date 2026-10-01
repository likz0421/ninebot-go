package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"ninecli/internal/api"
	"ninecli/internal/config"
)

type apiTokens = config.Tokens

var apiTokensZero config.Tokens

var loginCodeCmd = &cobra.Command{
	Use:   "login-code",
	Short: "SMS-code login (Passport) — sends code, then consumes it",
	Long: "Two-step SMS login. First run `ninebot login-code -m PHONE` to send the verification code to your phone, " +
		"then re-run with `-c CODE` to complete login. Saves tokens.json and auto-calls business_login on successful code consumption.",
	RunE: func(c *cobra.Command, args []string) error {
		return runLoginCode(c, args)
	},
}

var loginCodeFlags struct {
	phone string
	code  string
	area  string
}

func init() {
	loginCodeCmd.Flags().StringVarP(&loginCodeFlags.phone, "phone", "m", "", "phone number (without country code)")
	loginCodeCmd.Flags().StringVarP(&loginCodeFlags.code, "code", "c", "", "SMS verification code (omit to send code first)")
	loginCodeCmd.Flags().StringVarP(&loginCodeFlags.area, "area", "a", "", "phone country code (default from config: area_code)")
	_ = loginCodeCmd.MarkFlagRequired("phone")
	rootCmd.AddCommand(loginCodeCmd)
}

func runLoginCode(c *cobra.Command, args []string) error {
	d := dirOrDefaults()
	cfg, err := d.Load()
	if err != nil {
		return err
	}
	area := loginCodeFlags.area
	if area == "" {
		area = cfg.AreaCode
	}
	client := api.New(hostsOrDefaults(), cfg, nil)

	if loginCodeFlags.code == "" {
		m, err := client.SendCode(area, loginCodeFlags.phone)
		if err != nil {
			return err
		}
		if !api.IsSuccess(m) {
			return fmt.Errorf("send-code failed: %s", bizMsg(m))
		}
		if JSONOutput(m) {
			return nil
		}
		fmt.Println("code sent — re-run with -c <code> to finish login")
		return nil
	}

	m, err := client.PhoneCodeLogin(area, loginCodeFlags.phone, loginCodeFlags.code, "bj")
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
	tok.LoginType = "phoneCode"
	tok.Phone = loginCodeFlags.phone
	if err := finishLogin(&d, tok, m); err != nil {
		return err
	}
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

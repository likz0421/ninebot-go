package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"ninecli/internal/api"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Verify the saved token (calls POST /v5/user)",
	RunE: func(c *cobra.Command, args []string) error {
		cl, d, err := newBusinessForCmd()
		if err != nil {
			return err
		}
		tok, _ := d.LoadTokens()
		if tok == nil {
			return fmt.Errorf("not logged in")
		}
		m, err := cl.V5User()
		if err != nil {
			return err
		}
		if !api.IsSuccess(m) {
			return fmt.Errorf("whoami failed: %s", bizMsg(m))
		}
		if JSONOutput(m) {
			return nil
		}
		data, _ := m["data"].(map[string]any)
		if data == nil {
			data = m
		}
		fmt.Printf("uuid=%s username=%s phone=%s\n",
			firstNonEmpty(strField(data, "uuid"), tok.UID),
			strField(data, "username"), strField(data, "phone"))
		return nil
	},
}

var refreshCmd = &cobra.Command{
	Use:    "refresh",
	Short:  "Rotate the access token via refresh_token",
	Hidden: true,
	RunE: func(c *cobra.Command, args []string) error {
		cl, d, err := newBusinessForCmd()
		if err != nil {
			return err
		}
		tok, _ := d.LoadTokens()
		if tok == nil {
			return fmt.Errorf("not logged in")
		}
		m, err := cl.Refresh()
		if err != nil {
			return err
		}
		if !api.IsSuccess(m) {
			return fmt.Errorf("refresh failed: %s", bizMsg(m))
		}
		if err := finishLogin(d, tok, m); err != nil {
			return err
		}
		if JSONOutput(m) {
			return nil
		}
		fmt.Println("token refreshed — tokens.json updated")
		return nil
	},
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
	rootCmd.AddCommand(refreshCmd)
}

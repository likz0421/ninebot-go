// Package cmd wires up every ninecli subcommand via cobra.
package cmd

import (
	"github.com/spf13/cobra"
)

var (
	flagConfig       string
	flagJSON         bool
	flagYes          bool
	flagPassportBase string
	flagBizHost      string
	flagEbikeHost    string
	flagMotorHost    string
	flagTravelHost   string
)

var rootCmd = &cobra.Command{
	Use:   "ninecli",
	Short: "ninecli — Ninebot scooter Passport + vehicle info + control",
	Long:  "ninecli — Ninebot scooter Passport + vehicle info + control",
}

// Execute runs the root command.
func Execute() {
	_ = rootCmd.Execute()
}

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&flagPassportBase, "passport-base", "", "override Passport base URL (testing/development; default https://api-passport-bj.ninebot.com)")
	pf.StringVar(&flagBizHost, "biz-host", "", "override business_login host URL (testing/development; default https://api-jhcx-v6-bj.ninebot.com)")
	pf.StringVar(&flagEbikeHost, "ebike-host", "", "override ebike host URL (testing/development; default https://ebike.ninebot.com)")
	pf.StringVar(&flagMotorHost, "motor-host", "", "override motor host URL (testing/development; default https://steeldust.ninebot.com)")
	pf.StringVar(&flagTravelHost, "travel-host", "", "override travel host URL (testing/development; default https://cn-cbu-gateway.ninebot.com)")
	pf.StringVar(&flagConfig, "config", "", "config directory (default: $NINEBOT_CONFIG_DIR or ~/.config/ninebot)")
	pf.BoolVar(&flagJSON, "json", false, "emit raw decrypted JSON instead of human-readable output")
	pf.BoolVarP(&flagYes, "yes", "y", false, "bypass the y/N safety prompt on control commands (engine-start/engine-stop/buck)")
}

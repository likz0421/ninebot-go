package cmd

import (
	"github.com/spf13/cobra"
)

var bellCmd = &cobra.Command{
	Use:   "bell SN",
	Short: "Ring the find-my-vehicle bell",
	Long: "Ring the find-my-vehicle bell. Posts to POST /devices/control/bell on ebike.ninebot.com with the cmd 'operation' field = 'bell'. " +
		"Audible ring — non-destructive, no y/N prompt.",
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		return runControl("bell", args)
	},
}

var engineStartCmd = &cobra.Command{
	Use:   "engine-start SN",
	Short: "⚠️  Power on / unlock the vehicle",
	Long: "ACTUATES the physical vehicle. Posts to POST /devices/control/engine_start on ebike.ninebot.com with the cmd 'operation' field = 'engine_start'. " +
		"Use only with the owner's consent while physically present. Server rejects malformed or stale cmds (no vehicle action on rejection).",
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		return runControl("engine_start", args)
	},
}

var engineStopCmd = &cobra.Command{
	Use:   "engine-stop SN",
	Short: "⚠️  Power off / lock the vehicle",
	Long: "ACTUATES the physical vehicle. Posts to POST /devices/control/engine_stop on ebike.ninebot.com with the cmd 'operation' field = 'engine_stop'. " +
		"Use only with the owner's consent while physically present.",
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		return runControl("engine_stop", args)
	},
}

var buckCmd = &cobra.Command{
	Use:   "buck SN",
	Short: "⚠️  Open the seat trunk",
	Long: "ACTUATES the physical vehicle. Posts to POST /devices/control/open_buck (cmd 'operation' field = 'buck' — the URL uses the long form, " +
		"the op field uses the short form). Use only with the owner's consent while physically present.",
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		return runControl("buck", args)
	},
}

func init() {
	rootCmd.AddCommand(bellCmd)
	rootCmd.AddCommand(engineStartCmd)
	rootCmd.AddCommand(engineStopCmd)
	rootCmd.AddCommand(buckCmd)
}

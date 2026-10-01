package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"ninecli/internal/api"
)

var batteryCmd = &cobra.Command{
	Use:   "battery SN",
	Short: "Show battery info (voltage, temperature, cycles, charge power)",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, d, err := newBusinessForCmd()
		if err != nil {
			return err
		}
		tok, _ := d.LoadTokens()
		if tok == nil {
			return fmt.Errorf("not logged in")
		}
		if err := ensureBusinessUID(cl, d, tok); err != nil {
			return err
		}
		m, err := cl.BatteryInfo(args[0])
		if err != nil {
			return err
		}
		if !api.IsBizSuccess(m) {
			return fmt.Errorf("battery failed: %s", bizMsg(m))
		}
		data := firstObject(m["data"])
		if flagJSON {
			printJSON(data)
			return nil
		}
		// The battery payload carries no product name; the reference
		// resolves it from the vehicle cache.
		name := ""
		if cv, err := cachedVehicleBySN(d, args[0]); err == nil && cv != nil {
			name = cv.Product
		}
		printBattery(args[0], name, data)
		return nil
	},
}

// printBattery renders the battery payload in the reference layout.
func printBattery(sn, name string, data map[string]any) {
	kind := "铅酸"
	if strField(data, "battery_type") == "1" {
		kind = "锂电"
	}
	cells, _ := data["battery_list"].([]any)
	var cell map[string]any
	if len(cells) > 0 {
		cell, _ = cells[0].(map[string]any)
	}
	if cell == nil {
		cell = map[string]any{}
	}
	fmt.Printf("%s «%s»\n", sn, name)
	fmt.Printf("  type : %s\n", kind)
	fmt.Printf("  电量  : %v%%\n", data["electricity"])
	fmt.Printf("  电压  : %sV\n", strField(cell, "bms_volt"))
	fmt.Printf("  温度  : %s°C\n", strField(cell, "bat_temp"))
	fmt.Printf("  健康  : %v\n", cell["score"])
}

func init() {
	rootCmd.AddCommand(batteryCmd)
}

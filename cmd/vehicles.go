package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"ninecli/internal/api"
	"ninecli/internal/config"
)

var vehiclesCmd = &cobra.Command{
	Use:   "vehicles",
	Short: "List owned + shared vehicles",
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
		m, err := cl.MyVehicle()
		if err != nil {
			return err
		}
		if !api.IsBizSuccess(m) {
			return fmt.Errorf("vehicles failed: %s", bizMsg(m))
		}
		vs := vehicleRows(m)
		_ = d.SaveVehicleCache(vs)
		if flagJSON {
			printJSON(listPayload(m))
			return nil
		}
		for _, v := range vs {
			perm := "owned"
			if v.Shared {
				perm = "shared"
			}
			fmt.Printf("- %s  %s  «%s»  [%s]\n", v.SN, v.Product, v.Name, perm)
		}
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status SN",
	Short: "Show vehicle status (location, battery, lock, acc, perms)",
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
		m, err := cl.DesktopComponent(args[0])
		if err != nil {
			return err
		}
		if !api.IsBizSuccess(m) {
			return fmt.Errorf("status failed: %s", bizMsg(m))
		}
		obj := firstObject(m["data"])
		if flagJSON {
			printJSON(obj)
			return nil
		}
		fmt.Printf("%s «%s»\n", args[0], strField(obj, "ble_name"))
		if loc, ok := obj["loc"].(map[string]any); ok {
			fmt.Printf("  location: lat=%v lon=%v  (lock=%v)\n", loc["lat"], loc["lon"], loc["lock"])
		}
		fmt.Printf("  battery : %v%%   range=%vm   charging=%v\n",
			strField(obj, "dump_energy"), numField(obj, "range"), obj["charging"])
		if pwr, ok := obj["pwr"]; ok {
			fmt.Printf("  pwr=%v\n", pwr)
		}
		return nil
	},
}

// listPayload returns the vehicle array of a business envelope.
func listPayload(m map[string]any) any {
	if l, ok := m["data"].([]any); ok {
		return l
	}
	return []any{}
}

// firstObject returns the leading object of a `data` value that is either an
// array (status) or an object (battery, travel).
func firstObject(v any) map[string]any {
	switch t := v.(type) {
	case map[string]any:
		return t
	case []any:
		for _, e := range t {
			if m, ok := e.(map[string]any); ok {
				return m
			}
		}
	}
	return map[string]any{}
}

// vehicleRows maps a my-vehicle envelope onto the local cache shape. The
// gateways key the serial as `wnumber` and the nickname as `ble_name`.
func vehicleRows(m map[string]any) []config.CachedVehicle {
	var out []config.CachedVehicle
	seen := map[string]bool{}
	items, _ := m["data"].([]any)
	for _, it := range items {
		vm, ok := it.(map[string]any)
		if !ok {
			continue
		}
		sn := firstNonEmpty(strField(vm, "wnumber"), strField(vm, "sn"), strField(vm, "sn9bot"),
			strField(vm, "serial_num"))
		if sn == "" || seen[sn] {
			continue
		}
		seen[sn] = true
		out = append(out, config.CachedVehicle{
			SN: sn,
			Name: firstNonEmpty(strField(vm, "ble_name"), strField(vm, "device_name"),
				strField(vm, "name"), strField(vm, "alias")),
			Product: firstNonEmpty(strField(vm, "vehicle_name"), strField(vm, "product"),
				strField(vm, "product_name")),
			Shared: numField(vm, "is_common_user") == 1,
		})
	}
	return out
}

func init() {
	rootCmd.AddCommand(vehiclesCmd)
	rootCmd.AddCommand(statusCmd)
}

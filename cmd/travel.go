package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"ninecli/internal/api"
)

var travelCmd = &cobra.Command{
	Use:   "travel SN",
	Short: "Ride history (default: list current month); --detail <id> shows one ride",
	Long:  "Ride history (default: list current month); --detail <id> shows one ride",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		return runTravel(c, args)
	},
}

var travelFlags struct {
	detail travelDetailValue
	month  string
}

// travelDetailValue renders in help as "ninebot travel SN", matching
// the reference CLI's custom pflag.Value type.
type travelDetailValue string

func (v *travelDetailValue) String() string     { return string(*v) }
func (v *travelDetailValue) Set(s string) error { *v = travelDetailValue(s); return nil }
func (v *travelDetailValue) Type() string       { return "ninebot travel SN" }

func init() {
	travelCmd.Flags().Var(&travelFlags.detail, "detail", "show one ride by travel_id (use ninebot travel SN to find the id)")
	travelCmd.Flags().StringVar(&travelFlags.month, "month", "", "month in YYYYMM (default: current month)")
	rootCmd.AddCommand(travelCmd)
}

func runTravel(c *cobra.Command, args []string) error {
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
	sn := args[0]

	if travelFlags.detail != "" {
		m, err := cl.TravelDetail(sn, string(travelFlags.detail))
		if err != nil {
			return err
		}
		if !api.IsBizSuccess(m) {
			if travelDetailHasNoData(m) {
				fmt.Println("no detail data for this ride")
				return nil
			}
			return fmt.Errorf("travel detail failed: %s", bizMsg(m))
		}
		if flagJSON {
			printJSON(firstObject(m["data"]))
			return nil
		}
		printTravelDetail(sn, string(travelFlags.detail), m)
		return nil
	}

	month := travelFlags.month
	if month == "" {
		month = api.CurrentMonthYYYYMM()
	}
	m, err := cl.TravelList(sn, month)
	if err != nil {
		return err
	}
	if !api.IsBizSuccess(m) {
		return fmt.Errorf("travel failed: %s", bizMsg(m))
	}
	if flagJSON {
		printJSON(m["data"])
		return nil
	}
	printTravelGroups(sn, m)
	return nil
}

func travelDetailHasNoData(m map[string]any) bool {
	if data, ok := m["data"].(map[string]any); ok {
		if len(data) == 0 {
			return true
		}
	}
	return false
}

// printTravelGroups renders the month summary in the reference layout:
// a totals line, then one block per day with its rides.
func printTravelGroups(sn string, m map[string]any) {
	data, _ := m["data"].(map[string]any)
	if data == nil {
		fmt.Println("no rides this month")
		return
	}
	fmt.Printf("%s %v: %v rides  %vs  %v%% used\n", sn, strField(data, "month"),
		data["times"], data["duration"], data["ec"])

	type day struct {
		label string
		rides []map[string]any
		km    float64
		secs  float64
		max   float64
		used  float64
	}
	var days []*day
	index := map[string]*day{}
	items, _ := data["list"].([]any)
	for _, it := range items {
		rm, ok := it.(map[string]any)
		if !ok {
			continue
		}
		st := rideTime(rm)
		label := st.Format("01/02")
		d, seen := index[label]
		if !seen {
			d = &day{label: label}
			index[label] = d
			days = append(days, d)
		}
		d.rides = append(d.rides, rm)
		d.km += numField(rm, "mileages")
		d.secs += numField(rm, "duration")
		d.used += numField(rm, "used_electricity")
		if s := numField(rm, "speed"); s > d.max {
			d.max = s
		}
	}
	for _, d := range days {
		fmt.Printf("\n%s  %d rides  %.1fkm  %.0fs  max %.1fkm/h  %.0f%% used\n",
			d.label, len(d.rides), d.km, d.secs, d.max, d.used)
		for _, rm := range d.rides {
			fmt.Printf("  %s  %.2fkm  %.0fs  %.2fkm/h  %.1f%%  id=%s\n",
				rideTime(rm).Format("15:04"), numField(rm, "mileages"), numField(rm, "duration"),
				numField(rm, "speed"), numField(rm, "used_electricity"), strField(rm, "travel_id"))
		}
	}
}

// rideTime is when a ride is reported: the end stamp, falling back to
// the start when the gateway omits it.
func rideTime(ride map[string]any) time.Time {
	if t := numField(ride, "end_time"); t > 0 {
		return time.Unix(int64(t), 0).Local()
	}
	return time.Unix(int64(numField(ride, "start_time")), 0).Local()
}

// printTravelDetail renders one ride in the reference layout: the two
// timestamps, the distance/speed/energy summary, the thumbnail and the
// extent of the GPS trail.
func printTravelDetail(sn, travelID string, m map[string]any) {
	data, _ := m["data"].(map[string]any)
	if data == nil {
		fmt.Println("no detail data for this ride")
		return
	}
	fmt.Printf("%s %s\n", sn, travelID)
	fmt.Printf("  start    %s\n", stamp(data, "start_time"))
	fmt.Printf("  end      %s  (%.0fs)\n", stamp(data, "end_time"), numField(data, "duration"))
	fmt.Printf("  distance %vkm   max %vkm/h   energy %vWh   battery used %.1f%%\n",
		fieldText(data, "mileages"), fieldText(data, "speed"), fieldText(data, "ec"),
		numField(data, "used_electricity"))
	if img := strField(data, "img"); img != "" {
		fmt.Printf("  img      %s\n", img)
	}
	if trail := strField(data, "trail"); trail != "" {
		pts := strings.Split(trail, ";")
		fmt.Printf("  trail    %d points (lon,lat,speed,distFromPrev)\n", len(pts))
		fmt.Printf("           first %s\n", pts[0])
		fmt.Printf("           last  %s\n", pts[len(pts)-1])
	}
}

// fieldText renders a numeric-or-string JSON field without trailing noise.
func fieldText(m map[string]any, key string) string {
	switch t := m[key].(type) {
	case nil:
		return "0"
	case string:
		return t
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", t), "0"), ".")
	}
	return fmt.Sprint(m[key])
}

// stamp formats a unix field the way the reference does.
func stamp(m map[string]any, key string) string {
	return time.Unix(int64(numField(m, key)), 0).Local().Format("2006-01-02 15:04:05 MST")
}

// runControl is shared by bell/engine-start/engine-stop/buck.
func runControl(op string, args []string) error {
	info, ok := api.LookupControlOp(op)
	if !ok {
		return fmt.Errorf("unknown control op %q", op)
	}
	if info.Safety == "prompt" && !flagYes {
		fmt.Printf("%s\nProceed? (y/N): ", info.Warning)
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(strings.ToLower(line))
		if line != "y" && line != "yes" {
			fmt.Println("aborted")
			return nil
		}
	}
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
	m, err := cl.Control(op, args[0])
	if err != nil {
		return err
	}
	if !api.IsBizSuccess(m) {
		return fmt.Errorf("%s failed: %s", op, bizMsg(m))
	}
	if JSONOutput(m) {
		return nil
	}
	fmt.Printf("%s: ok\n", op)
	return nil
}

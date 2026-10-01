package api

import (
	"fmt"
	"time"

	"ninecli/internal/crypto"
)

// The travel gateway is the one business host that wants extra headers and
// a wider parameter set; both sets are verified against the reference
// client's traffic.
func (c *Client) travelHeaders() map[string]string {
	return map[string]string{
		"Accept":       "application/json",
		"access_token": c.Tokens.AccessToken,
		"rn-ver":       rnVersion,
		"rn-module":    "Track",
		"rn-version":   "0",
	}
}

// rnVersion is the tracking-library version the reference client reports.
const rnVersion = "743"

// TravelList fetches the ride history for one month.
func (c *Client) TravelList(sn, yyyymm string) (map[string]any, error) {
	extras := []crypto.KV{
		{Key: "wnumber", Value: sn},
		{Key: "rnVersion", Value: rnVersion},
		{Key: "vehicle_type", Value: vehicleTypeTravel},
		{Key: "month", Value: yyyymm},
		{Key: "page", Value: "1"},
		{Key: "current_version", Value: appVersion},
	}
	m, _, err := c.EncryptedPost(c.Hosts.TravelHost+"/app-api/travel/v6/travel-list2", extras, c.travelHeaders())
	return m, err
}

// TravelDetail fetches one ride's point stream.
func (c *Client) TravelDetail(sn, travelID string) (map[string]any, error) {
	extras := []crypto.KV{
		{Key: "wnumber", Value: sn},
		{Key: "rnVersion", Value: rnVersion},
		{Key: "vehicle_type", Value: vehicleTypeTravel},
		{Key: "travel_id", Value: travelID},
		{Key: "current_version", Value: appVersion},
	}
	m, _, err := c.EncryptedPost(c.Hosts.TravelHost+"/app-api/travel/v6/travel-info", extras, c.travelHeaders())
	return m, err
}

// CurrentMonthYYYYMM is the helper used by `travel` with no --month.
func CurrentMonthYYYYMM() string {
	return fmt.Sprintf("%04d%02d", time.Now().Year(), int(time.Now().Month()))
}

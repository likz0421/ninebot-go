package api

import (
	"ninecli/internal/crypto"
)

// vehicleType codes the reference client sends per gateway on the business
// channel (ebike 65, motor 64, travel gateway 112).
const (
	vehicleTypeEbike  = "65"
	vehicleTypeMotor  = "64"
	vehicleTypeTravel = "112"
)

// MyVehicle lists the vehicles this account can use. The reference client
// asks the ebike gateway and then the motor gateway with the same path and
// merges the two lists — either gateway may come back empty.
func (c *Client) MyVehicle() (map[string]any, error) {
	calls := []struct {
		url         string
		vehicleType string
	}{
		{c.Hosts.EbikeHost + "/vehicle/binding/my-vehicle", vehicleTypeEbike},
		{c.Hosts.MotorHost + "/vehicle/binding/my-vehicle", vehicleTypeMotor},
	}
	var merged []any
	var last map[string]any
	var ok bool
	for _, call := range calls {
		m, _, err := c.EncryptedPost(call.url,
			[]crypto.KV{{Key: "vehicle_type", Value: call.vehicleType}}, nil)
		if err != nil {
			return nil, err
		}
		last = m
		if !IsBizSuccess(m) {
			continue
		}
		ok = true
		list, _ := m["data"].([]any)
		for _, it := range list {
			if !containsVehicle(merged, it) {
				merged = append(merged, it)
			}
		}
	}
	if !ok {
		return last, nil
	}
	return map[string]any{"code": float64(1), "desc": "成功", "data": merged}, nil
}

// containsVehicle reports whether a merged list already carries this serial.
func containsVehicle(list []any, item any) bool {
	m, ok := item.(map[string]any)
	if !ok {
		return false
	}
	sn := strField(m, "wnumber")
	if sn == "" {
		sn = strField(m, "sn")
	}
	if sn == "" {
		return false
	}
	for _, e := range list {
		if em, ok := e.(map[string]any); ok {
			if strField(em, "wnumber") == sn || strField(em, "sn") == sn {
				return true
			}
		}
	}
	return false
}

// DesktopComponent returns the vehicle status payload (position, lock,
// battery, perms). Endpoint and field name verified against the reference
// client: the serial rides in `sn_str`, not `sn`.
func (c *Client) DesktopComponent(sn string) (map[string]any, error) {
	m, status, err := c.EncryptedPost(c.Hosts.EbikeHost+"/vehicle/vehicle/desktop-component",
		[]crypto.KV{{Key: "sn_str", Value: sn}}, nil)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, &Error{Status: status, Message: strField(m, "message")}
	}
	return m, nil
}

// BatteryInfo returns the battery payload; the serial rides in `wnumber`.
func (c *Client) BatteryInfo(sn string) (map[string]any, error) {
	m, status, err := c.EncryptedPost(c.Hosts.MotorHost+"/v6/vehicle/battery-info",
		[]crypto.KV{{Key: "wnumber", Value: sn}}, nil)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, &Error{Status: status, Message: strField(m, "message")}
	}
	return m, nil
}

// Control posts a vehicle control command over the encrypted channel.
//
// UNVERIFIED against live traffic on purpose: these operations actuate the
// physical vehicle. The endpoints and field names come from the reference
// CLI's own help text; the command is signed with a separate "control" RSA
// key that only BuildControlCmd knows about.
func (c *Client) Control(op, sn string) (map[string]any, error) {
	extras := []crypto.KV{
		{Key: "operation", Value: op},
		{Key: "sn", Value: sn},
		{Key: "nonce", Value: crypto.GenNonce(16)},
	}
	m, status, err := c.EncryptedPost(c.Hosts.EbikeHost+crypto.ControlURL(op), extras, nil)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, &Error{Status: status, Message: strField(m, "message")}
	}
	return m, nil
}

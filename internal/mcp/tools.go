package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"ninecli/internal/api"
)

type emptyInput struct{}

type authLoginInput struct {
	Account  string `json:"account" jsonschema:"phone number or nickname"`
	Password string `json:"password" jsonschema:"account password"`
}

type authSendCodeInput struct {
	Account string `json:"account" jsonschema:"phone number to send the SMS code to"`
}

type authConsumeCodeInput struct {
	Account string `json:"account" jsonschema:"phone number"`
	Code    string `json:"code" jsonschema:"SMS verification code"`
}

type snInput struct {
	SN string `json:"sn" jsonschema:"vehicle serial number"`
}

type travelInput struct {
	SN    string `json:"sn" jsonschema:"vehicle serial number"`
	Month string `json:"month,omitempty" jsonschema:"month in YYYYMM; defaults to current month"`
}

type travelDetailInput struct {
	SN       string `json:"sn" jsonschema:"vehicle serial number"`
	TravelID string `json:"travel_id" jsonschema:"travel id as returned by the travel tool"`
}

func mustJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

func unwrap(m map[string]any, err error) (*mcp.CallToolResult, any, error) {
	if err != nil {
		return nil, nil, err
	}
	return okResult(mustJSON(m)), nil, nil
}

func registerTools(server *mcp.Server, rt *Runtime) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "auth_login",
		Description: "Password login (Passport). Saves tokens.json.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in authLoginInput) (*mcp.CallToolResult, any, error) {
		cfg, err := rt.Dir.Load()
		if err != nil {
			return nil, nil, err
		}
		c := api.New(rt.Hosts, cfg, nil)
		m, err := c.Login(cfg.AreaCode, in.Account, in.Password)
		if err != nil {
			return nil, nil, err
		}
		if !api.IsSuccess(m) {
			return errResult("login failed: " + mstr(m, "msg")), nil, nil
		}
		tok, _ := rt.Dir.LoadTokens()
		if tok == nil {
			tok = &tokenZero
		}
		if err := applyTokens(rt.Dir, tok, m); err != nil {
			return nil, nil, err
		}
		rt.Invalidate()
		return unwrap(map[string]any{"status": "logged in"}, nil)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "auth_send_code",
		Description: "Send an SMS login code to a phone number.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in authSendCodeInput) (*mcp.CallToolResult, any, error) {
		cfg, err := rt.Dir.Load()
		if err != nil {
			return nil, nil, err
		}
		c := api.New(rt.Hosts, cfg, nil)
		m, err := c.SendCode(cfg.AreaCode, in.Account)
		if err != nil {
			return nil, nil, err
		}
		if !api.IsSuccess(m) {
			return errResult("send-code failed: " + mstr(m, "msg")), nil, nil
		}
		return unwrap(map[string]any{"status": "code sent"}, nil)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "auth_consume_code",
		Description: "Consume an SMS login code and finish the login.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in authConsumeCodeInput) (*mcp.CallToolResult, any, error) {
		cfg, err := rt.Dir.Load()
		if err != nil {
			return nil, nil, err
		}
		c := api.New(rt.Hosts, cfg, nil)
		m, err := c.PhoneCodeLogin(cfg.AreaCode, in.Account, in.Code, "bj")
		if err != nil {
			return nil, nil, err
		}
		if !api.IsSuccess(m) {
			return errResult("login failed: " + mstr(m, "msg")), nil, nil
		}
		tok, _ := rt.Dir.LoadTokens()
		if tok == nil {
			tok = &tokenZero
		}
		if err := applyTokens(rt.Dir, tok, m); err != nil {
			return nil, nil, err
		}
		rt.Invalidate()
		return unwrap(map[string]any{"status": "logged in"}, nil)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "auth_refresh",
		Description: "Rotate the access token via refresh_token.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in emptyInput) (*mcp.CallToolResult, any, error) {
		c, tok, err := rt.Client()
		if err != nil {
			return nil, nil, err
		}
		m, err := c.Refresh()
		if err != nil {
			return nil, nil, err
		}
		if !api.IsSuccess(m) {
			return errResult("refresh failed: " + mstr(m, "msg")), nil, nil
		}
		if err := applyTokens(rt.Dir, tok, m); err != nil {
			return nil, nil, err
		}
		rt.Invalidate()
		return unwrap(map[string]any{"status": "refreshed"}, nil)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "whoami",
		Description: "Verify the saved token (POST /v5/user).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in emptyInput) (*mcp.CallToolResult, any, error) {
		c, _, err := rt.Client()
		if err != nil {
			return nil, nil, err
		}
		m, err := c.V5User()
		if err != nil {
			return nil, nil, err
		}
		return unwrap(m, nil)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vehicles",
		Description: "List owned + shared vehicles.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in emptyInput) (*mcp.CallToolResult, any, error) {
		c, _, err := rt.Client()
		if err != nil {
			return nil, nil, err
		}
		m, err := c.MyVehicle()
		if err != nil {
			return nil, nil, err
		}
		return unwrap(m, nil)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vehicle_status",
		Description: "Show vehicle status (location, battery, lock, acc, perms).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in snInput) (*mcp.CallToolResult, any, error) {
		c, _, err := rt.Client()
		if err != nil {
			return nil, nil, err
		}
		m, err := c.DesktopComponent(in.SN)
		if err != nil {
			return nil, nil, err
		}
		return unwrap(m, nil)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vehicle_battery",
		Description: "Show battery info (voltage, temperature, cycles, charge power).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in snInput) (*mcp.CallToolResult, any, error) {
		c, _, err := rt.Client()
		if err != nil {
			return nil, nil, err
		}
		m, err := c.BatteryInfo(in.SN)
		if err != nil {
			return nil, nil, err
		}
		return unwrap(m, nil)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "travel",
		Description: "Ride history list for one month (default current).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in travelInput) (*mcp.CallToolResult, any, error) {
		c, _, err := rt.Client()
		if err != nil {
			return nil, nil, err
		}
		month := in.Month
		if month == "" {
			month = api.CurrentMonthYYYYMM()
		}
		m, err := c.TravelList(in.SN, month)
		if err != nil {
			return nil, nil, err
		}
		return unwrap(m, nil)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "travel_detail",
		Description: "Show one ride's detail stream by travel_id.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in travelDetailInput) (*mcp.CallToolResult, any, error) {
		c, _, err := rt.Client()
		if err != nil {
			return nil, nil, err
		}
		m, err := c.TravelDetail(in.SN, in.TravelID)
		if err != nil {
			return nil, nil, err
		}
		return unwrap(m, nil)
	})

	confirm := " ACTUATES the physical vehicle; requires user confirmation before calling."
	mcp.AddTool(server, &mcp.Tool{
		Name:        "engine_start",
		Description: "Power on / unlock the vehicle." + confirm,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in snInput) (*mcp.CallToolResult, any, error) {
		c, _, err := rt.Client()
		if err != nil {
			return nil, nil, err
		}
		m, err := c.Control("engine_start", in.SN)
		if err != nil {
			return nil, nil, err
		}
		return unwrap(m, nil)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "engine_stop",
		Description: "Power off / lock the vehicle." + confirm,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in snInput) (*mcp.CallToolResult, any, error) {
		c, _, err := rt.Client()
		if err != nil {
			return nil, nil, err
		}
		m, err := c.Control("engine_stop", in.SN)
		if err != nil {
			return nil, nil, err
		}
		return unwrap(m, nil)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "buck",
		Description: "Open the seat trunk." + confirm,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in snInput) (*mcp.CallToolResult, any, error) {
		c, _, err := rt.Client()
		if err != nil {
			return nil, nil, err
		}
		m, err := c.Control("buck", in.SN)
		if err != nil {
			return nil, nil, err
		}
		return unwrap(m, nil)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "bell",
		Description: "Ring the find-my-vehicle bell (non-destructive).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in snInput) (*mcp.CallToolResult, any, error) {
		c, _, err := rt.Client()
		if err != nil {
			return nil, nil, err
		}
		m, err := c.Control("bell", in.SN)
		if err != nil {
			return nil, nil, err
		}
		return unwrap(m, nil)
	})
}

func mstr(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

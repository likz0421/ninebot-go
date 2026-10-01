package cmd

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"ninecli/internal/api"
	"ninecli/internal/config"
)

//go:embed webui.html
var webuiFS embed.FS

var flagWebBind string

var webCmd = &cobra.Command{
	Use:    "web",
	Short:  "Launch the local web UI (http://127.0.0.1:8080)",
	Hidden: true,
	RunE: func(c *cobra.Command, args []string) error {
		mux := http.NewServeMux()
		mux.HandleFunc("/", handleWebIndex)
		mux.HandleFunc("/api/state", handleWebState)
		mux.HandleFunc("/api/login", handleWebLogin)
		mux.HandleFunc("/api/send-code", handleWebSendCode)
		mux.HandleFunc("/api/login-code", handleWebLoginCode)
		mux.HandleFunc("/api/logout", handleWebLogout)
		mux.HandleFunc("/api/vehicles", handleWebVehicles)
		mux.HandleFunc("/api/status", handleWebStatus)
		mux.HandleFunc("/api/battery", handleWebBattery)
		mux.HandleFunc("/api/travel", handleWebTravel)
		mux.HandleFunc("/api/control", handleWebControl)

		addr := flagWebBind
		if addr == "" {
			addr = "127.0.0.1:8080"
		}
		url := "http://" + addr
		fmt.Printf("web UI: %s  (Ctrl+C to stop)\n", url)
		server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
		var openCmd *exec.Cmd
		if runtime.GOOS == "windows" {
			openCmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		} else if runtime.GOOS == "darwin" {
			openCmd = exec.Command("open", url)
		} else {
			openCmd = exec.Command("xdg-open", url)
		}
		_ = openCmd.Start()
		if err := server.ListenAndServe(); err != nil {
			return fmt.Errorf("web UI start failed (port in use? try: ninecli web --bind 127.0.0.1:8901): %w", err)
		}
		return nil
	},
}

func init() {
	webCmd.Flags().StringVar(&flagWebBind, "bind", "127.0.0.1:8080", "bind address for the web UI")
	rootCmd.AddCommand(webCmd)
}

// ---- shared helpers ----

var webMu sync.Mutex

func webReadJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v)
}

func webWrite(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type webError struct {
	Error string `json:"error"`
}

func webFail(w http.ResponseWriter, status int, msg string) {
	webWrite(w, status, webError{Error: msg})
}

// webMsg extracts a human-readable error message from either the
// business envelope ("msg") or the passport envelope ("resultDesc").
func webMsg(m map[string]any) string {
	if s := bizMsg(m); s != "" {
		return s
	}
	if s := strField(m, "resultDesc"); s != "" {
		return s
	}
	return strFieldData(m, "msg")
}

func webSession() (*api.Client, *config.Dir, *config.Tokens, error) {
	d := dirOrDefaults()
	cfg, err := d.Load()
	if err != nil {
		return nil, nil, nil, err
	}
	tok, err := d.LoadTokens()
	if err != nil {
		return nil, nil, nil, err
	}
	if tok == nil || tok.AccessToken == "" {
		return nil, nil, nil, fmt.Errorf("not logged in")
	}
	return api.New(hostsOrDefaults(), cfg, tok), &d, tok, nil
}

func webBusiness(c *api.Client, d *config.Dir, tok *config.Tokens) error {
	webMu.Lock()
	defer webMu.Unlock()
	return ensureBusinessUID(c, d, tok)
}

// ---- handlers ----

func handleWebIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	b, err := webuiFS.ReadFile("webui.html")
	if err != nil {
		http.Error(w, "ui missing", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}

func handleWebState(w http.ResponseWriter, r *http.Request) {
	d := dirOrDefaults()
	tok, _ := d.LoadTokens()
	out := map[string]any{"logged_in": false}
	if tok != nil && tok.AccessToken != "" {
		out["logged_in"] = true
		out["phone"] = tok.Phone
		out["login_type"] = tok.LoginType
		if vs, err := d.LoadVehicleCache(); err == nil {
			out["vehicles"] = vs
		} else {
			out["vehicles"] = []any{}
		}
	}
	webWrite(w, 200, out)
}

func handleWebLogin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		User string `json:"user"`
		Pass string `json:"pass"`
		Area string `json:"area"`
	}
	if err := webReadJSON(r, &in); err != nil || in.User == "" || in.Pass == "" {
		webFail(w, 400, "\u8bf7\u586b\u5199\u624b\u673a\u53f7\u548c\u5bc6\u7801")
		return
	}
	area := in.Area
	if area == "" {
		area = "86"
	}
	d := dirOrDefaults()
	cfg, err := d.Load()
	if err != nil {
		webFail(w, 500, err.Error())
		return
	}
	client := api.New(hostsOrDefaults(), cfg, nil)
	m, err := client.Login(area, in.User, in.Pass)
	if err != nil {
		webFail(w, 502, err.Error())
		return
	}
	if !api.IsSuccess(m) {
		webFail(w, 200, "\u767b\u5f55\u5931\u8d25: "+webMsg(m))
		return
	}
	tok, _ := d.LoadTokens()
	if tok == nil {
		tok = &config.Tokens{}
	}
	tok.LoginType = "password"
	tok.Phone = in.User
	webMu.Lock()
	err = finishLogin(&d, tok, m)
	webMu.Unlock()
	if err != nil {
		webFail(w, 500, err.Error())
		return
	}
	if err := businessAfterLogin(client, &d, tok); err != nil {
		_ = err
	}
	_ = syncTokensToOrig(tok)
	webWrite(w, 200, map[string]any{"ok": true})
}

func handleWebSendCode(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Phone string `json:"phone"`
		Area  string `json:"area"`
	}
	if err := webReadJSON(r, &in); err != nil || in.Phone == "" {
		webFail(w, 400, "\u8bf7\u586b\u5199\u624b\u673a\u53f7")
		return
	}
	area := in.Area
	if area == "" {
		area = "86"
	}
	d := dirOrDefaults()
	cfg, err := d.Load()
	if err != nil {
		webFail(w, 500, err.Error())
		return
	}
	client := api.New(hostsOrDefaults(), cfg, nil)
	m, err := client.SendCode(area, in.Phone)
	if err != nil {
		webFail(w, 502, err.Error())
		return
	}
	if !api.IsSuccess(m) {
		webFail(w, 200, "\u53d1\u9001\u5931\u8d25: "+webMsg(m))
		return
	}
	webWrite(w, 200, map[string]any{"ok": true})
}

func handleWebLoginCode(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Phone string `json:"phone"`
		Code  string `json:"code"`
		Area  string `json:"area"`
	}
	if err := webReadJSON(r, &in); err != nil || in.Phone == "" || in.Code == "" {
		webFail(w, 400, "\u8bf7\u586b\u5199\u624b\u673a\u53f7\u548c\u9a8c\u8bc1\u7801")
		return
	}
	area := in.Area
	if area == "" {
		area = "86"
	}
	d := dirOrDefaults()
	cfg, err := d.Load()
	if err != nil {
		webFail(w, 500, err.Error())
		return
	}
	client := api.New(hostsOrDefaults(), cfg, nil)
	m, err := client.PhoneCodeLogin(area, in.Phone, in.Code, "bj")
	if err != nil {
		webFail(w, 502, err.Error())
		return
	}
	if !api.IsSuccess(m) {
		webFail(w, 200, "\u767b\u5f55\u5931\u8d25: "+webMsg(m))
		return
	}
	tok, _ := d.LoadTokens()
	if tok == nil {
		tok = &config.Tokens{}
	}
	tok.LoginType = "sms"
	tok.Phone = in.Phone
	webMu.Lock()
	err = finishLogin(&d, tok, m)
	webMu.Unlock()
	if err != nil {
		webFail(w, 500, err.Error())
		return
	}
	if err := businessAfterLogin(client, &d, tok); err != nil {
		_ = err
	}
	_ = syncTokensToOrig(tok)
	webWrite(w, 200, map[string]any{"ok": true})
}

func handleWebLogout(w http.ResponseWriter, r *http.Request) {
	d := dirOrDefaults()
	if err := d.DeleteTokens(); err != nil {
		webFail(w, 500, err.Error())
		return
	}
	removeOrigTokens()
	webWrite(w, 200, map[string]any{"ok": true})
}

func handleWebVehicles(w http.ResponseWriter, r *http.Request) {
	cl, d, tok, err := webSession()
	if err != nil {
		webFail(w, 500, err.Error())
		return
	}
	if err := ensureBusinessUID(cl, d, tok); err != nil {
		webFail(w, 500, err.Error())
		return
	}
	m, err := cl.MyVehicle()
	if err != nil {
		webFail(w, 502, err.Error())
		return
	}
	if !api.IsBizSuccess(m) {
		webFail(w, 502, webMsg(m))
		return
	}
	vs := vehicleRows(m)
	_ = d.SaveVehicleCache(vs)
	out := make([]map[string]any, 0, len(vs))
	for _, v := range vs {
		out = append(out, map[string]any{
			"sn": v.SN, "name": v.Name, "product": v.Product, "shared": v.Shared,
		})
	}
	webWrite(w, 200, map[string]any{"vehicles": out})
}

func handleWebStatus(w http.ResponseWriter, r *http.Request) {
	sn := r.URL.Query().Get("sn")
	if sn == "" {
		webFail(w, 400, "missing sn")
		return
	}
	cl, d, tok, err := webSession()
	if err != nil {
		webFail(w, 500, err.Error())
		return
	}
	if err := ensureBusinessUID(cl, d, tok); err != nil {
		webFail(w, 500, err.Error())
		return
	}
	m, err := cl.DesktopComponent(sn)
	if err != nil {
		webFail(w, 502, err.Error())
		return
	}
	if !api.IsBizSuccess(m) {
		webFail(w, 502, webMsg(m))
		return
	}
	out := firstObject(m["data"])
	// Flatten loc for the frontend
	if loc, ok := out["loc"].(map[string]any); ok {
		for k, v := range loc {
			out[k] = v
		}
	}
	webWrite(w, 200, out)
}

func handleWebBattery(w http.ResponseWriter, r *http.Request) {
	sn := r.URL.Query().Get("sn")
	if sn == "" {
		webFail(w, 400, "missing sn")
		return
	}
	cl, d, tok, err := webSession()
	if err != nil {
		webFail(w, 500, err.Error())
		return
	}
	if err := ensureBusinessUID(cl, d, tok); err != nil {
		webFail(w, 500, err.Error())
		return
	}
	m, err := cl.BatteryInfo(sn)
	if err != nil {
		webFail(w, 502, err.Error())
		return
	}
	if !api.IsBizSuccess(m) {
		webFail(w, 502, webMsg(m))
		return
	}
	webWrite(w, 200, firstObject(m["data"]))
}

func handleWebTravel(w http.ResponseWriter, r *http.Request) {
	sn := r.URL.Query().Get("sn")
	month := r.URL.Query().Get("month")
	if sn == "" {
		webFail(w, 400, "missing sn")
		return
	}
	cl, d, tok, err := webSession()
	if err != nil {
		webFail(w, 500, err.Error())
		return
	}
	if err := ensureBusinessUID(cl, d, tok); err != nil {
		webFail(w, 500, err.Error())
		return
	}
	if month == "" {
		month = api.CurrentMonthYYYYMM()
	}
	m, err := cl.TravelList(sn, month)
	if err != nil {
		webFail(w, 502, err.Error())
		return
	}
	if !api.IsBizSuccess(m) {
		webFail(w, 502, webMsg(m))
		return
	}
	webWrite(w, 200, firstObject(m["data"]))
}

func handleWebControl(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Op string `json:"op"`
		SN string `json:"sn"`
	}
	if err := webReadJSON(r, &in); err != nil || in.Op == "" || in.SN == "" {
		webFail(w, 400, "missing op or sn")
		return
	}
	if _, ok := api.LookupControlOp(in.Op); !ok {
		webFail(w, 400, "unknown op: "+in.Op)
		return
	}
	origCmd, ok := origControlCmd[in.Op]
	if !ok {
		webFail(w, 400, "no original binary mapping for op: "+in.Op)
		return
	}
	out, err := runOrig(origCmd, in.SN, "-y")
	if err != nil {
		webFail(w, 502, err.Error())
		return
	}
	// Try to parse JSON output; if not JSON, just return ok
	var m map[string]any
	if json.Unmarshal([]byte(out), &m) == nil {
		webWrite(w, 200, m)
	} else {
		webWrite(w, 200, map[string]any{"ok": true, "op": in.Op})
	}
}

// Package proxy implements the plaintext REST server (`ninecli serve`)
// that translates REST requests into Ninebot's encrypted APIs.
package proxy

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"ninecli/internal/api"
	"ninecli/internal/config"
)

// Server is the serve-mode REST proxy.
type Server struct {
	Client *api.Client
	Dir    config.Dir
	Quiet  bool
	Token  string // when set, require Bearer auth on every non-/healthz call
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) Header() http.Header { return s.ResponseWriter.Header() }

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// NewServer wires the proxy.
func NewServer(c *api.Client, d config.Dir, quiet bool, token string) *Server {
	return &Server{Client: c, Dir: d, Quiet: quiet, Token: token}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{
		"ok":    false,
		"error": map[string]any{"code": code, "message": msg},
	})
}

func writeUpstreamError(w http.ResponseWriter, err error) {
	code := "upstream_error"
	msg := err.Error()
	var apiErr *api.Error
	if asAPIError(err, &apiErr) {
		code = "upstream_error"
		msg = apiErr.Message
		if apiErr.Status == 401 {
			code = "token_expired"
		}
	}
	writeJSON(w, http.StatusBadGateway, map[string]any{
		"ok":    false,
		"error": map[string]any{"code": code, "message": msg},
	})
}

func writeProxyAuthError(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusUnauthorized, map[string]any{
		"ok":    false,
		"error": map[string]any{"code": "unauthorized", "message": msg},
	})
}

func notFoundHandler(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "no such endpoint")
}

func methodNotAllowed(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
}

// isValidMonth validates YYYYMM.
func isValidMonth(s string) bool {
	if len(s) != 6 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	var y, m int
	_, err := fmt.Sscanf(s, "%4d%2d", &y, &m)
	return err == nil && m >= 1 && m <= 12
}

// currentMonth returns the current YYYYMM.
func currentMonth() string { return api.CurrentMonthYYYYMM() }

// redactSN masks the middle of a serial number in logs.
func redactSN(sn string) string {
	if len(sn) <= 8 {
		return sn
	}
	return sn[:4] + "****" + sn[len(sn)-4:]
}

// extractUpstreamCode digs the upstream error code out of a payload.
func extractUpstreamCode(m map[string]any) string {
	for _, k := range []string{"code", "status", "error"} {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// readJSONStrict decodes exactly one JSON object body.
func readJSONStrict(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("bad json body: %w", err)
	}
	return nil
}

// requireBearerFor wraps a handler with optional Bearer auth.
func (s *Server) requireBearerFor(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.Token == "" {
			next(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+s.Token {
			writeProxyAuthError(w, "missing or invalid bearer token")
			return
		}
		next(w, r)
	}
}

// IsLoopback reports whether the bind address is loopback-only.
func IsLoopback(hostPort string) bool {
	host, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Handler builds the HTTP mux.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": "healthy"})
	})

	mux.HandleFunc("/auth/login", s.requireBearerFor(s.handleLogin))
	mux.HandleFunc("/auth/login-code", s.requireBearerFor(s.handleLoginCode))
	mux.HandleFunc("/auth/login-code/consume", s.requireBearerFor(s.handleConsumeCode))
	mux.HandleFunc("/auth/refresh", s.requireBearerFor(s.handleRefresh))
	mux.HandleFunc("/whoami", s.requireBearerFor(s.handleWhoami))
	mux.HandleFunc("/vehicles", s.requireBearerFor(s.handleVehicles))
	mux.HandleFunc("/vehicles/{sn}/status", s.requireBearerFor(func(w http.ResponseWriter, r *http.Request) {
		s.vehicleGet(w, r.PathValue("sn"), func(sn string) (map[string]any, error) { return s.Client.DesktopComponent(sn) })
	}))
	mux.HandleFunc("/vehicles/{sn}/battery", s.requireBearerFor(func(w http.ResponseWriter, r *http.Request) {
		s.vehicleGet(w, r.PathValue("sn"), func(sn string) (map[string]any, error) { return s.Client.BatteryInfo(sn) })
	}))
	mux.HandleFunc("/vehicles/{sn}/travel", s.requireBearerFor(s.handleTravelList))
	mux.HandleFunc("/vehicles/{sn}/travel/{detail}", s.requireBearerFor(func(w http.ResponseWriter, r *http.Request) {
		detail := r.PathValue("detail")
		if detail == "" {
			notFoundHandler(w, r)
			return
		}
		s.vehicleGet(w, r.PathValue("sn"), func(sn string) (map[string]any, error) { return s.Client.TravelDetail(sn, detail) })
	}))
	mux.HandleFunc("/vehicles/{sn}/engine/start", s.requireBearerFor(func(w http.ResponseWriter, r *http.Request) {
		s.vehicleControl(w, r.PathValue("sn"), "engine_start")
	}))
	mux.HandleFunc("/vehicles/{sn}/engine/stop", s.requireBearerFor(func(w http.ResponseWriter, r *http.Request) {
		s.vehicleControl(w, r.PathValue("sn"), "engine_stop")
	}))
	mux.HandleFunc("/vehicles/{sn}/buck", s.requireBearerFor(func(w http.ResponseWriter, r *http.Request) {
		s.vehicleControl(w, r.PathValue("sn"), "buck")
	}))
	mux.HandleFunc("/vehicles/{sn}/bell", s.requireBearerFor(func(w http.ResponseWriter, r *http.Request) {
		s.vehicleControl(w, r.PathValue("sn"), "bell")
	}))

	mux.HandleFunc("/", notFoundHandler)
	return mux
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	var in struct {
		Account  string `json:"account"`
		Password string `json:"password"`
	}
	if err := readJSONStrict(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	cfg, err := s.Dir.Load()
	if err != nil {
		writeUpstreamError(w, err)
		return
	}
	c := api.New(s.Client.Hosts, cfg, nil)
	m, err := c.Login(cfg.AreaCode, in.Account, in.Password)
	if err != nil {
		writeUpstreamError(w, err)
		return
	}
	if !api.IsSuccess(m) {
		writeUpstreamError(w, &api.Error{Status: 200, Code: extractUpstreamCode(m), Message: str(m, "msg")})
		return
	}
	tok, _ := s.Dir.LoadTokens()
	if tok == nil {
		tok = &config.Tokens{}
	}
	applyTokenFields(tok, api.ExtractTokens(m))
	tok.LoginType = "password"
	tok.Phone = in.Account
	if err := s.Dir.SaveTokens(tok); err != nil {
		writeUpstreamError(w, err)
		return
	}
	if err := businessExchange(c, s.Dir, tok); err != nil {
		writeUpstreamError(w, err)
		return
	}
	s.Client.Tokens = tok
	s.redactedLog("login ok for", redactSN(in.Account))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": "logged in"})
}

func (s *Server) handleLoginCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	var in struct {
		Account string `json:"account"`
	}
	if err := readJSONStrict(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	cfg, err := s.Dir.Load()
	if err != nil {
		writeUpstreamError(w, err)
		return
	}
	c := api.New(s.Client.Hosts, cfg, nil)
	m, err := c.SendCode(cfg.AreaCode, in.Account)
	if err != nil {
		writeUpstreamError(w, err)
		return
	}
	if !api.IsSuccess(m) {
		writeUpstreamError(w, &api.Error{Status: 200, Code: extractUpstreamCode(m), Message: str(m, "msg")})
		return
	}
	s.redactedLog("send-code ok for", redactSN(in.Account))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": "code sent"})
}

func (s *Server) handleConsumeCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	var in struct {
		Account string `json:"account"`
		Code    string `json:"code"`
	}
	if err := readJSONStrict(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	cfg, err := s.Dir.Load()
	if err != nil {
		writeUpstreamError(w, err)
		return
	}
	c := api.New(s.Client.Hosts, cfg, nil)
	m, err := c.PhoneCodeLogin(cfg.AreaCode, in.Account, in.Code, "bj")
	if err != nil {
		writeUpstreamError(w, err)
		return
	}
	if !api.IsSuccess(m) {
		writeUpstreamError(w, &api.Error{Status: 200, Code: extractUpstreamCode(m), Message: str(m, "msg")})
		return
	}
	tok, _ := s.Dir.LoadTokens()
	if tok == nil {
		tok = &config.Tokens{}
	}
	applyTokenFields(tok, api.ExtractTokens(m))
	tok.LoginType = "phoneCode"
	tok.Phone = in.Account
	if err := s.Dir.SaveTokens(tok); err != nil {
		writeUpstreamError(w, err)
		return
	}
	if err := businessExchange(c, s.Dir, tok); err != nil {
		writeUpstreamError(w, err)
		return
	}
	s.Client.Tokens = tok
	s.redactedLog("consume-code ok for", redactSN(in.Account))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": "logged in"})
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	tok, err := s.Dir.LoadTokens()
	if err != nil || tok == nil {
		writeProxyAuthError(w, "no tokens on disk; login first")
		return
	}
	cfg, _ := s.Dir.Load()
	c := api.New(s.Client.Hosts, cfg, tok)
	m, err := c.Refresh()
	if err != nil {
		writeUpstreamError(w, err)
		return
	}
	if !api.IsSuccess(m) {
		writeUpstreamError(w, &api.Error{Status: 200, Code: extractUpstreamCode(m), Message: str(m, "msg")})
		return
	}
	applyTokenFields(tok, api.ExtractTokens(m))
	if err := s.Dir.SaveTokens(tok); err != nil {
		writeUpstreamError(w, err)
		return
	}
	s.Client.Tokens = tok
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": "refreshed"})
}

func (s *Server) handleWhoami(w http.ResponseWriter, _ *http.Request) {
	m, err := s.Client.V5User()
	if err != nil {
		writeUpstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": m})
}

func (s *Server) handleVehicles(w http.ResponseWriter, _ *http.Request) {
	m, err := s.Client.MyVehicle()
	if err != nil {
		writeUpstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": m})
}

func (s *Server) handleTravelList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	month := r.URL.Query().Get("month")
	if month == "" {
		month = currentMonth()
	}
	if !isValidMonth(month) {
		writeError(w, http.StatusBadRequest, "bad_request", "month must be YYYYMM")
		return
	}
	sn := r.PathValue("sn")
	s.vehicleGet(w, sn, func(sn string) (map[string]any, error) { return s.Client.TravelList(sn, month) })
}

func (s *Server) vehicleGet(w http.ResponseWriter, sn string, fn func(sn string) (map[string]any, error)) {
	m, err := fn(sn)
	if err != nil {
		writeUpstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": m})
}

func (s *Server) vehicleControl(w http.ResponseWriter, sn, op string) {
	m, err := s.Client.Control(op, sn)
	if err != nil {
		writeUpstreamError(w, err)
		return
	}
	s.redactedLog("control", op, "on", redactSN(sn))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": m})
}

func (s *Server) redactedLog(parts ...string) {
	if s.Quiet {
		return
	}
	log.Println(strings.Join(parts, " "))
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rec := &statusRecorder{ResponseWriter: w}
	start := time.Now()
	s.Handler().ServeHTTP(rec, r)
	if !s.Quiet {
		log.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Millisecond))
	}
}

package mcp

import (
	"net"
	"net/http"
)

// RequireBearer wraps an HTTP handler with optional Bearer auth.
func RequireBearer(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+token {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// LoopbackOnly reports whether host is a loopback address.
func LoopbackOnly(hostPort string) bool {
	host, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

package api

import (
	"net/http"
	"strings"
)

func withCORS(next http.Handler, allowedOrigin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && origin == allowedOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Vary", "Origin")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// withAuth returns middleware that validates a Bearer token on protected routes.
// If token is empty, all requests are allowed (open mode for development).
// Public routes (/health, /metrics, /ws/logs) are always exempt.
func withAuth(next http.Handler, token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}

		path := r.URL.Path
		if path == "/health" || strings.HasPrefix(path, "/health/") ||
			path == "/metrics" || path == "/ws/logs" {
			next.ServeHTTP(w, r)
			return
		}

		if !strings.HasPrefix(path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="logtailr"`)
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		const prefix = "Bearer "
		if !strings.HasPrefix(auth, prefix) {
			writeError(w, http.StatusUnauthorized, "invalid authorization header")
			return
		}

		if strings.TrimSpace(auth[len(prefix):]) != token {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

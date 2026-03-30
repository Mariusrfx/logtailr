package api

import (
	"net/http"
	"strings"
	"sync"
	"time"
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

// withAuth validates a Bearer token on all non-static routes.
// If token is empty, all requests are allowed (open mode for development).
// Only truly public routes are exempt: /health (liveness probe) and /metrics (Prometheus scrape).
// WebSocket accepts token via ?token= query param since browsers can't set headers on WS.
func withAuth(next http.Handler, token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}

		path := r.URL.Path

		// Public: liveness probe and Prometheus scrape only
		if path == "/health" || path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		// Static assets (frontend) — no auth needed
		if path == "/" || strings.HasPrefix(path, "/assets/") || path == "/favicon.svg" {
			next.ServeHTTP(w, r)
			return
		}

		// WebSocket: accept token via query param (browsers can't set WS headers)
		if path == "/ws/logs" {
			qToken := r.URL.Query().Get("token")
			if qToken == token {
				next.ServeHTTP(w, r)
				return
			}
			// Also check Authorization header for non-browser clients
			if extractBearerToken(r) == token {
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// All other routes: require Bearer token
		bearerToken := extractBearerToken(r)
		if bearerToken == "" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="logtailr"`)
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		if bearerToken != token {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if strings.HasPrefix(auth, prefix) {
		return strings.TrimSpace(auth[len(prefix):])
	}
	return ""
}

// withSecurityHeaders adds standard security headers to all responses.
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-XSS-Protection", "0") // Disabled in favor of CSP
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self' ws: wss:; img-src 'self' data:; font-src 'self'")

		next.ServeHTTP(w, r)
	})
}

// withRateLimit applies a simple per-IP rate limiter.
// Allows `limit` requests per `window` per IP. Returns 429 when exceeded.
func withRateLimit(next http.Handler, limit int, window time.Duration) http.Handler {
	type entry struct {
		count   int
		resetAt time.Time
	}

	var mu sync.Mutex
	clients := make(map[string]*entry)

	// Cleanup old entries periodically
	go func() {
		for {
			time.Sleep(window)
			mu.Lock()
			now := time.Now()
			for ip, e := range clients {
				if now.After(e.resetAt) {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't rate limit static assets or health checks
		path := r.URL.Path
		if path == "/" || strings.HasPrefix(path, "/assets/") ||
			path == "/favicon.svg" || path == "/health" || path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		ip := r.RemoteAddr
		if idx := strings.LastIndex(ip, ":"); idx != -1 {
			ip = ip[:idx]
		}
		// Trust X-Forwarded-For if behind proxy
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip = strings.TrimSpace(strings.Split(forwarded, ",")[0])
		}

		mu.Lock()
		e, ok := clients[ip]
		now := time.Now()
		if !ok || now.After(e.resetAt) {
			e = &entry{count: 0, resetAt: now.Add(window)}
			clients[ip] = e
		}
		e.count++
		count := e.count
		mu.Unlock()

		if count > limit {
			w.Header().Set("Retry-After", "60")
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}

		next.ServeHTTP(w, r)
	})
}

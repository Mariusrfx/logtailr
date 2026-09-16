package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newRateLimitedHandler builds a rate-limited 200 handler with an hour window
// (so the per-IP window never resets during the test) and a stop channel.
func newRateLimitedHandler(limit int, trustProxy ...bool) (http.Handler, chan struct{}) {
	stop := make(chan struct{})
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	tp := false
	if len(trustProxy) > 0 {
		tp = trustProxy[0]
	}
	return withRateLimit(inner, limit, time.Hour, stop, tp), stop
}

func TestRateLimit_UnderLimitPasses(t *testing.T) {
	h, stop := newRateLimitedHandler(5)
	defer close(stop)

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}
}

func TestRateLimit_OverLimitReturns429(t *testing.T) {
	h, stop := newRateLimitedHandler(3)
	defer close(stop)

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if i < 3 {
			if rec.Code != http.StatusOK {
				t.Fatalf("request %d: expected 200, got %d", i+1, rec.Code)
			}
			continue
		}
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("request %d: expected 429, got %d", i+1, rec.Code)
		}
		if rec.Header().Get("Retry-After") == "" {
			t.Error("expected Retry-After header on 429 response")
		}
	}
}

func TestRateLimit_PerIPIsolation(t *testing.T) {
	h, stop := newRateLimitedHandler(2)
	defer close(stop)

	// Exhaust the limit from IP A
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
		req.RemoteAddr = "192.0.2.1:1111"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if i == 2 && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("IP A request 3: expected 429, got %d", rec.Code)
		}
	}

	// IP B is not affected by IP A's exhaustion
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
	req.RemoteAddr = "192.0.2.2:2222"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("IP B: expected 200, got %d", rec.Code)
	}
}

func TestRateLimit_StopKeepsHandlerFunctional(t *testing.T) {
	h, stop := newRateLimitedHandler(3)

	// Stopping the cleanup goroutine must not break request handling
	close(stop)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 after stop, got %d", rec.Code)
	}
}

func TestRateLimit_XFFIgnoredByDefault(t *testing.T) {
	h, stop := newRateLimitedHandler(2)
	defer close(stop)

	// Every request comes from the same real IP but carries a different
	// spoofed X-Forwarded-For. Without proxy trust the limiter must see one
	// client and return 429 after the limit.
	for i := 0; i < 4; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
		req.RemoteAddr = "192.0.2.1:1111"
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", i+1))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if i < 2 {
			if rec.Code != http.StatusOK {
				t.Fatalf("request %d: expected 200, got %d", i+1, rec.Code)
			}
			continue
		}
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("request %d: spoofed X-Forwarded-For must not bypass the limit, got %d", i+1, rec.Code)
		}
	}
}

func TestRateLimit_XFFTrustedWhenProxy(t *testing.T) {
	h, stop := newRateLimitedHandler(1, true)
	defer close(stop)

	// With proxy trust, the spoofed X-Forwarded-For identifies the client.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
	req.RemoteAddr = "127.0.0.1:1111"
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", rec.Code)
	}

	// Same spoofed client exceeds its limit...
	req = httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
	req.RemoteAddr = "127.0.0.1:1111"
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("spoofed client over limit: expected 429, got %d", rec.Code)
	}

	// ...while another client is unaffected.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
	req.RemoteAddr = "127.0.0.1:1111"
	req.Header.Set("X-Forwarded-For", "203.0.113.11")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("other client: expected 200, got %d", rec.Code)
	}
}

func TestRateLimit_PublicPathsAreExempt(t *testing.T) {
	h, stop := newRateLimitedHandler(1)
	defer close(stop)

	// Exhaust the limit on a counted path
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
	}

	// Public paths must still pass even though the limit is exhausted
	for _, path := range []string{"/health", "/metrics", "/"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", path, rec.Code)
		}
	}
}

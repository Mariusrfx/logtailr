package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWithRequestID_GeneratesWhenAbsent(t *testing.T) {
	var fromCtx string
	h := withRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fromCtx = requestIDFrom(r)
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Header().Get(requestIDHeader) != fromCtx {
		t.Fatalf("header %q != context %q", rec.Header().Get(requestIDHeader), fromCtx)
	}
	if len(fromCtx) != 32 {
		t.Fatalf("generated id = %q, want 32 hex chars", fromCtx)
	}
}

func TestWithRequestID_HonorsValidInbound(t *testing.T) {
	const inbound = "abc-123.DEF_456/789"
	h := withRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requestIDFrom(r) != inbound {
			t.Fatalf("context id = %q, want %q", requestIDFrom(r), inbound)
		}
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(requestIDHeader, inbound)
	h.ServeHTTP(rec, req)

	if rec.Header().Get(requestIDHeader) != inbound {
		t.Fatalf("header = %q, want %q", rec.Header().Get(requestIDHeader), inbound)
	}
}

func TestWithRequestID_RejectsInvalidInbound(t *testing.T) {
	for _, inbound := range []string{
		strings.Repeat("a", 129), // too long
		"abc;def",                // invalid char
		"line\nbreak",            // invalid char
	} {
		var generated string
		h := withRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			generated = requestIDFrom(r)
		}))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(requestIDHeader, inbound)
		h.ServeHTTP(rec, req)

		if generated == inbound {
			t.Fatalf("invalid inbound id %q was honored", inbound)
		}
		if len(generated) != 32 {
			t.Fatalf("replacement id = %q, want 32 hex chars", generated)
		}
	}
}

func TestWriteError_IncludesRequestID(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {})
	var h http.Handler = inner
	h = withAuth(h, "secret")
	h = withRequestID(h)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	id := rec.Header().Get(requestIDHeader)
	if id == "" || body["request_id"] != id {
		t.Fatalf("request_id mismatch: header=%q body=%q", id, body["request_id"])
	}
	if body["error"] == "" {
		t.Fatal("error field missing")
	}
}

package output

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestWebhookWriter_InvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  WebhookConfig
		want string
	}{
		{"no url", WebhookConfig{}, "url is required"},
		{"bad url scheme", WebhookConfig{URL: "ftp://example.com"}, "must start with http"},
		{"bad batch size", WebhookConfig{URL: "https://example.com", BatchSize: 999}, "batch_size must be"},
		{"bad batch timeout", WebhookConfig{URL: "https://example.com", BatchTimeout: "nope"}, "invalid batch_timeout"},
		{"timeout too long", WebhookConfig{URL: "https://example.com", BatchTimeout: "5m"}, "batch_timeout must be"},
		{"bad min level", WebhookConfig{URL: "https://example.com", MinLevel: "nope"}, "invalid min_level"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewWebhookWriter(tt.cfg)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q should contain %q", err.Error(), tt.want)
			}
		})
	}
}

func TestWebhookWriter_SendsBatch(t *testing.T) {
	var received atomic.Value

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected content type: %s", r.Header.Get("Content-Type"))
		}

		body, _ := io.ReadAll(r.Body)
		received.Store(string(body))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ww, err := NewWebhookWriter(WebhookConfig{
		URL:       server.URL,
		BatchSize: 2,
	})
	if err != nil {
		t.Fatalf("NewWebhookWriter() error = %v", err)
	}

	_ = ww.Write(newTestLine("error", "first error"))
	_ = ww.Write(newTestLine("warn", "first warning"))
	_ = ww.Close()

	body, ok := received.Load().(string)
	if !ok || body == "" {
		t.Fatal("server received no data")
	}

	var payload webhookPayload
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("invalid JSON payload: %v", err)
	}

	if payload.Count != 2 {
		t.Errorf("count = %d, want 2", payload.Count)
	}
	if len(payload.Logs) != 2 {
		t.Errorf("logs length = %d, want 2", len(payload.Logs))
	}
	if !strings.Contains(payload.Text, "Logtailr") {
		t.Errorf("text should contain Logtailr, got %q", payload.Text)
	}
}

func TestWebhookWriter_MinLevelFilter(t *testing.T) {
	var callCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ww, err := NewWebhookWriter(WebhookConfig{
		URL:       server.URL,
		MinLevel:  "error",
		BatchSize: 1,
	})
	if err != nil {
		t.Fatalf("NewWebhookWriter() error = %v", err)
	}

	// These should be filtered out
	_ = ww.Write(newTestLine("debug", "debug msg"))
	_ = ww.Write(newTestLine("info", "info msg"))
	_ = ww.Write(newTestLine("warn", "warn msg"))

	// This should pass through
	_ = ww.Write(newTestLine("error", "error msg"))
	_ = ww.Close()

	if callCount.Load() != 1 {
		t.Errorf("expected 1 webhook call (only error), got %d", callCount.Load())
	}
}

func TestWebhookWriter_FlushOnClose(t *testing.T) {
	var received atomic.Value

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received.Store(string(body))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ww, err := NewWebhookWriter(WebhookConfig{
		URL:       server.URL,
		BatchSize: 100, // large batch so it won't auto-flush
	})
	if err != nil {
		t.Fatalf("NewWebhookWriter() error = %v", err)
	}

	_ = ww.Write(newTestLine("error", "pending"))
	_ = ww.Close() // should flush the pending item

	body, ok := received.Load().(string)
	if !ok || body == "" {
		t.Fatal("Close() should flush remaining items")
	}

	var payload webhookPayload
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if payload.Count != 1 {
		t.Errorf("count = %d, want 1", payload.Count)
	}
}

func TestWebhookWriter_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	ww, err := NewWebhookWriter(WebhookConfig{
		URL:       server.URL,
		BatchSize: 1,
	})
	if err != nil {
		t.Fatalf("NewWebhookWriter() error = %v", err)
	}

	// Write should trigger flush which gets HTTP 500
	err = ww.Write(newTestLine("error", "test"))
	_ = ww.Close()

	if err == nil {
		t.Error("expected error for HTTP 500, got nil")
	}
}

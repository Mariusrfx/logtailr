package alert

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestConsoleNotifier(t *testing.T) {
	n := NewConsoleNotifier()
	defer func() {
		if err := n.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	}()

	err := n.Notify(&Event{
		Rule:      "test-rule",
		Severity:  string(SeverityCritical),
		Message:   "test alert message",
		Source:    "app",
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("console notify failed: %v", err)
	}
}

func TestWebhookNotifier_Success(t *testing.T) {
	var received *Event
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json")
		}

		body, _ := io.ReadAll(r.Body)
		received = &Event{}
		_ = json.Unmarshal(body, received)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := NewWebhookNotifier(server.URL)
	defer func() {
		if err := n.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	}()

	event := &Event{
		Rule:      "test-rule",
		Severity:  string(SeverityWarning),
		Message:   "webhook test",
		Source:    "app",
		Timestamp: time.Now(),
	}

	err := n.Notify(event)
	if err != nil {
		t.Fatalf("webhook notify failed: %v", err)
	}

	if received == nil {
		t.Fatal("webhook did not receive event")
	}
	if received.Rule != "test-rule" {
		t.Errorf("expected rule 'test-rule', got %q", received.Rule)
	}
}

func TestWebhookNotifier_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	n := NewWebhookNotifier(server.URL)
	defer func() {
		if err := n.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	}()

	err := n.Notify(&Event{
		Rule:     "test",
		Severity: "warning",
		Message:  "test",
	})
	if err == nil {
		t.Fatal("expected error for server 500")
	}
}

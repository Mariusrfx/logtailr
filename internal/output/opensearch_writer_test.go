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

func TestOpenSearchWriter_InvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  OpenSearchConfig
		want string
	}{
		{"no hosts", OpenSearchConfig{Index: "logs"}, "at least one host"},
		{"no index", OpenSearchConfig{Hosts: []string{"http://localhost:9200"}}, "index is required"},
		{"bulk too large", OpenSearchConfig{Hosts: []string{"http://localhost:9200"}, Index: "logs", BulkSize: 99999}, "bulk_size must be"},
		{"bad flush interval", OpenSearchConfig{Hosts: []string{"http://localhost:9200"}, Index: "logs", FlushInterval: "nope"}, "invalid flush_interval"},
		{"flush too long", OpenSearchConfig{Hosts: []string{"http://localhost:9200"}, Index: "logs", FlushInterval: "5m"}, "flush_interval must be"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewOpenSearchWriter(tt.cfg)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q should contain %q", err.Error(), tt.want)
			}
		})
	}
}

func TestOpenSearchWriter_BulkInsert(t *testing.T) {
	var received atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle index template creation (startup)
		if strings.HasPrefix(r.URL.Path, "/_index_template/") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"acknowledged":true}`))
			return
		}

		if r.URL.Path != "/_bulk" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/x-ndjson" {
			t.Errorf("unexpected content type: %s", r.Header.Get("Content-Type"))
		}

		body, _ := io.ReadAll(r.Body)
		lines := strings.Split(strings.TrimSpace(string(body)), "\n")
		// Each doc = 2 lines (action + doc)
		received.Add(int32(len(lines) / 2))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errors":false}`))
	}))
	defer server.Close()

	ow, err := NewOpenSearchWriter(OpenSearchConfig{
		Hosts:    []string{server.URL},
		Index:    "test-logs",
		BulkSize: 3,
	})
	if err != nil {
		t.Fatalf("NewOpenSearchWriter() error = %v", err)
	}

	// Write 3 lines to trigger a bulk flush
	for i := range 3 {
		line := newTestLine("info", "message "+string(rune('A'+i)))
		if err := ow.Write(line); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}

	if err := ow.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if received.Load() != 3 {
		t.Errorf("server received %d docs, want 3", received.Load())
	}
}

func TestOpenSearchWriter_BasicAuth(t *testing.T) {
	var gotAuth atomic.Value

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/_index_template/") {
			_, _ = w.Write([]byte(`{"acknowledged":true}`))
			return
		}
		gotAuth.Store(r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"errors":false}`))
	}))
	defer server.Close()

	ow, err := NewOpenSearchWriter(OpenSearchConfig{
		Hosts:    []string{server.URL},
		Index:    "test",
		Username: "admin",
		Password: "secret",
		BulkSize: 1,
	})
	if err != nil {
		t.Fatalf("NewOpenSearchWriter() error = %v", err)
	}

	_ = ow.Write(newTestLine("info", "test"))
	_ = ow.Close()

	auth, ok := gotAuth.Load().(string)
	if !ok || auth == "" {
		t.Error("expected Authorization header, got none")
	}
	if !strings.HasPrefix(auth, "Basic ") {
		t.Errorf("expected Basic auth, got %q", auth)
	}
}

func TestOpenSearchWriter_IndexDatePattern(t *testing.T) {
	var gotPath atomic.Value

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/_index_template/") {
			_, _ = w.Write([]byte(`{"acknowledged":true}`))
			return
		}
		gotPath.Store(r.URL.Path)
		_, _ = w.Write([]byte(`{"errors":false}`))
	}))
	defer server.Close()

	ow, err := NewOpenSearchWriter(OpenSearchConfig{
		Hosts:    []string{server.URL},
		Index:    "logs-%{+YYYY.MM.dd}",
		BulkSize: 1,
	})
	if err != nil {
		t.Fatalf("NewOpenSearchWriter() error = %v", err)
	}

	_ = ow.Write(newTestLine("info", "test"))
	_ = ow.Close()

	// The path should be /_bulk (index is in the body)
	path, _ := gotPath.Load().(string)
	if path != "/_bulk" {
		t.Errorf("unexpected path: %s", path)
	}
}

func TestOpenSearchWriter_RetryOnFailure(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/_index_template/") {
			_, _ = w.Write([]byte(`{"acknowledged":true}`))
			return
		}
		n := attempts.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"errors":false}`))
	}))
	defer server.Close()

	ow, err := NewOpenSearchWriter(OpenSearchConfig{
		Hosts:      []string{server.URL},
		Index:      "test",
		BulkSize:   1,
		MaxRetries: 3,
	})
	if err != nil {
		t.Fatalf("NewOpenSearchWriter() error = %v", err)
	}

	_ = ow.Write(newTestLine("error", "test"))
	err = ow.Close()
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if attempts.Load() != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts.Load())
	}
}

func TestOpenSearchWriter_BulkBodyFormat(t *testing.T) {
	var gotBody atomic.Value

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/_index_template/") {
			_, _ = w.Write([]byte(`{"acknowledged":true}`))
			return
		}
		body, _ := io.ReadAll(r.Body)
		gotBody.Store(string(body))
		_, _ = w.Write([]byte(`{"errors":false}`))
	}))
	defer server.Close()

	ow, err := NewOpenSearchWriter(OpenSearchConfig{
		Hosts:    []string{server.URL},
		Index:    "test-index",
		BulkSize: 1,
	})
	if err != nil {
		t.Fatalf("NewOpenSearchWriter() error = %v", err)
	}

	_ = ow.Write(newTestLine("info", "hello"))
	_ = ow.Close()

	body, _ := gotBody.Load().(string)
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) != 2 {
		t.Fatalf("bulk body should have 2 lines (action + doc), got %d", len(lines))
	}

	// Verify action line
	var action map[string]map[string]string
	if err := json.Unmarshal([]byte(lines[0]), &action); err != nil {
		t.Fatalf("action line not valid JSON: %v", err)
	}
	if action["index"]["_index"] != "test-index" {
		t.Errorf("index = %q, want %q", action["index"]["_index"], "test-index")
	}

	// Verify doc line
	var doc map[string]interface{}
	if err := json.Unmarshal([]byte(lines[1]), &doc); err != nil {
		t.Fatalf("doc line not valid JSON: %v", err)
	}
	if doc["message"] != "hello" {
		t.Errorf("message = %q, want %q", doc["message"], "hello")
	}
}

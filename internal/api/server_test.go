package api

import (
	"encoding/json"
	"logtailr/internal/config"
	"logtailr/internal/health"
	"logtailr/pkg/logline"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func newTestServer(t *testing.T) (*Server, *health.Monitor) {
	t.Helper()
	monitor := health.NewMonitor()
	s := NewServer(ServerConfig{
		Addr:    "127.0.0.1:0",
		Monitor: monitor,
		Config: &config.Config{
			Sources: []logline.SourceConfig{
				{Name: "app.log", Type: "file", Path: "/var/log/app.log"},
			},
			Global: config.GlobalConfig{Level: "info", Output: "console"},
		},
	})
	return s, monitor
}

func TestHealthEndpoint_AllHealthy(t *testing.T) {
	s, monitor := newTestServer(t)
	monitor.RegisterSource("app.log")
	monitor.MarkHealthy("app.log")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}

	if body["status"] != "healthy" {
		t.Errorf("expected status healthy, got %v", body["status"])
	}
	sources := body["sources"].(map[string]interface{})
	if sources["healthy"] != float64(1) {
		t.Errorf("expected 1 healthy, got %v", sources["healthy"])
	}
}

func TestHealthEndpoint_Degraded(t *testing.T) {
	s, monitor := newTestServer(t)
	monitor.RegisterSource("app.log")
	monitor.RegisterSource("nginx")
	monitor.MarkHealthy("app.log")
	monitor.MarkDegraded("nginx", nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)

	var body map[string]interface{}
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body["status"] != "degraded" {
		t.Errorf("expected degraded, got %v", body["status"])
	}
}

func TestHealthEndpoint_Unhealthy(t *testing.T) {
	s, monitor := newTestServer(t)
	monitor.RegisterSource("app.log")
	monitor.MarkFailed("app.log", nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)

	var body map[string]interface{}
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body["status"] != "unhealthy" {
		t.Errorf("expected unhealthy, got %v", body["status"])
	}
}

func TestHealthSourcesEndpoint(t *testing.T) {
	s, monitor := newTestServer(t)
	monitor.RegisterSource("app.log")
	monitor.MarkHealthy("app.log")

	req := httptest.NewRequest(http.MethodGet, "/health/sources", nil)
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	_ = json.NewDecoder(rec.Body).Decode(&body)
	sources := body["sources"].([]interface{})
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	src := sources[0].(map[string]interface{})
	if src["name"] != "app.log" {
		t.Errorf("expected app.log, got %v", src["name"])
	}
	if src["status"] != "healthy" {
		t.Errorf("expected healthy, got %v", src["status"])
	}
}

func TestHealthSourceEndpoint_Found(t *testing.T) {
	s, monitor := newTestServer(t)
	monitor.RegisterSource("app.log")
	monitor.MarkHealthy("app.log")

	req := httptest.NewRequest(http.MethodGet, "/health/sources/app.log", nil)
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body["name"] != "app.log" {
		t.Errorf("expected app.log, got %v", body["name"])
	}
}

func TestHealthSourceEndpoint_NotFound(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/health/sources/nonexistent", nil)
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestConfigEndpoint(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/config", nil)
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	_ = json.NewDecoder(rec.Body).Decode(&body)
	sources := body["sources"].([]interface{})
	if len(sources) != 1 {
		t.Errorf("expected 1 source, got %d", len(sources))
	}
}

func TestConfigEndpoint_SanitizesSecrets(t *testing.T) {
	monitor := health.NewMonitor()
	s := NewServer(ServerConfig{
		Addr:    "127.0.0.1:0",
		Monitor: monitor,
		Config: &config.Config{
			Sources: []logline.SourceConfig{
				{Name: "app.log", Type: "file", Path: "/var/log/app.log"},
			},
			Global: config.GlobalConfig{Level: "info"},
			Outputs: config.OutputsConfig{
				OpenSearch: &config.OpenSearchOutputConfig{
					Enabled:  true,
					Hosts:    []string{"https://es.example.com:9200"},
					Index:    "logs",
					Username: "admin",
					Password: "super-secret-password",
				},
				Webhook: &config.WebhookOutputConfig{
					Enabled: true,
					URL:     "https://hooks.slack.com/services/secret",
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/config", nil)
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "super-secret-password") {
		t.Error("response contains OpenSearch password")
	}
	if strings.Contains(body, "hooks.slack.com") {
		t.Error("response contains webhook URL")
	}
}

func TestMetricsEndpoint(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "logtailr_active_sources") {
		t.Error("metrics response missing logtailr_active_sources")
	}
}

func TestCORSHeaders_AllowedOrigin(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "http://127.0.0.1:0")
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "http://127.0.0.1:0" {
		t.Errorf("expected allowed origin, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSHeaders_BlockedOrigin(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "http://evil.com")
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("expected no CORS header for blocked origin, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSPreflight(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}
}

func TestWebSocket_Connect(t *testing.T) {
	s, _ := newTestServer(t)
	go s.hub.Run()
	defer s.hub.Stop()

	ts := httptest.NewServer(s.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/logs"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Give hub time to register
	time.Sleep(50 * time.Millisecond)

	if s.hub.ClientCount() != 1 {
		t.Errorf("expected 1 client, got %d", s.hub.ClientCount())
	}
}

func TestWebSocket_ReceiveLogs(t *testing.T) {
	s, _ := newTestServer(t)
	go s.hub.Run()
	defer s.hub.Stop()

	ts := httptest.NewServer(s.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/logs"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer func() { _ = conn.Close() }()

	time.Sleep(50 * time.Millisecond)

	// Broadcast a log
	testLog := &logline.LogLine{
		Timestamp: time.Now(),
		Level:     "error",
		Message:   "test error message",
		Source:    "app.log",
	}
	s.hub.Broadcast(testLog)

	// Read from WebSocket
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	var received logline.LogLine
	if err := json.Unmarshal(msg, &received); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if received.Message != "test error message" {
		t.Errorf("expected 'test error message', got %q", received.Message)
	}
	if received.Level != "error" {
		t.Errorf("expected level error, got %q", received.Level)
	}
}

func TestWebSocket_LevelFilter(t *testing.T) {
	s, _ := newTestServer(t)
	go s.hub.Run()
	defer s.hub.Stop()

	ts := httptest.NewServer(s.httpServer.Handler)
	defer ts.Close()

	// Connect with level=error filter
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/logs?level=error"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer func() { _ = conn.Close() }()

	time.Sleep(50 * time.Millisecond)

	// Broadcast an info log (should be filtered)
	s.hub.Broadcast(&logline.LogLine{
		Timestamp: time.Now(),
		Level:     "info",
		Message:   "should not receive",
		Source:    "app.log",
	})

	// Broadcast an error log (should pass)
	s.hub.Broadcast(&logline.LogLine{
		Timestamp: time.Now(),
		Level:     "error",
		Message:   "should receive",
		Source:    "app.log",
	})

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	var received logline.LogLine
	_ = json.Unmarshal(msg, &received)
	if received.Message != "should receive" {
		t.Errorf("expected 'should receive', got %q", received.Message)
	}
}

func TestHub_Broadcast(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	client := &Client{
		Send: make(chan *logline.LogLine, 10),
	}
	hub.Register(client)
	time.Sleep(20 * time.Millisecond)

	testLog := &logline.LogLine{
		Level:   "error",
		Message: "test",
	}
	hub.Broadcast(testLog)

	select {
	case received := <-client.Send:
		if received.Message != "test" {
			t.Errorf("expected 'test', got %q", received.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for broadcast")
	}
}

func TestHub_SourceFilter(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	client := &Client{
		Send:       make(chan *logline.LogLine, 10),
		SourceName: "nginx",
	}
	hub.Register(client)
	time.Sleep(20 * time.Millisecond)

	// Send from app.log (should be filtered)
	hub.Broadcast(&logline.LogLine{Source: "app.log", Message: "filtered"})
	// Send from nginx (should pass)
	hub.Broadcast(&logline.LogLine{Source: "nginx", Message: "passed"})

	select {
	case received := <-client.Send:
		if received.Message != "passed" {
			t.Errorf("expected 'passed', got %q", received.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestMetrics_LogsTotal(t *testing.T) {
	s, _ := newTestServer(t)

	s.metrics.LogsTotal.WithLabelValues("app.log", "error").Inc()
	s.metrics.LogsTotal.WithLabelValues("app.log", "error").Inc()
	s.metrics.LogsTotal.WithLabelValues("app.log", "info").Inc()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `logtailr_logs_total{level="error",source="app.log"} 2`) {
		t.Error("expected 2 error logs for app.log in metrics")
	}
	if !strings.Contains(body, `logtailr_logs_total{level="info",source="app.log"} 1`) {
		t.Error("expected 1 info log for app.log in metrics")
	}
}

func TestHealthSource_NotFound_NoReflection(t *testing.T) {
	s, _ := newTestServer(t)

	// Path value should NOT be reflected in error response
	req := httptest.NewRequest(http.MethodGet, "/health/sources/<script>alert(1)</script>", nil)
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "<script>") {
		t.Error("error response reflects unsanitized user input")
	}
}

func TestSanitizeLabel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"app.log", "app.log"},
		{"nginx-container", "nginx-container"},
		{"", "unknown"},
		{"a]b[c{d", "unknown"},
		{strings.Repeat("a", 200), strings.Repeat("a", 128)},
	}

	for _, tt := range tests {
		got := SanitizeLabel(tt.input, 128)
		if got != tt.expected {
			t.Errorf("SanitizeLabel(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestSanitizeInput(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"hello\x00world", "helloworld"},
		{"  spaces  ", "spaces"},
		{strings.Repeat("x", 200), strings.Repeat("x", 128)},
	}

	for _, tt := range tests {
		got := sanitizeInput(tt.input, 128)
		if got != tt.expected {
			t.Errorf("sanitizeInput(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestHub_DoubleUnregister(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	client := &Client{
		Send: make(chan *logline.LogLine, 10),
	}
	hub.Register(client)
	time.Sleep(20 * time.Millisecond)

	// Double unregister should not panic
	hub.Unregister(client)
	time.Sleep(20 * time.Millisecond)
	hub.Unregister(client)
	time.Sleep(20 * time.Millisecond)
}

func TestConfigEndpoint_SanitizesUsername(t *testing.T) {
	monitor := health.NewMonitor()
	s := NewServer(ServerConfig{
		Addr:    "127.0.0.1:0",
		Monitor: monitor,
		Config: &config.Config{
			Sources: []logline.SourceConfig{
				{Name: "app.log", Type: "file", Path: "/var/log/app.log"},
			},
			Outputs: config.OutputsConfig{
				OpenSearch: &config.OpenSearchOutputConfig{
					Enabled:  true,
					Hosts:    []string{"https://es.example.com:9200"},
					Index:    "logs",
					Username: "admin",
					Password: "secret",
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/config", nil)
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "admin") {
		t.Error("response contains OpenSearch username")
	}
}

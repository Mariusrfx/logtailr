package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"logtailr/internal/config"
	"logtailr/internal/health"

	"github.com/gorilla/websocket"
)

func newTestServerForWS(t *testing.T) *httptest.Server {
	t.Helper()
	s := NewServer(ServerConfig{
		Addr:    "127.0.0.1:0",
		Monitor: health.NewMonitor(),
		Config:  &config.Config{},
	})
	ts := httptest.NewServer(s.httpServer.Handler)
	t.Cleanup(ts.Close)
	return ts
}

func dialWS(t *testing.T, ts *httptest.Server) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/logs"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// readClose waits for the server to close the connection and returns the
// CloseError, failing if a different kind of error arrives first.
func readClose(t *testing.T, conn *websocket.Conn) *websocket.CloseError {
	t.Helper()
	for {
		_, _, err := conn.ReadMessage()
		if ce, ok := err.(*websocket.CloseError); ok {
			return ce
		}
		if err != nil {
			t.Fatalf("expected CloseError, got %v", err)
		}
	}
}

func TestWebSocket_MessageRateLimitClosesConnection(t *testing.T) {
	ts := newTestServerForWS(t)
	conn := dialWS(t, ts)

	// Burst is 20; with no time to refill, message 21 triggers the close.
	for i := 0; i < 25; i++ {
		if err := conn.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
			break
		}
	}

	ce := readClose(t, conn)
	if ce.Code != websocket.ClosePolicyViolation {
		t.Fatalf("close code = %d, want %d (policy violation)", ce.Code, websocket.ClosePolicyViolation)
	}
}

func TestWebSocket_MessageAboveReadLimitClosedWith1009(t *testing.T) {
	ts := newTestServerForWS(t)
	conn := dialWS(t, ts)

	big := make([]byte, wsMaxMsgSize+1)
	if err := conn.WriteMessage(websocket.TextMessage, big); err != nil {
		t.Fatalf("write: %v", err)
	}

	ce := readClose(t, conn)
	if ce.Code != websocket.CloseMessageTooBig {
		t.Fatalf("close code = %d, want %d (message too big)", ce.Code, websocket.CloseMessageTooBig)
	}
}

func TestListAudit_WithoutStoreReturns503(t *testing.T) {
	s := NewServer(ServerConfig{
		Addr:    "127.0.0.1:0",
		Monitor: health.NewMonitor(),
		Config:  &config.Config{},
	})
	ts := httptest.NewServer(s.httpServer.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/audit")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
}

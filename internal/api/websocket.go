package api

import (
	"encoding/json"
	"log/slog"
	"logtailr/internal/safego"
	"logtailr/pkg/logline"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if s.hub.ClientCount() >= maxWsClients {
		http.Error(w, "too many WebSocket connections", http.StatusServiceUnavailable)
		return
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(req *http.Request) bool {
			origin := req.Header.Get("Origin")
			if origin == "" {
				return true
			}
			u, err := url.Parse(origin)
			if err != nil {
				return false
			}
			return u.Host == req.Host
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	levelFilter := strings.ToLower(r.URL.Query().Get("level"))
	sourceFilter := r.URL.Query().Get("source")

	if levelFilter != "" {
		if _, ok := logline.LogLevels[levelFilter]; !ok {
			levelFilter = ""
		}
	}
	sourceFilter = sanitizeInput(sourceFilter, maxSourceNameLen)

	client := &Client{
		Send:       make(chan *logline.LogLine, clientSendBuffer),
		MinLevel:   levelFilter,
		SourceName: sourceFilter,
	}

	s.hub.Register(client)
	s.metrics.WebSocketClients.Inc()

	safego.Go("ws-write", func() { s.wsWritePump(conn, client) }, nil)
	safego.Go("ws-read", func() { s.wsReadPump(conn, client) }, nil)
}

func (s *Server) wsWritePump(conn *websocket.Conn, client *Client) {
	ticker := time.NewTicker(wsPingPeriod)
	defer func() {
		ticker.Stop()
		_ = conn.Close()
		s.hub.Unregister(client)
		s.metrics.WebSocketClients.Dec()
	}()

	for {
		select {
		case line, ok := <-client.Send:
			_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if !ok {
				_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			data, err := json.Marshal(line)
			if err != nil {
				continue
			}
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}

		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (s *Server) wsReadPump(conn *websocket.Conn, client *Client) {
	defer func() {
		s.hub.Unregister(client)
		_ = conn.Close()
	}()

	conn.SetReadLimit(wsMaxMsgSize)
	_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})

	// Per-connection token bucket: clients can only send subscription/filter
	// updates, so a hard cap makes flooding a connection pointless.
	tokens := float64(wsMsgRateBurst)
	last := time.Now()

	for {
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			// Echo the close frame (e.g. 1009 message too big) per spec.
			if ce, ok := err.(*websocket.CloseError); ok {
				_ = conn.WriteControl(websocket.CloseMessage,
					websocket.FormatCloseMessage(ce.Code, ce.Text),
					time.Now().Add(time.Second))
			}
			return
		}

		now := time.Now()
		tokens += now.Sub(last).Seconds() * wsMsgRateLimit
		if tokens > wsMsgRateBurst {
			tokens = wsMsgRateBurst
		}
		last = now

		if msgType != websocket.TextMessage {
			slog.Debug("ws: ignoring non-text client message", "type", msgType)
			continue
		}

		if tokens < 1 {
			slog.Warn("ws: client message rate exceeded, closing connection", "remote", conn.RemoteAddr())
			_ = conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "message rate limit exceeded"),
				time.Now().Add(time.Second))
			return
		}
		tokens--

		slog.Debug("ws: client message", "bytes", len(msg), "msg", truncateString(string(msg), wsMaxLoggedMsg))
	}
}

func truncateString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

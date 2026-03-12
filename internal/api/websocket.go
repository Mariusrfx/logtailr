package api

import (
	"encoding/json"
	"logtailr/pkg/logline"
	"net/http"
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
			host := req.Host
			return strings.Contains(origin, host)
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

	go s.wsWritePump(conn, client)
	go s.wsReadPump(conn, client)
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

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			return
		}
	}
}

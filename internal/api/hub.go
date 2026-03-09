package api

import (
	"logtailr/pkg/logline"
	"sync"
)

const (
	hubBroadcastBuffer = 256
	clientSendBuffer   = 64
)

// Hub manages WebSocket client subscriptions and broadcasts log lines.
type Hub struct {
	mu         sync.RWMutex
	clients    map[*Client]bool
	broadcast  chan *logline.LogLine
	register   chan *Client
	unregister chan *Client
	done       chan struct{}
}

// Client represents a connected WebSocket subscriber.
type Client struct {
	Send       chan *logline.LogLine
	MinLevel   string // optional filter: only send logs >= this level
	SourceName string // optional filter: only send logs from this source
	closed     bool   // guard against double-close
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan *logline.LogLine, hubBroadcastBuffer),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		done:       make(chan struct{}),
	}
}

// Run starts the hub's main loop. Call in a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				if !client.closed {
					client.closed = true
					close(client.Send)
				}
			}
			h.mu.Unlock()

		case line := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				if !matchesFilter(client, line) {
					continue
				}
				select {
				case client.Send <- line:
				default:
					// Client too slow, drop message
				}
			}
			h.mu.RUnlock()

		case <-h.done:
			h.mu.Lock()
			for client := range h.clients {
				if !client.closed {
					client.closed = true
					close(client.Send)
				}
				delete(h.clients, client)
			}
			h.mu.Unlock()
			return
		}
	}
}

// Broadcast sends a log line to all connected clients.
func (h *Hub) Broadcast(line *logline.LogLine) {
	select {
	case h.broadcast <- line:
	default:
		// Hub buffer full, drop
	}
}

// Register adds a client to the hub.
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// Stop shuts down the hub.
func (h *Hub) Stop() {
	close(h.done)
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func matchesFilter(client *Client, line *logline.LogLine) bool {
	if client.SourceName != "" && client.SourceName != line.Source {
		return false
	}
	if client.MinLevel != "" {
		minLvl, ok := logline.LogLevels[client.MinLevel]
		if !ok {
			return true
		}
		lineLvl, ok := logline.LogLevels[line.Level]
		if !ok {
			return true
		}
		if lineLvl < minLvl {
			return false
		}
	}
	return true
}

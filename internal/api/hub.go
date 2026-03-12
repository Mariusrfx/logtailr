package api

import (
	"logtailr/pkg/logline"
	"sync"
)

const (
	hubBroadcastBuffer = 256
	clientSendBuffer   = 64
)

type Hub struct {
	mu         sync.RWMutex
	clients    map[*Client]bool
	broadcast  chan *logline.LogLine
	register   chan *Client
	unregister chan *Client
	done       chan struct{}
}

type Client struct {
	Send       chan *logline.LogLine
	MinLevel   string
	SourceName string
	closed     bool
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan *logline.LogLine, hubBroadcastBuffer),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		done:       make(chan struct{}),
	}
}

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

func (h *Hub) Broadcast(line *logline.LogLine) {
	select {
	case h.broadcast <- line:
	default:
	}
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

func (h *Hub) Stop() {
	close(h.done)
}

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

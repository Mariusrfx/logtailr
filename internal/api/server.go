package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"logtailr/internal/config"
	"logtailr/internal/health"
	"logtailr/pkg/logline"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	wsWriteWait      = 10 * time.Second
	wsPingPeriod     = 30 * time.Second
	wsPongWait       = 60 * time.Second
	wsMaxMsgSize     = 512
	shutdownWait     = 5 * time.Second
	maxSourceNameLen = 128
	maxWsClients     = 100
)

// safeLabel matches valid Prometheus label values (alphanumeric, dash, underscore, dot, slash, colon).
var safeLabel = regexp.MustCompile(`^[a-zA-Z0-9/_.:@-]+$`)

// Server is the HTTP API server exposing health, metrics, and WebSocket endpoints.
type Server struct {
	httpServer *http.Server
	monitor    *health.Monitor
	hub        *Hub
	metrics    *Metrics
	registry   *prometheus.Registry
	cfg        *config.Config
	startTime  time.Time
	cancelCtx  context.CancelFunc
}

// ServerConfig holds configuration for the API server.
type ServerConfig struct {
	Addr    string // bind address, e.g. "127.0.0.1:8080"
	Monitor *health.Monitor
	Config  *config.Config
}

// NewServer creates a new API server.
func NewServer(sc ServerConfig) *Server {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector())

	s := &Server{
		monitor:   sc.Monitor,
		hub:       NewHub(),
		registry:  registry,
		cfg:       sc.Config,
		startTime: time.Now(),
	}
	s.metrics = NewMetrics(registry)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /health/sources", s.handleHealthSources)
	mux.HandleFunc("GET /health/sources/{name}", s.handleHealthSource)
	mux.HandleFunc("GET /config", s.handleConfig)
	mux.HandleFunc("GET /ws/logs", s.handleWebSocket)
	mux.Handle("GET /metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	allowedOrigin := fmt.Sprintf("http://%s", sc.Addr)

	s.httpServer = &http.Server{
		Addr:              sc.Addr,
		Handler:           withCORS(mux, allowedOrigin),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	return s
}

// Start starts the API server and hub in background goroutines.
func (s *Server) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelCtx = cancel
	go s.hub.Run()
	go s.runMetricsUpdater(ctx)
	go func() {
		log.Printf("API server listening on %s", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("API server error: %v", err)
		}
	}()
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() error {
	if s.cancelCtx != nil {
		s.cancelCtx()
	}
	s.hub.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), shutdownWait)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

// Hub returns the broadcast hub for sending logs to WebSocket clients.
func (s *Server) Hub() *Hub {
	return s.hub
}

// Metrics returns the Prometheus metrics for recording from the pipeline.
func (s *Server) Metrics() *Metrics {
	return s.metrics
}

// --- REST Handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	healthy, degraded, failed, stopped := s.monitor.GetHealthCount()
	total := healthy + degraded + failed + stopped

	status := "healthy"
	if failed > 0 {
		status = "unhealthy"
	} else if degraded > 0 {
		status = "degraded"
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    status,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"uptime":    time.Since(s.startTime).Round(time.Second).String(),
		"sources": map[string]int{
			"total":    total,
			"healthy":  healthy,
			"degraded": degraded,
			"failed":   failed,
			"stopped":  stopped,
		},
	})
}

func (s *Server) handleHealthSources(w http.ResponseWriter, _ *http.Request) {
	statuses := s.monitor.GetAllStatuses()
	sources := make([]map[string]interface{}, 0, len(statuses))

	for _, sh := range statuses {
		entry := map[string]interface{}{
			"name":        sh.Name,
			"status":      string(sh.Status),
			"error_count": sh.ErrorCount,
			"last_update": sh.LastUpdate.UTC().Format(time.RFC3339),
			"uptime":      time.Since(sh.StartTime).Round(time.Second).String(),
		}
		if sh.LastError != nil {
			entry["last_error"] = sh.LastError.Error()
		}
		sources = append(sources, entry)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"sources": sources,
	})
}

func (s *Server) handleHealthSource(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	// Sanitize: truncate and strip non-printable characters
	name = sanitizeInput(name, maxSourceNameLen)

	sh, ok := s.monitor.GetStatus(name)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "source not found",
		})
		return
	}

	entry := map[string]interface{}{
		"name":        sh.Name,
		"status":      string(sh.Status),
		"error_count": sh.ErrorCount,
		"last_update": sh.LastUpdate.UTC().Format(time.RFC3339),
		"uptime":      time.Since(sh.StartTime).Round(time.Second).String(),
	}
	if sh.LastError != nil {
		entry["last_error"] = sh.LastError.Error()
	}

	writeJSON(w, http.StatusOK, entry)
}

func (s *Server) handleConfig(w http.ResponseWriter, _ *http.Request) {
	if s.cfg == nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"mode": "single-file",
		})
		return
	}

	// Sanitize: remove secrets from output
	sanitized := map[string]interface{}{
		"sources": s.cfg.Sources,
		"global":  s.cfg.Global,
	}

	if s.cfg.Outputs.OpenSearch != nil {
		osCfg := *s.cfg.Outputs.OpenSearch
		osCfg.Password = "***"
		osCfg.Username = "***"
		sanitized["outputs_opensearch"] = osCfg
	}
	if s.cfg.Outputs.Webhook != nil {
		sanitized["outputs_webhook"] = map[string]interface{}{
			"enabled":       s.cfg.Outputs.Webhook.Enabled,
			"min_level":     s.cfg.Outputs.Webhook.MinLevel,
			"batch_size":    s.cfg.Outputs.Webhook.BatchSize,
			"batch_timeout": s.cfg.Outputs.Webhook.BatchTimeout,
			"url":           "***",
		}
	}

	writeJSON(w, http.StatusOK, sanitized)
}

// --- WebSocket Handler ---

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Limit concurrent WebSocket connections
	if s.hub.ClientCount() >= maxWsClients {
		http.Error(w, "too many WebSocket connections", http.StatusServiceUnavailable)
		return
	}

	// Validate origin: allow same-host connections only
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(req *http.Request) bool {
			origin := req.Header.Get("Origin")
			if origin == "" {
				return true // Non-browser clients (curl, wscat)
			}
			// Allow if origin host matches the server's listen address
			host := req.Host
			return strings.Contains(origin, host)
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	// Parse optional query filters
	levelFilter := strings.ToLower(r.URL.Query().Get("level"))
	sourceFilter := r.URL.Query().Get("source")

	// Validate filter values
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

// --- Helpers ---

func (s *Server) runMetricsUpdater(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.metrics.UpdateSourceHealth(s.monitor)
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func withCORS(next http.Handler, allowedOrigin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == allowedOrigin || origin == "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Vary", "Origin")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// sanitizeInput truncates and strips non-printable characters from user input.
func sanitizeInput(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	// Strip non-printable characters
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, s)
}

// SanitizeLabel returns a safe string for use as a Prometheus label value.
// Invalid values are replaced with "unknown".
func SanitizeLabel(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	if s == "" || !safeLabel.MatchString(s) {
		return "unknown"
	}
	return s
}

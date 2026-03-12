package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"logtailr/internal/alert"
	"logtailr/internal/config"
	"logtailr/internal/health"
	"net/http"
	"time"

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

type Server struct {
	httpServer  *http.Server
	monitor     *health.Monitor
	hub         *Hub
	metrics     *Metrics
	registry    *prometheus.Registry
	cfg         *config.Config
	alertEngine *alert.Engine
	startTime   time.Time
	cancelCtx   context.CancelFunc
}

type ServerConfig struct {
	Addr        string
	Monitor     *health.Monitor
	Config      *config.Config
	AlertEngine *alert.Engine
}

func NewServer(sc ServerConfig) *Server {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector())

	s := &Server{
		monitor:     sc.Monitor,
		hub:         NewHub(),
		registry:    registry,
		cfg:         sc.Config,
		alertEngine: sc.AlertEngine,
		startTime:   time.Now(),
	}
	s.metrics = NewMetrics(registry)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /health/sources", s.handleHealthSources)
	mux.HandleFunc("GET /health/sources/{name}", s.handleHealthSource)
	mux.HandleFunc("GET /config", s.handleConfig)
	mux.HandleFunc("GET /ws/logs", s.handleWebSocket)
	mux.HandleFunc("GET /alerts", s.handleAlerts)
	mux.HandleFunc("GET /alerts/rules", s.handleAlertRules)
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

func (s *Server) Stop() error {
	if s.cancelCtx != nil {
		s.cancelCtx()
	}
	s.hub.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), shutdownWait)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) Hub() *Hub {
	return s.hub
}

func (s *Server) Metrics() *Metrics {
	return s.metrics
}

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

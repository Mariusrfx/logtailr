package api

import (
	"logtailr/internal/health"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	LogsTotal          *prometheus.CounterVec
	AlertsTotal        *prometheus.CounterVec
	SourceHealthy      *prometheus.GaugeVec
	SourceErrorsTotal  *prometheus.CounterVec
	ProcessingDuration *prometheus.HistogramVec
	ActiveSources      prometheus.Gauge
	WebSocketClients   prometheus.Gauge
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		LogsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "logtailr_logs_total",
				Help: "Total number of log lines processed, by source and level.",
			},
			[]string{"source", "level"},
		),
		AlertsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "logtailr_alerts_total",
				Help: "Total number of alerts fired, by rule and severity.",
			},
			[]string{"rule", "severity"},
		),
		SourceHealthy: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "logtailr_source_healthy",
				Help: "Whether a source is healthy (1) or not (0).",
			},
			[]string{"source", "status"},
		),
		SourceErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "logtailr_source_errors_total",
				Help: "Total errors per source.",
			},
			[]string{"source"},
		),
		ProcessingDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "logtailr_processing_duration_seconds",
				Help:    "Time spent processing a log line.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"source"},
		),
		ActiveSources: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "logtailr_active_sources",
				Help: "Number of active (non-stopped) sources.",
			},
		),
		WebSocketClients: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "logtailr_websocket_clients",
				Help: "Number of connected WebSocket clients.",
			},
		),
	}

	reg.MustRegister(
		m.LogsTotal,
		m.AlertsTotal,
		m.SourceHealthy,
		m.SourceErrorsTotal,
		m.ProcessingDuration,
		m.ActiveSources,
		m.WebSocketClients,
	)

	return m
}

func (m *Metrics) UpdateSourceHealth(monitor *health.Monitor) {
	statuses := monitor.GetAllStatuses()
	healthy, degraded, failed, _ := monitor.GetHealthCount()
	m.ActiveSources.Set(float64(healthy + degraded + failed))

	for _, s := range statuses {
		for _, st := range []string{"healthy", "degraded", "failed", "stopped"} {
			m.SourceHealthy.WithLabelValues(s.Name, st).Set(0)
		}
		m.SourceHealthy.WithLabelValues(s.Name, string(s.Status)).Set(1)
	}
}

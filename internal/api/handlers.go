package api

import (
	"net/http"
	"time"
)

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

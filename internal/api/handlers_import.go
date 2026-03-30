package api

import (
	"fmt"
	"io"
	"net/http"

	"logtailr/internal/config"
)

func (s *Server) handleImportYAML(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}

	body := http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	defer func() { _ = body.Close() }()

	data, err := io.ReadAll(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	if len(data) == 0 {
		writeError(w, http.StatusBadRequest, "empty request body")
		return
	}

	cfg, err := config.ParseYAMLBytes(data, s.allowLocal)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("config validation failed: %v", err))
		return
	}

	if err := config.ImportToStore(r.Context(), s.store, cfg); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("import failed: %v", err))
		return
	}

	outputCount := 0
	if cfg.Outputs.OpenSearch != nil && cfg.Outputs.OpenSearch.Enabled {
		outputCount++
	}
	if cfg.Outputs.Webhook != nil && cfg.Outputs.Webhook.Enabled {
		outputCount++
	}
	if cfg.Outputs.File != nil {
		outputCount++
	}

	ruleCount := 0
	if cfg.Alerts != nil {
		ruleCount = len(cfg.Alerts.Rules)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"imported": map[string]int{
			"sources":     len(cfg.Sources),
			"outputs":     outputCount,
			"alert_rules": ruleCount,
		},
	})
}

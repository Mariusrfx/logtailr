package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Server) requireStore(w http.ResponseWriter) bool {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "database not configured (use --db-url)",
		})
		return false
	}
	return true
}

func parseUUID(raw string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(raw); err != nil {
		return id, fmt.Errorf("invalid UUID: %w", err)
	}
	return id, nil
}

const maxRequestBodySize = 1 << 20 // 1 MB

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	defer func() { _ = r.Body.Close() }()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// Validation allowlists
var validSourceTypes = map[string]bool{
	"file": true, "docker": true, "journalctl": true, "stdin": true, "kubernetes": true,
}

var validOutputTypes = map[string]bool{
	"opensearch": true, "webhook": true, "file": true,
}

var validAlertRuleTypes = map[string]bool{
	"pattern": true, "level": true, "error_rate": true, "health_change": true,
}

var validAlertSeverities = map[string]bool{
	"warning": true, "critical": true,
}

var validParsers = map[string]bool{
	"": true, "json": true, "logfmt": true, "text": true,
}

var validSettingKeys = map[string]bool{
	"global.level": true, "global.regex": true, "global.output": true,
	"global.output_path": true, "global.show_health": true, "global.aggregate": true,
	"global.aggregate_window": true, "alerts.default_cooldown": true,
	"alerts.notify.console": true, "alerts.notify.webhook.url": true,
}

const maxFieldLen = 1024

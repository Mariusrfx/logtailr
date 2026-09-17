package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"logtailr/internal/ssrf"
	"logtailr/internal/store"

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

// audit records a CRUD operation; a failed insert must never fail the request.
func (s *Server) audit(r *http.Request, action, resource, resourceID string) {
	if s.store == nil {
		return
	}
	if err := s.store.RecordAudit(r.Context(), action, resource, resourceID, requestIDFrom(r)); err != nil {
		apiLog(r).Warn("audit record failed", "action", action, "resource", resource, "error", err)
	}
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

func writeError(w http.ResponseWriter, r *http.Request, status int, msg string) {
	if r != nil {
		apiLog(r).Debug("api error response", "status", status, "path", r.URL.Path, "error", msg)
	}
	writeJSON(w, status, map[string]string{"error": msg, "request_id": requestIDFrom(r)})
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

// sensitiveKeys are JSON keys in output config that should be masked in API responses.
var sensitiveKeys = []string{"password", "token", "secret", "api_key", "apikey"}

// secretMask is the placeholder used by maskMapSecrets; updates carrying it
// keep the previously stored value.
const secretMask = "****"

// maskOutputSecrets returns a copy of the OutputRow with sensitive fields in Config masked.
func maskOutputSecrets(row *store.OutputRow) *store.OutputRow {
	if row == nil || len(row.Config) == 0 {
		return row
	}
	var cfgMap map[string]any
	if err := json.Unmarshal(row.Config, &cfgMap); err != nil {
		return row
	}

	maskMapSecrets(cfgMap)

	masked, err := json.Marshal(cfgMap)
	if err != nil {
		return row
	}

	cp := *row
	cp.Config = masked
	return &cp
}

func maskMapSecrets(m map[string]any) {
	for k, v := range m {
		lower := strings.ToLower(k)
		for _, sensitive := range sensitiveKeys {
			if strings.Contains(lower, sensitive) {
				if s, ok := v.(string); ok && s != "" {
					m[k] = secretMask
				}
				break
			}
		}
		if nested, ok := v.(map[string]any); ok {
			maskMapSecrets(nested)
		}
	}
}

// validateOutputConfigSSRF checks that URLs inside output configs don't target internal networks.
func validateOutputConfigSSRF(outputType string, cfgJSON json.RawMessage) error {
	if len(cfgJSON) == 0 {
		return nil
	}

	var cfgMap map[string]any
	if err := json.Unmarshal(cfgJSON, &cfgMap); err != nil {
		return nil // validation of JSON structure is handled elsewhere
	}

	switch outputType {
	case "webhook":
		if rawURL, ok := cfgMap["url"].(string); ok && rawURL != "" {
			if err := ssrf.ValidateExternalURL(rawURL); err != nil {
				return fmt.Errorf("webhook url: %w", err)
			}
		}
	case "opensearch":
		if hosts, ok := cfgMap["hosts"].([]any); ok {
			for _, h := range hosts {
				if hostStr, ok := h.(string); ok {
					if err := ssrf.ValidateExternalURL(hostStr); err != nil {
						return fmt.Errorf("opensearch host %q: %w", hostStr, err)
					}
				}
			}
		}
	}
	return nil
}

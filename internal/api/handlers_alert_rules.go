package api

import (
	"logtailr/internal/store"
	"net/http"
)

type alertRuleRequest struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Severity  string `json:"severity"`
	Pattern   string `json:"pattern,omitempty"`
	Level     string `json:"level,omitempty"`
	Source    string `json:"source,omitempty"`
	Threshold int    `json:"threshold,omitempty"`
	Window    string `json:"window,omitempty"`
	Cooldown  string `json:"cooldown,omitempty"`
	Enabled   *bool  `json:"enabled,omitempty"`
}

func (s *Server) handleListAlertRules(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	rows, err := s.store.ListAlertRules(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": rows, "total": len(rows)})
}

func (s *Server) handleGetAlertRule(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	row, err := s.store.GetAlertRuleByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (s *Server) handleCreateAlertRule(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	var req alertRuleRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name == "" || req.Type == "" || req.Severity == "" {
		writeError(w, http.StatusBadRequest, "name, type, and severity are required")
		return
	}
	if !validAlertRuleTypes[req.Type] {
		writeError(w, http.StatusBadRequest, "invalid alert rule type")
		return
	}
	if !validAlertSeverities[req.Severity] {
		writeError(w, http.StatusBadRequest, "invalid severity (must be warning or critical)")
		return
	}
	if len(req.Name) > maxFieldLen {
		writeError(w, http.StatusBadRequest, "field too long")
		return
	}
	row := alertRuleRequestToRow(&req)
	if err := s.store.CreateAlertRule(r.Context(), row); err != nil {
		writeError(w, http.StatusConflict, "alert rule already exists or invalid data")
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

func (s *Server) handleUpdateAlertRule(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req alertRuleRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	row := alertRuleRequestToRow(&req)
	row.ID = id
	if err := s.store.UpdateAlertRule(r.Context(), row); err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (s *Server) handleDeleteAlertRule(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.DeleteAlertRule(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func alertRuleRequestToRow(req *alertRuleRequest) *store.AlertRuleRow {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	return &store.AlertRuleRow{
		Name:      req.Name,
		Type:      req.Type,
		Severity:  req.Severity,
		Pattern:   req.Pattern,
		Level:     req.Level,
		Source:    req.Source,
		Threshold: req.Threshold,
		Window:    req.Window,
		Cooldown:  req.Cooldown,
		Enabled:   enabled,
	}
}

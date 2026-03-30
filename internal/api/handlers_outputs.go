package api

import (
	"encoding/json"
	"logtailr/internal/store"
	"net/http"
)

type outputRequest struct {
	Name    string          `json:"name"`
	Type    string          `json:"type"`
	Config  json.RawMessage `json:"config,omitempty"`
	Enabled *bool           `json:"enabled,omitempty"`
}

func (s *Server) handleListOutputs(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	rows, err := s.store.ListOutputs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	masked := make([]*store.OutputRow, len(rows))
	for i := range rows {
		masked[i] = maskOutputSecrets(&rows[i])
	}
	writeJSON(w, http.StatusOK, map[string]any{"outputs": masked, "total": len(masked)})
}

func (s *Server) handleGetOutput(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	row, err := s.store.GetOutputByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, maskOutputSecrets(row))
}

func (s *Server) handleCreateOutput(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	var req outputRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name == "" || req.Type == "" {
		writeError(w, http.StatusBadRequest, "name and type are required")
		return
	}
	if !validOutputTypes[req.Type] {
		writeError(w, http.StatusBadRequest, "invalid output type")
		return
	}
	if len(req.Name) > maxFieldLen {
		writeError(w, http.StatusBadRequest, "field too long")
		return
	}
	if !s.allowLocal {
		if err := validateOutputConfigSSRF(req.Type, req.Config); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	row := outputRequestToRow(&req)
	if err := s.store.CreateOutput(r.Context(), row); err != nil {
		writeError(w, http.StatusConflict, "output already exists or invalid data")
		return
	}
	writeJSON(w, http.StatusCreated, maskOutputSecrets(row))
}

func (s *Server) handleUpdateOutput(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req outputRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !s.allowLocal && req.Type != "" {
		if err := validateOutputConfigSSRF(req.Type, req.Config); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	row := outputRequestToRow(&req)
	row.ID = id
	if err := s.store.UpdateOutput(r.Context(), row); err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, maskOutputSecrets(row))
}

func (s *Server) handleDeleteOutput(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.DeleteOutput(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func outputRequestToRow(req *outputRequest) *store.OutputRow {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	cfg := []byte("{}")
	if req.Config != nil {
		cfg = req.Config
	}
	return &store.OutputRow{
		Name:    req.Name,
		Type:    req.Type,
		Config:  cfg,
		Enabled: enabled,
	}
}

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
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"outputs": rows, "total": len(rows)})
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
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (s *Server) handleCreateOutput(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	var req outputRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name == "" || req.Type == "" {
		writeError(w, http.StatusBadRequest, "name and type are required")
		return
	}
	row := outputRequestToRow(&req)
	if err := s.store.CreateOutput(r.Context(), row); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, row)
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
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	row := outputRequestToRow(&req)
	row.ID = id
	if err := s.store.UpdateOutput(r.Context(), row); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, row)
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
		writeError(w, http.StatusNotFound, err.Error())
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

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
		writeError(w, r, http.StatusInternalServerError, "internal error")
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
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	row, err := s.store.GetOutputByID(r.Context(), id)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not found")
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
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name == "" || req.Type == "" {
		writeError(w, r, http.StatusBadRequest, "name and type are required")
		return
	}
	if !validOutputTypes[req.Type] {
		writeError(w, r, http.StatusBadRequest, "invalid output type")
		return
	}
	if len(req.Name) > maxFieldLen {
		writeError(w, r, http.StatusBadRequest, "field too long")
		return
	}
	if !s.allowLocal {
		if err := validateOutputConfigSSRF(req.Type, req.Config); err != nil {
			writeError(w, r, http.StatusBadRequest, err.Error())
			return
		}
	}
	row := outputRequestToRow(&req)
	if err := s.store.CreateOutput(r.Context(), row); err != nil {
		writeError(w, r, http.StatusConflict, "output already exists or invalid data")
		return
	}
	s.audit(r, "create", "output", row.ID.String())
	writeJSON(w, http.StatusCreated, maskOutputSecrets(row))
}

func (s *Server) handleUpdateOutput(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	var req outputRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if !s.allowLocal && req.Type != "" {
		if err := validateOutputConfigSSRF(req.Type, req.Config); err != nil {
			writeError(w, r, http.StatusBadRequest, err.Error())
			return
		}
	}
	// Fetch the existing row so masked secrets echoed back by the client
	// (e.g. "****") don't overwrite the stored credentials.
	existing, err := s.store.GetOutputByID(r.Context(), id)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not found")
		return
	}
	row := outputRequestToRow(&req)
	row.ID = id
	row.Config = mergeMaskedSecrets(existing.Config, row.Config)
	if err := s.store.UpdateOutput(r.Context(), row); err != nil {
		writeError(w, r, http.StatusNotFound, "not found")
		return
	}
	s.audit(r, "update", "output", row.ID.String())
	writeJSON(w, http.StatusOK, maskOutputSecrets(row))
}

func (s *Server) handleDeleteOutput(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.DeleteOutput(r.Context(), id); err != nil {
		writeError(w, r, http.StatusNotFound, "not found")
		return
	}
	s.audit(r, "delete", "output", id.String())
	w.WriteHeader(http.StatusNoContent)
}

// mergeMaskedSecrets restores the original secret values where the incoming
// config carries the mask placeholder, so updating an output does not destroy
// stored credentials.
func mergeMaskedSecrets(existing, incoming []byte) []byte {
	if len(incoming) == 0 {
		return existing
	}
	if len(existing) == 0 {
		return incoming
	}
	var oldMap, newMap map[string]any
	if err := json.Unmarshal(existing, &oldMap); err != nil {
		return incoming
	}
	if err := json.Unmarshal(incoming, &newMap); err != nil {
		return incoming
	}
	restoreMaskedSecrets(newMap, oldMap)
	merged, err := json.Marshal(newMap)
	if err != nil {
		return incoming
	}
	return merged
}

func restoreMaskedSecrets(newMap, oldMap map[string]any) {
	for k, v := range newMap {
		if s, ok := v.(string); ok && s == secretMask {
			if old, exists := oldMap[k]; exists {
				newMap[k] = old
			}
			continue
		}
		if nested, ok := v.(map[string]any); ok {
			if oldNested, ok := oldMap[k].(map[string]any); ok {
				restoreMaskedSecrets(nested, oldNested)
			}
		}
	}
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

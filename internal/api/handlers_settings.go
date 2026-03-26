package api

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleGetSetting(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	key := r.PathValue("key")
	value, err := s.store.GetSetting(r.Context(), key)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if value == nil {
		writeError(w, http.StatusNotFound, "setting not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"key": key, "value": value})
}

func (s *Server) handleSetSetting(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	key := r.PathValue("key")

	var body struct {
		Value json.RawMessage `json:"value"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.Value == nil {
		writeError(w, http.StatusBadRequest, "value is required")
		return
	}
	if err := s.store.SetSetting(r.Context(), key, body.Value); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"key": key, "value": body.Value})
}

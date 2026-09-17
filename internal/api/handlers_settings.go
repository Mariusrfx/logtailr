package api

import (
	"encoding/json"
	"logtailr/internal/ssrf"
	"net/http"
)

func (s *Server) handleGetSetting(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	key := r.PathValue("key")
	if !validSettingKeys[key] {
		writeError(w, r, http.StatusBadRequest, "unknown setting key")
		return
	}
	value, err := s.store.GetSetting(r.Context(), key)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to read setting")
		return
	}
	if value == nil {
		writeError(w, r, http.StatusNotFound, "setting not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"key": key, "value": value})
}

func (s *Server) handleSetSetting(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	key := r.PathValue("key")
	if !validSettingKeys[key] {
		writeError(w, r, http.StatusBadRequest, "unknown setting key")
		return
	}

	var body struct {
		Value json.RawMessage `json:"value"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if body.Value == nil {
		writeError(w, r, http.StatusBadRequest, "value is required")
		return
	}
	if key == "alerts.notify.webhook.url" {
		var u string
		if err := json.Unmarshal(body.Value, &u); err != nil || u == "" {
			writeError(w, r, http.StatusBadRequest, "value must be a non-empty URL string")
			return
		}
		if !s.allowLocal {
			if err := ssrf.ValidateExternalURL(u); err != nil {
				writeError(w, r, http.StatusBadRequest, err.Error())
				return
			}
		}
	}
	if err := s.store.SetSetting(r.Context(), key, body.Value); err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to save setting")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"key": key, "value": body.Value})
}

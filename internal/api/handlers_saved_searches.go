package api

import (
	"encoding/json"
	"logtailr/internal/store"
	"net/http"
)

type savedSearchRequest struct {
	Name    string          `json:"name"`
	Filters json.RawMessage `json:"filters,omitempty"`
}

func (s *Server) handleListSavedSearches(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	rows, err := s.store.ListSavedSearches(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"saved_searches": rows, "total": len(rows)})
}

func (s *Server) handleGetSavedSearch(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	row, err := s.store.GetSavedSearchByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (s *Server) handleCreateSavedSearch(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	var req savedSearchRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	row := savedSearchRequestToRow(&req)
	if err := s.store.CreateSavedSearch(r.Context(), row); err != nil {
		writeError(w, http.StatusConflict, "already exists")
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

func (s *Server) handleUpdateSavedSearch(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req savedSearchRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	row := savedSearchRequestToRow(&req)
	row.ID = id
	if err := s.store.UpdateSavedSearch(r.Context(), row); err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (s *Server) handleDeleteSavedSearch(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.DeleteSavedSearch(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func savedSearchRequestToRow(req *savedSearchRequest) *store.SavedSearchRow {
	filters := []byte("{}")
	if req.Filters != nil {
		filters = req.Filters
	}
	return &store.SavedSearchRow{
		Name:    req.Name,
		Filters: filters,
	}
}

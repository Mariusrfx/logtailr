package api

import (
	"logtailr/internal/store"
	"net/http"
)

type sourceRequest struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Path          string `json:"path,omitempty"`
	Container     string `json:"container,omitempty"`
	Unit          string `json:"unit,omitempty"`
	Priority      string `json:"priority,omitempty"`
	OutputFormat  string `json:"output_format,omitempty"`
	Namespace     string `json:"namespace,omitempty"`
	Pod           string `json:"pod,omitempty"`
	LabelSelector string `json:"label_selector,omitempty"`
	Kubeconfig    string `json:"kubeconfig,omitempty"`
	Follow        *bool  `json:"follow,omitempty"`
	Parser        string `json:"parser,omitempty"`
}

func (s *Server) handleListSources(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	rows, err := s.store.ListSources(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": rows, "total": len(rows)})
}

func (s *Server) handleGetSource(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	row, err := s.store.GetSourceByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (s *Server) handleCreateSource(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	var req sourceRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name == "" || req.Type == "" {
		writeError(w, http.StatusBadRequest, "name and type are required")
		return
	}
	if !validSourceTypes[req.Type] {
		writeError(w, http.StatusBadRequest, "invalid source type")
		return
	}
	if !validParsers[req.Parser] {
		writeError(w, http.StatusBadRequest, "invalid parser")
		return
	}
	if len(req.Name) > maxFieldLen || len(req.Path) > maxFieldLen {
		writeError(w, http.StatusBadRequest, "field too long")
		return
	}
	row := sourceRequestToRow(&req)
	if err := s.store.CreateSource(r.Context(), row); err != nil {
		writeError(w, http.StatusConflict, "source already exists or invalid data")
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

func (s *Server) handleUpdateSource(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req sourceRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	row := sourceRequestToRow(&req)
	row.ID = id
	if err := s.store.UpdateSource(r.Context(), row); err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (s *Server) handleDeleteSource(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.DeleteSource(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func sourceRequestToRow(req *sourceRequest) *store.SourceRow {
	follow := true
	if req.Follow != nil {
		follow = *req.Follow
	}
	return &store.SourceRow{
		Name:          req.Name,
		Type:          req.Type,
		Path:          req.Path,
		Container:     req.Container,
		Unit:          req.Unit,
		Priority:      req.Priority,
		OutputFormat:  req.OutputFormat,
		Namespace:     req.Namespace,
		Pod:           req.Pod,
		LabelSelector: req.LabelSelector,
		Kubeconfig:    req.Kubeconfig,
		Follow:        follow,
		Parser:        req.Parser,
	}
}

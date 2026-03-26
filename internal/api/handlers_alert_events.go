package api

import (
	"logtailr/internal/store"
	"net/http"
	"strconv"
	"time"
)

func (s *Server) handleListAlertEvents(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}

	q := r.URL.Query()
	f := store.AlertEventFilter{
		Severity: q.Get("severity"),
		RuleName: q.Get("rule"),
		Source:   q.Get("source"),
	}

	if v := q.Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid 'from' timestamp")
			return
		}
		f.From = &t
	}
	if v := q.Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid 'to' timestamp")
			return
		}
		f.To = &t
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "invalid 'limit'")
			return
		}
		if n > 1000 {
			n = 1000
		}
		f.Limit = n
	}
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "invalid 'offset'")
			return
		}
		f.Offset = n
	}

	rows, err := s.store.ListAlertEvents(r.Context(), f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": rows, "total": len(rows)})
}

func (s *Server) handleAckAlertEvent(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.AcknowledgeAlertEvent(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "acknowledged"})
}

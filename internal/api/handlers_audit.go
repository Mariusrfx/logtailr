package api

import (
	"net/http"
	"strconv"
)

func (s *Server) handleListAudit(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	limit := 100
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(w, r, http.StatusBadRequest, "invalid 'limit'")
			return
		}
		limit = n
	}
	if limit > 1000 {
		limit = 1000
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, r, http.StatusBadRequest, "invalid 'offset'")
			return
		}
		offset = n
	}
	rows, err := s.store.ListAudit(r.Context(), limit, offset)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"audit": rows, "total": len(rows)})
}

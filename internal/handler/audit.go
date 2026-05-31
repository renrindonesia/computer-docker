package handler

import (
	"net/http"
	"strconv"
)

// auditList returns the most recent audited actions. ?limit= caps the count.
func (h *Handler) auditList(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	writeJSON(w, http.StatusOK, map[string]any{
		"entries": h.audit.Recent(limit),
	})
}

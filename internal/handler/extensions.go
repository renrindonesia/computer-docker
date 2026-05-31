package handler

import (
	"net/http"

	"computer-use/internal/procapi"
)

func (h *Handler) extList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"extensions": h.ext.Statuses(r.Context()),
	})
}

// extInstall starts the extension's install command as a background process
// and returns its process handle so the caller can poll /procs/{id}/logs.
func (h *Handler) extInstall(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	e, ok := h.ext.Get(name)
	if !ok {
		writeErr(w, http.StatusNotFound, "unknown extension")
		return
	}
	if len(e.Install) == 0 {
		writeErr(w, http.StatusBadRequest, "extension has no install command")
		return
	}
	p, err := h.procs.Start(procapi.StartRequest{
		Command: e.Install[0],
		Args:    e.Install[1:],
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"extension":  name,
		"installing": true,
		"process":    p,
		"hint":       "poll /api/v1/procs/" + p.ID + "/logs",
	})
}

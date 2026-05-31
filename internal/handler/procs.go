package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"computer-use/internal/procapi"
)

func (h *Handler) procStart(w http.ResponseWriter, r *http.Request) {
	var req procapi.StartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	p, err := h.procs.Start(req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (h *Handler) procList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"processes": h.procs.List()})
}

func (h *Handler) procGet(w http.ResponseWriter, r *http.Request) {
	p, err := h.procs.Get(r.PathValue("id"))
	if err != nil {
		h.procErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *Handler) procLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := h.procs.Logs(r.PathValue("id"))
	if err != nil {
		h.procErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"logs": logs})
}

func (h *Handler) procStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.procs.Stop(id); err != nil {
		h.procErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "stopped": true})
}

func (h *Handler) procDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.procs.Remove(id); err != nil {
		h.procErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "removed": true})
}

func (h *Handler) procErr(w http.ResponseWriter, err error) {
	if errors.Is(err, procapi.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "process not found")
		return
	}
	writeErr(w, http.StatusInternalServerError, err.Error())
}

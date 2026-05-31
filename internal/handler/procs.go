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
	h.audit.Record("proc_start", req.Command, r.RemoteAddr, map[string]any{"id": p.ID, "args": req.Args})
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
	id := r.PathValue("id")
	if r.URL.Query().Get("follow") != "" {
		h.procLogsStream(w, r, id)
		return
	}
	logs, err := h.procs.Logs(id)
	if err != nil {
		h.procErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"logs": logs})
}

// procLogsStream streams log lines as Server-Sent Events until the client
// disconnects or the process exits.
func (h *Handler) procLogsStream(w http.ResponseWriter, r *http.Request, id string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	snapshot, ch, cancel, err := h.procs.Subscribe(id)
	if err != nil {
		h.procErr(w, err)
		return
	}
	defer cancel()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	send := func(event string, v any) {
		b, _ := json.Marshal(v)
		_, _ = w.Write([]byte("event: " + event + "\ndata: "))
		_, _ = w.Write(b)
		_, _ = w.Write([]byte("\n\n"))
		flusher.Flush()
	}

	for _, ln := range snapshot {
		send("log", ln)
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case ln, open := <-ch:
			if !open {
				send("end", map[string]any{"id": id})
				return
			}
			send("log", ln)
		}
	}
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

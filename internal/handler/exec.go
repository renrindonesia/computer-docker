package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"computer-use/internal/execapi"
)

func (h *Handler) execRun(w http.ResponseWriter, r *http.Request) {
	var req execapi.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	h.audit.Record("exec", req.Command, r.RemoteAddr, map[string]any{"args": req.Args})
	res, err := h.exec.Run(r.Context(), req)
	if err != nil {
		if errors.Is(err, execapi.ErrNoCommand) {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

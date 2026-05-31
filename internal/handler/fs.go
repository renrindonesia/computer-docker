package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"computer-use/internal/fsapi"
)

func (h *Handler) fsErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, fsapi.ErrEscape):
		writeErr(w, http.StatusForbidden, err.Error())
	case errors.Is(err, os.ErrNotExist):
		writeErr(w, http.StatusNotFound, "not found")
	case errors.Is(err, os.ErrPermission):
		writeErr(w, http.StatusForbidden, "permission denied")
	default:
		writeErr(w, http.StatusInternalServerError, err.Error())
	}
}

func (h *Handler) fsList(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	entries, err := h.fs.List(path)
	if err != nil {
		h.fsErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": path, "entries": entries})
}

func (h *Handler) fsStat(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	entry, err := h.fs.Stat(path)
	if err != nil {
		h.fsErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

func (h *Handler) fsRead(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	data, err := h.fs.Read(path)
	if err != nil {
		h.fsErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": path, "content": string(data)})
}

type writeReq struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (h *Handler) fsWrite(w http.ResponseWriter, r *http.Request) {
	var req writeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Path == "" {
		writeErr(w, http.StatusBadRequest, "path required")
		return
	}
	if err := h.fs.Write(req.Path, []byte(req.Content)); err != nil {
		h.fsErr(w, err)
		return
	}
	h.audit.Record("fs_write", req.Path, r.RemoteAddr, map[string]any{"bytes": len(req.Content)})
	writeJSON(w, http.StatusOK, map[string]any{"path": req.Path, "bytes": len(req.Content)})
}

type pathReq struct {
	Path string `json:"path"`
}

func (h *Handler) fsMkdir(w http.ResponseWriter, r *http.Request) {
	var req pathReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Path == "" {
		writeErr(w, http.StatusBadRequest, "path required")
		return
	}
	if err := h.fs.Mkdir(req.Path); err != nil {
		h.fsErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": req.Path, "created": true})
}

func (h *Handler) fsDelete(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeErr(w, http.StatusBadRequest, "path required")
		return
	}
	if err := h.fs.Delete(path); err != nil {
		h.fsErr(w, err)
		return
	}
	h.audit.Record("fs_delete", path, r.RemoteAddr, nil)
	writeJSON(w, http.StatusOK, map[string]any{"path": path, "deleted": true})
}

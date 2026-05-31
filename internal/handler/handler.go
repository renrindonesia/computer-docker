// Package handler wires HTTP routes to the fs and exec services.
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"computer-use/internal/audit"
	"computer-use/internal/execapi"
	"computer-use/internal/extapi"
	"computer-use/internal/fsapi"
	"computer-use/internal/procapi"
)

// Handler holds dependencies for the API handlers.
type Handler struct {
	fs     *fsapi.Service
	exec   *execapi.Service
	procs  *procapi.Manager
	ext    *extapi.Manager
	audit  *audit.Recorder
	logger *slog.Logger
}

// New creates a Handler.
func New(fs *fsapi.Service, exec *execapi.Service, procs *procapi.Manager, ext *extapi.Manager, aud *audit.Recorder, logger *slog.Logger) *Handler {
	return &Handler{fs: fs, exec: exec, procs: procs, ext: ext, audit: aud, logger: logger}
}

// Routes registers all endpoints on mux.
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /api/v1/info", h.info)
	mux.HandleFunc("GET /api/v1/audit", h.auditList)

	// extensions
	mux.HandleFunc("GET /api/v1/extensions", h.extList)
	mux.HandleFunc("POST /api/v1/extensions/{name}/install", h.extInstall)

	// filesystem
	mux.HandleFunc("GET /api/v1/fs/list", h.fsList)
	mux.HandleFunc("GET /api/v1/fs/stat", h.fsStat)
	mux.HandleFunc("GET /api/v1/fs/read", h.fsRead)
	mux.HandleFunc("POST /api/v1/fs/write", h.fsWrite)
	mux.HandleFunc("POST /api/v1/fs/mkdir", h.fsMkdir)
	mux.HandleFunc("DELETE /api/v1/fs/delete", h.fsDelete)
	mux.HandleFunc("POST /api/v1/fs/move", h.fsMove)
	mux.HandleFunc("POST /api/v1/fs/copy", h.fsCopy)
	mux.HandleFunc("POST /api/v1/fs/chmod", h.fsChmod)
	mux.HandleFunc("POST /api/v1/fs/patch", h.fsPatch)
	mux.HandleFunc("GET /api/v1/fs/search", h.fsSearch)
	mux.HandleFunc("POST /api/v1/fs/upload", h.fsUpload)
	mux.HandleFunc("GET /api/v1/fs/download", h.fsDownload)

	// exec (synchronous)
	mux.HandleFunc("POST /api/v1/exec", h.execRun)

	// processes (background)
	mux.HandleFunc("POST /api/v1/procs", h.procStart)
	mux.HandleFunc("GET /api/v1/procs", h.procList)
	mux.HandleFunc("GET /api/v1/procs/{id}", h.procGet)
	mux.HandleFunc("GET /api/v1/procs/{id}/logs", h.procLogs)
	mux.HandleFunc("POST /api/v1/procs/{id}/stop", h.procStop)
	mux.HandleFunc("DELETE /api/v1/procs/{id}", h.procDelete)
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

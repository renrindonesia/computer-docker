package handler

import (
	"net/http"
	"os"
	"runtime"
)

// info aggregates a one-shot view of the sandbox: system facts, running
// processes, extension status, and the root-level file listing.
func (h *Handler) info(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()

	rootFiles, err := h.fs.List("/")
	if err != nil {
		rootFiles = nil
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"system": map[string]any{
			"hostname":   hostname,
			"os":         runtime.GOOS,
			"arch":       runtime.GOARCH,
			"num_cpu":    runtime.NumCPU(),
			"go_version": runtime.Version(),
		},
		"fs_root":    h.fs.Root,
		"processes":  h.procs.List(),
		"extensions": h.ext.Statuses(r.Context()),
		"root_files": rootFiles,
	})
}

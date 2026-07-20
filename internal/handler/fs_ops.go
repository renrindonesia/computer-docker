package handler

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"computer-use/internal/fsapi"
)

type moveReq struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func (h *Handler) fsMove(w http.ResponseWriter, r *http.Request) {
	var req moveReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.From == "" || req.To == "" {
		writeErr(w, http.StatusBadRequest, "from and to required")
		return
	}
	if err := h.fs.Move(req.From, req.To); err != nil {
		h.fsErr(w, err)
		return
	}
	h.audit.Record("fs_move", req.To, r.RemoteAddr, map[string]any{"from": req.From})
	writeJSON(w, http.StatusOK, map[string]any{"from": req.From, "to": req.To, "moved": true})
}

func (h *Handler) fsCopy(w http.ResponseWriter, r *http.Request) {
	var req moveReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.From == "" || req.To == "" {
		writeErr(w, http.StatusBadRequest, "from and to required")
		return
	}
	if err := h.fs.Copy(req.From, req.To); err != nil {
		h.fsErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"from": req.From, "to": req.To, "copied": true})
}

type chmodReq struct {
	Path string `json:"path"`
	Mode string `json:"mode"` // octal, e.g. "0755"
}

func (h *Handler) fsChmod(w http.ResponseWriter, r *http.Request) {
	var req chmodReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		writeErr(w, http.StatusBadRequest, "path and mode required")
		return
	}
	m, err := strconv.ParseUint(req.Mode, 8, 32)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "mode must be octal e.g. 0755")
		return
	}
	if err := h.fs.Chmod(req.Path, os.FileMode(m)); err != nil {
		h.fsErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": req.Path, "mode": req.Mode})
}

type patchReq struct {
	Path string `json:"path"`
	Old  string `json:"old"`
	New  string `json:"new"`
}

// fsPatch applies an apply_patch-style edit: replace the unique `old` block
// with `new`. Empty `old` creates/overwrites the file.
func (h *Handler) fsPatch(w http.ResponseWriter, r *http.Request) {
	var req patchReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		writeErr(w, http.StatusBadRequest, "path required")
		return
	}
	err := h.fs.Patch(req.Path, req.Old, req.New)
	switch {
	case err == nil:
		h.audit.Record("fs_edit", req.Path, r.RemoteAddr, nil)
		writeJSON(w, http.StatusOK, map[string]any{"path": req.Path, "patched": true})
	case errors.Is(err, fsapi.ErrPatchNoMatch):
		writeErr(w, http.StatusConflict, "old block not found")
	case errors.Is(err, fsapi.ErrPatchAmbiguous):
		writeErr(w, http.StatusConflict, "old block matches multiple locations; add more context")
	default:
		h.fsErr(w, err)
	}
}

func (h *Handler) fsSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	path := q.Get("path")
	limit, _ := strconv.Atoi(q.Get("limit"))
	hits, err := h.fs.Search(path, q.Get("glob"), q.Get("content"), limit)
	if err != nil {
		h.fsErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(hits), "hits": hits})
}

// fsUpload accepts multipart form file uploads. Target path = ?path= dir,
// each file written under it by its form filename.
func (h *Handler) fsUpload(w http.ResponseWriter, r *http.Request) {
	dir := r.URL.Query().Get("path")
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		writeErr(w, http.StatusBadRequest, "no file field")
		return
	}
	written := []string{}
	for _, fh := range files {
		src, err := fh.Open()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		data, err := io.ReadAll(src)
		src.Close()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		dst := filepath.ToSlash(filepath.Join(dir, fh.Filename))
		if err := h.fs.Write(dst, data); err != nil {
			h.fsErr(w, err)
			return
		}
		written = append(written, "/"+dst)
	}
	writeJSON(w, http.StatusOK, map[string]any{"written": written})
}

// fsDownload streams raw file bytes (binary-safe).
func (h *Handler) fsDownload(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeErr(w, http.StatusBadRequest, "path required")
		return
	}
	abs, err := h.fs.AbsFor(path)
	if err != nil {
		h.fsErr(w, err)
		return
	}
	f, err := os.Open(abs)
	if err != nil {
		h.fsErr(w, err)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		writeErr(w, http.StatusBadRequest, "not a file")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(abs)+"\"")
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	_, _ = io.Copy(w, f)
}

// fsView serves a file inline so browsers render it in place — images show as
// images, video/audio stream in a player — instead of downloading. It sets the
// MIME type from the file extension and delegates to http.ServeContent, which
// handles Range requests (needed for video/audio seeking) and conditional
// caching headers.
func (h *Handler) fsView(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeErr(w, http.StatusBadRequest, "path required")
		return
	}
	abs, err := h.fs.AbsFor(path)
	if err != nil {
		h.fsErr(w, err)
		return
	}
	f, err := os.Open(abs)
	if err != nil {
		h.fsErr(w, err)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		writeErr(w, http.StatusBadRequest, "not a file")
		return
	}

	// Files under FS_ROOT are attacker-controllable (agents write there), and
	// this endpoint serves on the app's own origin. Rendering e.g. an .html or
	// .svg inline would execute its script in that origin — stored XSS that
	// could read the API key or drive /vnc/. So only render a fixed allowlist of
	// non-executable media inline; force everything else to download as opaque
	// bytes. Defense in depth: never sniff the type, and sandbox the response.
	safeName := strings.ReplaceAll(filepath.Base(abs), `"`, "")
	ct := mime.TypeByExtension(filepath.Ext(abs))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; media-src 'self'; sandbox")
	if inlineSafeMIME[ct] {
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Content-Disposition", "inline; filename=\""+safeName+"\"")
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+safeName+"\"")
	}
	http.ServeContent(w, r, safeName, info.ModTime(), f)
}

// inlineSafeMIME lists content types safe to render inline on the app origin —
// images, video and audio, which browsers display but do not execute as script.
// Anything not here (html, svg, js, pdf, unknown) is served as a download.
var inlineSafeMIME = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
	"image/webp": true,
	"image/avif": true,
	"image/bmp":  true,
	"image/tiff": true,
	"video/mp4":  true,
	"video/webm": true,
	"video/ogg":  true,
	"audio/mpeg": true,
	"audio/ogg":  true,
	"audio/wav":  true,
	"audio/webm": true,
	"audio/aac":  true,
	"audio/flac": true,
}

package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"computer-use/internal/audit"
	"computer-use/internal/config"
	"computer-use/internal/docs"
	"computer-use/internal/execapi"
	"computer-use/internal/fsapi"
	"computer-use/internal/handler"
	"computer-use/internal/mcpserver"
	"computer-use/internal/middleware"
	"computer-use/internal/procapi"
	"computer-use/internal/vnc"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg := config.Load(".env")

	fsSvc, err := fsapi.New(cfg.FSRoot)
	if err != nil {
		logger.Error("fs init failed", "err", err)
		os.Exit(1)
	}
	execSvc := execapi.New(fsSvc.Root, cfg.ExecTimeout, cfg.ExecMaxTimeout)
	procMgr := procapi.NewManager(fsSvc.Root)

	auditRec, err := audit.New(1000, cfg.AuditLog)
	if err != nil {
		logger.Error("audit init failed", "err", err)
		os.Exit(1)
	}
	defer auditRec.Close()

	h := handler.New(fsSvc, execSvc, procMgr, auditRec, logger)

	mux := http.NewServeMux()
	h.Routes(mux)
	docs.Register(mux)

	// MCP over Streamable HTTP — same services, same auth, mounted at /mcp.
	mcpSrv := mcpserver.New(fsSvc, execSvc, procMgr, auditRec)
	mux.Handle("/mcp", mcpSrv.Handler())
	mux.Handle("/mcp/", mcpSrv.Handler())

	// Live desktop: reverse-proxy the noVNC viewer (websockify on a loopback
	// port) under /vnc/. Protected at the VNC layer, not the API key — noVNC's
	// relative asset/websocket URLs can't carry ?key=, so /vnc/ is allow-listed.
	if cfg.VNCUpstream != "" {
		vncProxy, err := vnc.Handler(cfg.VNCUpstream, cfg.VNCFrameAncestors)
		if err != nil {
			logger.Error("vnc proxy init failed", "err", err)
			os.Exit(1)
		}
		mux.Handle("/vnc/", vncProxy)
	}

	// /healthz, /docs and /openapi.json are public; everything else needs the key.
	auth := middleware.APIKey(cfg.APIKey, "/healthz", "/docs", "/openapi.json", "/vnc/")
	root := middleware.Chain(mux,
		middleware.Recover(logger),
		middleware.Logging(logger),
		auth,
	)

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      root,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // exec may stream long-running output; rely on exec timeout
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("server starting", "addr", cfg.Addr, "fs_root", fsSvc.Root, "auth", cfg.APIKey != "")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown failed", "err", err)
	}
}

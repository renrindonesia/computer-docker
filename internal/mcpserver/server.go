// Package mcpserver exposes the sandbox (filesystem, exec, processes,
// extensions) to agents as Model Context Protocol tools over Streamable HTTP.
package mcpserver

import (
	"errors"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"computer-use/internal/execapi"
	"computer-use/internal/extapi"
	"computer-use/internal/fsapi"
	"computer-use/internal/procapi"
)

var errUnknownExt = errors.New("unknown extension")

// Server bundles the sandbox services exposed as MCP tools.
type Server struct {
	fs   *fsapi.Service
	exec *execapi.Service
	proc *procapi.Manager
	ext  *extapi.Manager
}

// New creates an MCP Server over the given services.
func New(fs *fsapi.Service, exec *execapi.Service, proc *procapi.Manager, ext *extapi.Manager) *Server {
	return &Server{fs: fs, exec: exec, proc: proc, ext: ext}
}

// mcpServer builds the underlying *mcp.Server with all tools registered.
func (s *Server) mcpServer() *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "computer-docker",
		Version: "0.1.0",
	}, nil)

	// Filesystem
	mcp.AddTool(srv, &mcp.Tool{Name: "fs_list", Description: "List entries of a directory under the sandbox root."}, s.fsList)
	mcp.AddTool(srv, &mcp.Tool{Name: "fs_read", Description: "Read a text file's contents."}, s.fsRead)
	mcp.AddTool(srv, &mcp.Tool{Name: "fs_write", Description: "Create or overwrite a text file (parent dirs auto-created)."}, s.fsWrite)
	mcp.AddTool(srv, &mcp.Tool{Name: "fs_edit", Description: "Replace the unique occurrence of `old` with `new` in a file (apply_patch-style). Empty `old` creates/overwrites."}, s.fsEdit)
	mcp.AddTool(srv, &mcp.Tool{Name: "fs_search", Description: "Search files by name glob and/or content substring, returning matches with line numbers."}, s.fsSearch)
	mcp.AddTool(srv, &mcp.Tool{Name: "fs_move", Description: "Move or rename a file or directory."}, s.fsMove)
	mcp.AddTool(srv, &mcp.Tool{Name: "fs_remove", Description: "Delete a file or directory tree."}, s.fsRemove)

	// Exec
	mcp.AddTool(srv, &mcp.Tool{Name: "exec", Description: "Run a command synchronously and capture stdout, stderr and exit code. Timeout-bounded; use proc_start for long-running commands."}, s.execRun)

	// Processes
	mcp.AddTool(srv, &mcp.Tool{Name: "proc_start", Description: "Start a long-running command in the background. Returns a process id to poll."}, s.procStart)
	mcp.AddTool(srv, &mcp.Tool{Name: "proc_list", Description: "List all background processes and their state."}, s.procList)
	mcp.AddTool(srv, &mcp.Tool{Name: "proc_logs", Description: "Get the buffered stdout/stderr log lines of a background process."}, s.procLogs)
	mcp.AddTool(srv, &mcp.Tool{Name: "proc_stop", Description: "Send SIGTERM to a background process group."}, s.procStop)

	// Extensions + info
	mcp.AddTool(srv, &mcp.Tool{Name: "ext_list", Description: "List installable extensions (e.g. browser-use) and whether each is installed."}, s.extList)
	mcp.AddTool(srv, &mcp.Tool{Name: "ext_install", Description: "Install an extension; runs in the background and returns a process id to poll."}, s.extInstall)
	mcp.AddTool(srv, &mcp.Tool{Name: "info", Description: "Snapshot of the machine: system facts, running processes, extensions, and root files."}, s.info)

	return srv
}

// Handler returns an http.Handler serving MCP over Streamable HTTP. Mount it at
// /mcp behind the same auth as the REST API.
func (s *Server) Handler() http.Handler {
	srv := s.mcpServer()
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return srv
	}, nil)
}

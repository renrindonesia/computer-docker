package mcpserver

import (
	"context"
	"os"
	"runtime"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"computer-use/internal/execapi"
	"computer-use/internal/fsapi"
	"computer-use/internal/procapi"
)

// ---- Filesystem ----

type pathIn struct {
	Path string `json:"path" jsonschema:"path relative to the sandbox root"`
}

type fsListOut struct {
	Entries []fsapi.Entry `json:"entries"`
}

func (s *Server) fsList(_ context.Context, _ *mcp.CallToolRequest, in pathIn) (*mcp.CallToolResult, fsListOut, error) {
	entries, err := s.fs.List(in.Path)
	if err != nil {
		return nil, fsListOut{}, err
	}
	return nil, fsListOut{Entries: entries}, nil
}

type fsReadOut struct {
	Content string `json:"content"`
}

func (s *Server) fsRead(_ context.Context, _ *mcp.CallToolRequest, in pathIn) (*mcp.CallToolResult, fsReadOut, error) {
	data, err := s.fs.Read(in.Path)
	if err != nil {
		return nil, fsReadOut{}, err
	}
	return nil, fsReadOut{Content: string(data)}, nil
}

type fsWriteIn struct {
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
}

type okPathOut struct {
	Path string `json:"path"`
	OK   bool   `json:"ok"`
}

func (s *Server) fsWrite(_ context.Context, _ *mcp.CallToolRequest, in fsWriteIn) (*mcp.CallToolResult, okPathOut, error) {
	if err := s.fs.Write(in.Path, []byte(in.Content)); err != nil {
		return nil, okPathOut{}, err
	}
	s.audit.Record("fs_write", in.Path, "mcp", map[string]any{"bytes": len(in.Content)})
	return nil, okPathOut{Path: in.Path, OK: true}, nil
}

type fsEditIn struct {
	Path string `json:"path"`
	Old  string `json:"old,omitempty" jsonschema:"unique block to replace; empty creates/overwrites the file"`
	New  string `json:"new,omitempty"`
}

func (s *Server) fsEdit(_ context.Context, _ *mcp.CallToolRequest, in fsEditIn) (*mcp.CallToolResult, okPathOut, error) {
	if err := s.fs.Patch(in.Path, in.Old, in.New); err != nil {
		return nil, okPathOut{}, err
	}
	s.audit.Record("fs_edit", in.Path, "mcp", nil)
	return nil, okPathOut{Path: in.Path, OK: true}, nil
}

type fsSearchIn struct {
	Path    string `json:"path,omitempty"`
	Glob    string `json:"glob,omitempty" jsonschema:"filename glob, e.g. *.go (optional)"`
	Content string `json:"content,omitempty" jsonschema:"substring to grep for (optional)"`
	Limit   int    `json:"limit,omitempty" jsonschema:"max hits, default 200"`
}

type fsSearchOut struct {
	Count int               `json:"count"`
	Hits  []fsapi.SearchHit `json:"hits"`
}

func (s *Server) fsSearch(_ context.Context, _ *mcp.CallToolRequest, in fsSearchIn) (*mcp.CallToolResult, fsSearchOut, error) {
	hits, err := s.fs.Search(in.Path, in.Glob, in.Content, in.Limit)
	if err != nil {
		return nil, fsSearchOut{}, err
	}
	return nil, fsSearchOut{Count: len(hits), Hits: hits}, nil
}

type fromToIn struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func (s *Server) fsMove(_ context.Context, _ *mcp.CallToolRequest, in fromToIn) (*mcp.CallToolResult, okPathOut, error) {
	if err := s.fs.Move(in.From, in.To); err != nil {
		return nil, okPathOut{}, err
	}
	s.audit.Record("fs_move", in.To, "mcp", map[string]any{"from": in.From})
	return nil, okPathOut{Path: in.To, OK: true}, nil
}

func (s *Server) fsRemove(_ context.Context, _ *mcp.CallToolRequest, in pathIn) (*mcp.CallToolResult, okPathOut, error) {
	if err := s.fs.Delete(in.Path); err != nil {
		return nil, okPathOut{}, err
	}
	s.audit.Record("fs_delete", in.Path, "mcp", nil)
	return nil, okPathOut{Path: in.Path, OK: true}, nil
}

// ---- Exec ----

type execIn struct {
	Command    string   `json:"command"`
	Args       []string `json:"args,omitempty"`
	Dir        string   `json:"dir,omitempty"`
	Stdin      string   `json:"stdin,omitempty"`
	TimeoutSec int      `json:"timeout_sec,omitempty"`
}

func (s *Server) execRun(ctx context.Context, _ *mcp.CallToolRequest, in execIn) (*mcp.CallToolResult, execapi.Result, error) {
	res, err := s.exec.Run(ctx, execapi.Request{
		Command: in.Command,
		Args:    in.Args,
		Dir:     in.Dir,
		Stdin:   in.Stdin,
		Timeout: in.TimeoutSec,
	})
	if err != nil {
		return nil, execapi.Result{}, err
	}
	s.audit.Record("exec", in.Command, "mcp", map[string]any{"args": in.Args})
	return nil, res, nil
}

// ---- Processes ----

type procStartIn struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Dir     string            `json:"dir,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

func (s *Server) procStart(_ context.Context, _ *mcp.CallToolRequest, in procStartIn) (*mcp.CallToolResult, procapi.ProcView, error) {
	p, err := s.proc.Start(procapi.StartRequest{
		Command: in.Command,
		Args:    in.Args,
		Dir:     in.Dir,
		Env:     in.Env,
	})
	if err != nil {
		return nil, procapi.ProcView{}, err
	}
	s.audit.Record("proc_start", in.Command, "mcp", map[string]any{"id": p.ID, "args": in.Args})
	return nil, p, nil
}

type emptyIn struct{}

type procListOut struct {
	Processes []procapi.ProcView `json:"processes"`
}

func (s *Server) procList(_ context.Context, _ *mcp.CallToolRequest, _ emptyIn) (*mcp.CallToolResult, procListOut, error) {
	return nil, procListOut{Processes: s.proc.List()}, nil
}

type idIn struct {
	ID string `json:"id"`
}

type procLogsOut struct {
	Logs []procapi.LogLine `json:"logs"`
}

func (s *Server) procLogs(_ context.Context, _ *mcp.CallToolRequest, in idIn) (*mcp.CallToolResult, procLogsOut, error) {
	logs, err := s.proc.Logs(in.ID)
	if err != nil {
		return nil, procLogsOut{}, err
	}
	return nil, procLogsOut{Logs: logs}, nil
}

type okIDOut struct {
	ID string `json:"id"`
	OK bool   `json:"ok"`
}

func (s *Server) procStop(_ context.Context, _ *mcp.CallToolRequest, in idIn) (*mcp.CallToolResult, okIDOut, error) {
	if err := s.proc.Stop(in.ID); err != nil {
		return nil, okIDOut{}, err
	}
	return nil, okIDOut{ID: in.ID, OK: true}, nil
}

// ---- Info ----

type infoOut struct {
	System    map[string]any     `json:"system"`
	FSRoot    string             `json:"fs_root"`
	Processes []procapi.ProcView `json:"processes"`
	RootFiles []fsapi.Entry      `json:"root_files"`
}

func (s *Server) info(_ context.Context, _ *mcp.CallToolRequest, _ emptyIn) (*mcp.CallToolResult, infoOut, error) {
	hostname, _ := os.Hostname()
	rootFiles, _ := s.fs.List("/")
	return nil, infoOut{
		System: map[string]any{
			"hostname":   hostname,
			"os":         runtime.GOOS,
			"arch":       runtime.GOARCH,
			"num_cpu":    strconv.Itoa(runtime.NumCPU()),
			"go_version": runtime.Version(),
		},
		FSRoot:    s.fs.Root,
		Processes: s.proc.List(),
		RootFiles: rootFiles,
	}, nil
}

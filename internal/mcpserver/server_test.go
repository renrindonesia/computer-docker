package mcpserver

import (
	"context"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"computer-use/internal/audit"
	"computer-use/internal/execapi"
	"computer-use/internal/extapi"
	"computer-use/internal/fsapi"
	"computer-use/internal/procapi"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	fs, err := fsapi.New(t.TempDir())
	if err != nil {
		t.Fatalf("fsapi.New: %v", err)
	}
	aud, _ := audit.New(100, "")
	return New(
		fs,
		execapi.New(fs.Root, 5*time.Second, 10*time.Second),
		procapi.NewManager(fs.Root),
		extapi.NewManager(),
		aud,
	)
}

// connect wires an in-memory client to the MCP server.
func connect(t *testing.T, s *Server) *mcp.ClientSession {
	t.Helper()
	ct, st := mcp.NewInMemoryTransports()
	if _, err := s.mcpServer().Connect(context.Background(), st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func TestListToolsCount(t *testing.T) {
	cs := connect(t, newTestServer(t))
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(res.Tools) != 15 {
		names := make([]string, 0, len(res.Tools))
		for _, tl := range res.Tools {
			names = append(names, tl.Name)
		}
		t.Fatalf("want 15 tools, got %d: %v", len(res.Tools), names)
	}
}

func TestCallWriteThenRead(t *testing.T) {
	cs := connect(t, newTestServer(t))
	ctx := context.Background()

	if _, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "fs_write",
		Arguments: map[string]any{"path": "a.txt", "content": "hello mcp"},
	}); err != nil {
		t.Fatalf("fs_write: %v", err)
	}

	out, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "fs_read",
		Arguments: map[string]any{"path": "a.txt"},
	})
	if err != nil {
		t.Fatalf("fs_read: %v", err)
	}
	if out.IsError {
		t.Fatalf("fs_read returned tool error: %+v", out.Content)
	}
	got, ok := out.StructuredContent.(map[string]any)
	if !ok || got["content"] != "hello mcp" {
		t.Fatalf("unexpected content: %#v", out.StructuredContent)
	}
}

func TestCallExec(t *testing.T) {
	cs := connect(t, newTestServer(t))
	out, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "exec",
		Arguments: map[string]any{"command": "echo", "args": []string{"hi"}},
	})
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if out.IsError {
		t.Fatalf("exec tool error: %+v", out.Content)
	}
}

func TestCallInfo(t *testing.T) {
	cs := connect(t, newTestServer(t))
	out, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "info"})
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if out.IsError {
		t.Fatalf("info tool error: %+v", out.Content)
	}
}

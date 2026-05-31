package handler_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"computer-use/internal/execapi"
	"computer-use/internal/fsapi"
	"computer-use/internal/handler"
	"computer-use/internal/middleware"
	"computer-use/internal/procapi"
)

func newServer(t *testing.T, apiKey string) *httptest.Server {
	t.Helper()
	fs, err := fsapi.New(t.TempDir())
	if err != nil {
		t.Fatalf("fsapi.New: %v", err)
	}
	ex := execapi.New(fs.Root, 5*time.Second, 10*time.Second)
	pm := procapi.NewManager(fs.Root)
	h := handler.New(fs, ex, pm, slog.New(slog.NewTextHandler(io.Discard, nil)))

	mux := http.NewServeMux()
	h.Routes(mux)
	auth := middleware.APIKey(apiKey, "/healthz")
	srv := httptest.NewServer(middleware.Chain(mux, auth))
	t.Cleanup(srv.Close)
	return srv
}

func TestHealthPublic(t *testing.T) {
	srv := newServer(t, "k")
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestAuthRequired(t *testing.T) {
	srv := newServer(t, "secret")
	resp, _ := http.Get(srv.URL + "/api/v1/fs/list")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", resp.StatusCode)
	}
}

func TestAuthHeaderAccepted(t *testing.T) {
	srv := newServer(t, "secret")
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/fs/list?path=/", nil)
	req.Header.Set("X-API-Key", "secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("want 200 got %d", resp.StatusCode)
	}
}

func TestWriteThenRead(t *testing.T) {
	srv := newServer(t, "")
	body := `{"path":"x.txt","content":"data"}`
	resp, err := http.Post(srv.URL+"/api/v1/fs/write", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("write status %d", resp.StatusCode)
	}

	resp2, err := http.Get(srv.URL + "/api/v1/fs/read?path=x.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	var out struct {
		Content string `json:"content"`
	}
	_ = json.NewDecoder(resp2.Body).Decode(&out)
	if out.Content != "data" {
		t.Fatalf("content %q", out.Content)
	}
}

func TestExecEndpoint(t *testing.T) {
	srv := newServer(t, "")
	resp, err := http.Post(srv.URL+"/api/v1/exec", "application/json",
		strings.NewReader(`{"command":"echo","args":["yo"]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct {
		Stdout   string `json:"stdout"`
		ExitCode int    `json:"exit_code"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if strings.TrimSpace(out.Stdout) != "yo" {
		t.Fatalf("stdout %q", out.Stdout)
	}
}

func TestWriteBadJSON(t *testing.T) {
	srv := newServer(t, "")
	resp, _ := http.Post(srv.URL+"/api/v1/fs/write", "application/json", strings.NewReader("{bad"))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", resp.StatusCode)
	}
}

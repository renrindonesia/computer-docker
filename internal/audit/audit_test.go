package audit

import (
	"bufio"
	"os"
	"path/filepath"
	"testing"
)

func TestRingCapAndRecent(t *testing.T) {
	r, _ := New(3, "")
	for _, a := range []string{"a", "b", "c", "d"} {
		r.Record(a, "t", "remote", nil)
	}
	got := r.Recent(0)
	if len(got) != 3 {
		t.Fatalf("want 3 got %d", len(got))
	}
	if got[0].Action != "b" || got[2].Action != "d" {
		t.Fatalf("ring order wrong: %v", got)
	}
}

func TestRecentLimit(t *testing.T) {
	r, _ := New(10, "")
	for _, a := range []string{"a", "b", "c"} {
		r.Record(a, "", "", nil)
	}
	if g := r.Recent(1); len(g) != 1 || g[0].Action != "c" {
		t.Fatalf("limit wrong: %v", g)
	}
}

func TestFileAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	r, err := New(10, path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	r.Record("exec", "ls", "1.2.3.4", map[string]any{"args": []string{"-la"}})
	r.Record("fs_write", "a.txt", "mcp", nil)
	_ = r.Close()

	f, _ := os.Open(path)
	defer f.Close()
	n := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		n++
	}
	if n != 2 {
		t.Fatalf("want 2 lines got %d", n)
	}
}

func TestNilSafe(t *testing.T) {
	var r *Recorder
	r.Record("x", "", "", nil) // must not panic
}

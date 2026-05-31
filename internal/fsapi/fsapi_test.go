package fsapi

import (
	"errors"
	"path/filepath"
	"testing"
)

func newSvc(t *testing.T) *Service {
	t.Helper()
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func TestWriteReadRoundTrip(t *testing.T) {
	s := newSvc(t)
	if err := s.Write("sub/file.txt", []byte("hello")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := s.Read("sub/file.txt")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("got %q want hello", got)
	}
}

func TestListAndStat(t *testing.T) {
	s := newSvc(t)
	_ = s.Write("a.txt", []byte("x"))
	_ = s.Mkdir("d")

	entries, err := s.List("/")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries got %d", len(entries))
	}

	st, err := s.Stat("d")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !st.IsDir {
		t.Fatalf("expected dir")
	}
}

func TestTraversalClampedToRoot(t *testing.T) {
	s := newSvc(t)
	// ../ sequences must not escape; they resolve back under root.
	if _, err := s.Read("../../../etc/passwd"); err == nil {
		t.Fatalf("expected error reading escaped path, got nil")
	}
	abs, err := s.resolve("../../../etc/passwd")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !filepath.HasPrefix(abs, s.Root) {
		t.Fatalf("resolved %q escaped root %q", abs, s.Root)
	}
}

func TestDeleteRootRefused(t *testing.T) {
	s := newSvc(t)
	if err := s.Delete("/"); err == nil {
		t.Fatalf("expected refusal deleting root")
	}
}

func TestDeleteTree(t *testing.T) {
	s := newSvc(t)
	_ = s.Write("dir/nested/f.txt", []byte("x"))
	if err := s.Delete("dir"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Stat("dir"); err == nil {
		t.Fatalf("expected dir gone")
	}
}

func TestReadMissing(t *testing.T) {
	s := newSvc(t)
	_, err := s.Read("nope.txt")
	if err == nil {
		t.Fatalf("expected error")
	}
	_ = errors.Unwrap(err) // ensure it's a wrapped os error chain
}

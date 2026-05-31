package fsapi

import "testing"

func TestMove(t *testing.T) {
	s := newSvc(t)
	_ = s.Write("a.txt", []byte("x"))
	if err := s.Move("a.txt", "sub/b.txt"); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if _, err := s.Stat("a.txt"); err == nil {
		t.Fatalf("source should be gone")
	}
	got, err := s.Read("sub/b.txt")
	if err != nil || string(got) != "x" {
		t.Fatalf("dst read: %v %q", err, got)
	}
}

func TestCopy(t *testing.T) {
	s := newSvc(t)
	_ = s.Write("a.txt", []byte("data"))
	if err := s.Copy("a.txt", "c.txt"); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	for _, p := range []string{"a.txt", "c.txt"} {
		got, err := s.Read(p)
		if err != nil || string(got) != "data" {
			t.Fatalf("%s: %v %q", p, err, got)
		}
	}
}

func TestChmod(t *testing.T) {
	s := newSvc(t)
	_ = s.Write("a.sh", []byte("#!/bin/sh"))
	if err := s.Chmod("a.sh", 0o755); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	st, _ := s.Stat("a.sh")
	if st.Mode != "-rwxr-xr-x" {
		t.Fatalf("mode %q", st.Mode)
	}
}

func TestSearchByGlob(t *testing.T) {
	s := newSvc(t)
	_ = s.Write("x.go", []byte("package x"))
	_ = s.Write("y.txt", []byte("hi"))
	_ = s.Write("sub/z.go", []byte("package z"))
	hits, err := s.Search("/", "*.go", "", 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("want 2 go files got %d", len(hits))
	}
}

func TestSearchByContent(t *testing.T) {
	s := newSvc(t)
	_ = s.Write("a.txt", []byte("hello\nNEEDLE here\nbye"))
	_ = s.Write("b.txt", []byte("nothing"))
	hits, err := s.Search("/", "", "NEEDLE", 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 || hits[0].Line != 2 {
		t.Fatalf("unexpected hits: %+v", hits)
	}
}

func TestSearchLimit(t *testing.T) {
	s := newSvc(t)
	for _, n := range []string{"1", "2", "3", "4"} {
		_ = s.Write(n+".txt", []byte("x"))
	}
	hits, _ := s.Search("/", "*.txt", "", 2)
	if len(hits) != 2 {
		t.Fatalf("want 2 (limit) got %d", len(hits))
	}
}

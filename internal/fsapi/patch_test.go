package fsapi

import (
	"errors"
	"testing"
)

func TestPatchReplaceUnique(t *testing.T) {
	s := newSvc(t)
	_ = s.Write("a.txt", []byte("line one\nline two\nline three\n"))
	if err := s.Patch("a.txt", "line two", "LINE 2"); err != nil {
		t.Fatalf("Patch: %v", err)
	}
	got, _ := s.Read("a.txt")
	if string(got) != "line one\nLINE 2\nline three\n" {
		t.Fatalf("got %q", got)
	}
}

func TestPatchCreateWhenOldEmpty(t *testing.T) {
	s := newSvc(t)
	if err := s.Patch("new.txt", "", "fresh"); err != nil {
		t.Fatalf("Patch: %v", err)
	}
	got, _ := s.Read("new.txt")
	if string(got) != "fresh" {
		t.Fatalf("got %q", got)
	}
}

func TestPatchNoMatch(t *testing.T) {
	s := newSvc(t)
	_ = s.Write("a.txt", []byte("hello"))
	if err := s.Patch("a.txt", "absent", "x"); !errors.Is(err, ErrPatchNoMatch) {
		t.Fatalf("want ErrPatchNoMatch got %v", err)
	}
}

func TestPatchAmbiguous(t *testing.T) {
	s := newSvc(t)
	_ = s.Write("a.txt", []byte("dup\ndup\n"))
	if err := s.Patch("a.txt", "dup", "x"); !errors.Is(err, ErrPatchAmbiguous) {
		t.Fatalf("want ErrPatchAmbiguous got %v", err)
	}
}

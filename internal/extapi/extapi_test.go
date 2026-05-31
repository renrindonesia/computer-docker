package extapi

import (
	"context"
	"testing"
)

func TestCatalogHasBrowserUse(t *testing.T) {
	m := NewManager()
	e, ok := m.Get("browser-use")
	if !ok {
		t.Fatal("browser-use missing from catalog")
	}
	if e.URL != "https://github.com/browser-use/browser-use" {
		t.Fatalf("url %q", e.URL)
	}
	if len(e.Install) == 0 || len(e.Check) == 0 {
		t.Fatal("browser-use missing install/check command")
	}
}

func TestListOrdered(t *testing.T) {
	m := NewManager()
	if len(m.List()) == 0 {
		t.Fatal("empty catalog")
	}
}

func TestGetUnknown(t *testing.T) {
	m := NewManager()
	if _, ok := m.Get("nope"); ok {
		t.Fatal("expected miss")
	}
}

func TestCheckNotInstalled(t *testing.T) {
	m := NewManager()
	// command that always fails => not installed, no panic.
	st := m.Check(context.Background(), Extension{
		Name:  "x",
		Check: []string{"sh", "-c", "exit 1"},
	})
	if st.Installed {
		t.Fatal("should be not installed")
	}
}

func TestCheckInstalled(t *testing.T) {
	m := NewManager()
	st := m.Check(context.Background(), Extension{
		Name:  "x",
		Check: []string{"sh", "-c", "echo ok"},
	})
	if !st.Installed || st.Detail != "ok" {
		t.Fatalf("got installed=%v detail=%q", st.Installed, st.Detail)
	}
}

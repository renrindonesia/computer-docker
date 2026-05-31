// Package extapi manages installable system extensions (e.g. browser-use).
//
// An extension is a named, data-driven capability with a Check command (fast,
// "is it installed?") and an Install command (slow, run as a background
// process by the caller). The catalog is fixed at build time; installation
// mutates the container at runtime.
package extapi

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// Extension is a catalog entry.
type Extension struct {
	Name        string
	Description string
	URL         string
	// Check is an argv run with a short timeout; exit 0 => installed.
	Check []string
	// Install is an argv meant to run as a long background process.
	Install []string
}

// Status is the runtime view of an extension.
type Status struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Installed   bool   `json:"installed"`
	Detail      string `json:"detail,omitempty"`
}

// Manager owns the extension catalog.
type Manager struct {
	order   []string
	catalog map[string]Extension
}

// NewManager returns a Manager preloaded with the built-in catalog.
func NewManager() *Manager {
	m := &Manager{catalog: map[string]Extension{}}
	for _, e := range builtins() {
		m.order = append(m.order, e.Name)
		m.catalog[e.Name] = e
	}
	return m
}

func builtins() []Extension {
	return []Extension{
		{
			Name:        "browser-use",
			Description: "Browser automation for AI agents (Playwright-based).",
			URL:         "https://github.com/browser-use/browser-use",
			Check:       []string{"python3", "-c", "import browser_use, sys; sys.stdout.write(getattr(browser_use,'__version__','installed'))"},
			Install:     []string{"sh", "-c", "pip install --no-cache-dir browser-use && playwright install --with-deps chromium"},
		},
	}
}

// Get returns one extension by name.
func (m *Manager) Get(name string) (Extension, bool) {
	e, ok := m.catalog[name]
	return e, ok
}

// List returns the catalog in declared order.
func (m *Manager) List() []Extension {
	out := make([]Extension, 0, len(m.order))
	for _, n := range m.order {
		out = append(out, m.catalog[n])
	}
	return out
}

// Check runs the extension's Check command and reports installed state.
func (m *Manager) Check(ctx context.Context, e Extension) Status {
	st := Status{Name: e.Name, Description: e.Description, URL: e.URL}
	if len(e.Check) == 0 {
		return st
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, e.Check[0], e.Check[1:]...)
	out, err := cmd.CombinedOutput()
	st.Installed = err == nil
	st.Detail = strings.TrimSpace(string(out))
	return st
}

// Statuses checks the whole catalog.
func (m *Manager) Statuses(ctx context.Context) []Status {
	out := make([]Status, 0, len(m.order))
	for _, n := range m.order {
		out = append(out, m.Check(ctx, m.catalog[n]))
	}
	return out
}

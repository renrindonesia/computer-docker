// Package audit records an append-only trail of mutating actions (exec, process
// starts, file writes, installs). It keeps an in-memory ring for quick reads and
// optionally appends JSON lines to a file outside the fs jail.
package audit

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// Entry is one audited action.
type Entry struct {
	Time   time.Time      `json:"time"`
	Action string         `json:"action"`
	Target string         `json:"target,omitempty"`
	Remote string         `json:"remote,omitempty"`
	Meta   map[string]any `json:"meta,omitempty"`
}

// Recorder stores recent entries in a ring and, if configured, appends to a file.
type Recorder struct {
	mu    sync.Mutex
	ring  []Entry
	max   int
	file  *os.File
	nowFn func() time.Time
}

// New creates a Recorder keeping the last max entries. If path is non-empty,
// entries are also appended there as JSON lines.
func New(max int, path string) (*Recorder, error) {
	if max <= 0 {
		max = 1000
	}
	r := &Recorder{max: max, nowFn: time.Now}
	if path != "" {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return nil, err
		}
		r.file = f
	}
	return r, nil
}

// Record appends an action. Safe for concurrent use; never blocks on I/O errors.
func (r *Recorder) Record(action, target, remote string, meta map[string]any) {
	if r == nil {
		return
	}
	e := Entry{
		Time:   r.nowFn(),
		Action: action,
		Target: target,
		Remote: remote,
		Meta:   meta,
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ring = append(r.ring, e)
	if len(r.ring) > r.max {
		r.ring = r.ring[len(r.ring)-r.max:]
	}
	if r.file != nil {
		if b, err := json.Marshal(e); err == nil {
			_, _ = r.file.Write(append(b, '\n'))
		}
	}
}

// Recent returns up to limit most-recent entries (newest last). limit<=0 => all.
func (r *Recorder) Recent(limit int) []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit <= 0 || limit > len(r.ring) {
		limit = len(r.ring)
	}
	out := make([]Entry, limit)
	copy(out, r.ring[len(r.ring)-limit:])
	return out
}

// Close closes the backing file if any.
func (r *Recorder) Close() error {
	if r != nil && r.file != nil {
		return r.file.Close()
	}
	return nil
}

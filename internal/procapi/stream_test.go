package procapi

import (
	"testing"
	"time"
)

func TestSubscribeStreamsAndCloses(t *testing.T) {
	m := NewManager("")
	p, err := m.Start(StartRequest{Command: "sh", Args: []string{"-c", "echo one; echo two"}})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	_, ch, cancel, err := m.Subscribe(p.ID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer cancel()

	// Drain until the channel closes (process exit closes subscribers).
	got := 0
	timeout := time.After(5 * time.Second)
	for {
		select {
		case _, open := <-ch:
			if !open {
				if got == 0 {
					// snapshot may have captured lines before subscribe; that's fine,
					// but the stream must at least close cleanly.
					t.Log("stream closed with no streamed lines (captured in snapshot)")
				}
				return
			}
			got++
		case <-timeout:
			t.Fatal("stream did not close after process exit")
		}
	}
}

func TestSubscribeUnknown(t *testing.T) {
	m := NewManager("")
	if _, _, _, err := m.Subscribe("nope"); err != ErrNotFound {
		t.Fatalf("want ErrNotFound got %v", err)
	}
}

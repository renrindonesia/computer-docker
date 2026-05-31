package procapi

import (
	"testing"
	"time"
)

func waitState(t *testing.T, m *Manager, id string, want State) ProcView {
	t.Helper()
	for i := 0; i < 200; i++ {
		p, err := m.Get(id)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if p.State == want {
			return p
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("process %s never reached %s", id, want)
	return ProcView{}
}

func TestStartAndExit(t *testing.T) {
	m := NewManager("")
	p, err := m.Start(StartRequest{Command: "echo", Args: []string{"hello"}})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if p.PID == 0 {
		t.Fatalf("no pid")
	}
	done := waitState(t, m, p.ID, StateExited)
	if done.ExitCode != 0 {
		t.Fatalf("exit %d", done.ExitCode)
	}
}

func TestLogsCaptured(t *testing.T) {
	m := NewManager("")
	p, _ := m.Start(StartRequest{Command: "sh", Args: []string{"-c", "echo out; echo err 1>&2"}})
	waitState(t, m, p.ID, StateExited)
	// allow pump goroutines to flush
	time.Sleep(50 * time.Millisecond)
	logs, err := m.Logs(p.ID)
	if err != nil {
		t.Fatalf("Logs: %v", err)
	}
	var sawOut, sawErr bool
	for _, l := range logs {
		if l.Stream == "stdout" && l.Text == "out" {
			sawOut = true
		}
		if l.Stream == "stderr" && l.Text == "err" {
			sawErr = true
		}
	}
	if !sawOut || !sawErr {
		t.Fatalf("missing logs: out=%v err=%v (%d lines)", sawOut, sawErr, len(logs))
	}
}

func TestStopLongRunning(t *testing.T) {
	m := NewManager("")
	p, err := m.Start(StartRequest{Command: "sleep", Args: []string{"30"}})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := m.Stop(p.ID); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	done := waitState(t, m, p.ID, StateExited)
	if !done.EndedAt.After(done.StartedAt) {
		t.Fatalf("ended not after started")
	}
}

func TestRemove(t *testing.T) {
	m := NewManager("")
	p, _ := m.Start(StartRequest{Command: "echo", Args: []string{"x"}})
	if err := m.Remove(p.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := m.Get(p.ID); err != ErrNotFound {
		t.Fatalf("want ErrNotFound got %v", err)
	}
}

func TestStartNoCommand(t *testing.T) {
	m := NewManager("")
	if _, err := m.Start(StartRequest{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGetUnknown(t *testing.T) {
	m := NewManager("")
	if _, err := m.Get("nope"); err != ErrNotFound {
		t.Fatalf("want ErrNotFound got %v", err)
	}
}

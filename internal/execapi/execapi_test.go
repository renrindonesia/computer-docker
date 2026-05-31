package execapi

import (
	"context"
	"strings"
	"testing"
	"time"
)

func newSvc() *Service {
	return New("", 5*time.Second, 10*time.Second)
}

func TestRunEcho(t *testing.T) {
	res, err := newSvc().Run(context.Background(), Request{Command: "echo", Args: []string{"hi"}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.TrimSpace(res.Stdout) != "hi" {
		t.Fatalf("stdout %q", res.Stdout)
	}
	if res.ExitCode != 0 {
		t.Fatalf("exit %d", res.ExitCode)
	}
}

func TestRunNonZeroExit(t *testing.T) {
	res, err := newSvc().Run(context.Background(), Request{Command: "sh", Args: []string{"-c", "exit 3"}})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.ExitCode != 3 {
		t.Fatalf("want exit 3 got %d", res.ExitCode)
	}
}

func TestRunStdin(t *testing.T) {
	res, err := newSvc().Run(context.Background(), Request{Command: "cat", Stdin: "piped"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Stdout != "piped" {
		t.Fatalf("stdout %q", res.Stdout)
	}
}

func TestRunTimeout(t *testing.T) {
	s := New("", time.Second, 10*time.Second)
	res, err := s.Run(context.Background(), Request{Command: "sleep", Args: []string{"5"}, Timeout: 1})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.TimedOut {
		t.Fatalf("expected timeout")
	}
}

func TestRunNoCommand(t *testing.T) {
	_, err := newSvc().Run(context.Background(), Request{})
	if err != ErrNoCommand {
		t.Fatalf("want ErrNoCommand got %v", err)
	}
}

func TestMaxTimeoutClamp(t *testing.T) {
	s := New("", time.Second, 2*time.Second)
	// request asks 100s, clamp to MaxTimeout 2s; sleep 5 must time out.
	res, err := s.Run(context.Background(), Request{Command: "sleep", Args: []string{"5"}, Timeout: 100})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.TimedOut {
		t.Fatalf("expected clamp+timeout")
	}
}

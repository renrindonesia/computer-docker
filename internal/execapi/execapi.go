// Package execapi runs shell commands with timeout and output capture.
package execapi

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"
)

// Service runs commands rooted at WorkDir with a default timeout.
type Service struct {
	WorkDir        string
	DefaultTimeout time.Duration
	MaxTimeout     time.Duration
}

// New creates an exec Service.
func New(workDir string, defaultTimeout, maxTimeout time.Duration) *Service {
	return &Service{WorkDir: workDir, DefaultTimeout: defaultTimeout, MaxTimeout: maxTimeout}
}

// Request describes a command to run.
type Request struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Dir     string   `json:"dir"`
	Stdin   string   `json:"stdin"`
	Timeout int      `json:"timeout_sec"`
}

// Result is the outcome of a run.
type Result struct {
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
	TimedOut   bool   `json:"timed_out"`
	DurationMs int64  `json:"duration_ms"`
}

// ErrNoCommand is returned when Command is empty.
var ErrNoCommand = errors.New("command required")

// Run executes the request and captures stdout/stderr.
func (s *Service) Run(ctx context.Context, req Request) (Result, error) {
	if req.Command == "" {
		return Result{}, ErrNoCommand
	}

	timeout := s.DefaultTimeout
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
	}
	if s.MaxTimeout > 0 && timeout > s.MaxTimeout {
		timeout = s.MaxTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, req.Command, req.Args...)
	cmd.Dir = s.WorkDir
	if req.Dir != "" {
		cmd.Dir = req.Dir
	}
	if req.Stdin != "" {
		cmd.Stdin = bytes.NewBufferString(req.Stdin)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	dur := time.Since(start)

	res := Result{
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		DurationMs: dur.Milliseconds(),
		ExitCode:   0,
	}

	if ctx.Err() == context.DeadlineExceeded {
		res.TimedOut = true
		res.ExitCode = -1
		return res, nil
	}

	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			res.ExitCode = ee.ExitCode()
			return res, nil
		}
		// command not found, permission, etc.
		return res, err
	}

	return res, nil
}

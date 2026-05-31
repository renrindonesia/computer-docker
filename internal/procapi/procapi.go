// Package procapi manages long-running background processes.
package procapi

import (
	"bufio"
	"errors"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// ErrNotFound is returned for an unknown process id.
var ErrNotFound = errors.New("process not found")

// State is a process lifecycle stage.
type State string

const (
	StateRunning State = "running"
	StateExited  State = "exited"
)

// ringBuffer keeps the last N log lines (combined stdout+stderr).
type ringBuffer struct {
	mu    sync.Mutex
	lines []LogLine
	max   int
}

// LogLine is one captured output line from a process.
type LogLine struct {
	Stream string    `json:"stream"`
	Text   string    `json:"text"`
	At     time.Time `json:"at"`
}

func (r *ringBuffer) add(stream, text string, at time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, LogLine{stream, text, at})
	if len(r.lines) > r.max {
		r.lines = r.lines[len(r.lines)-r.max:]
	}
}

func (r *ringBuffer) snapshot() []LogLine {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]LogLine, len(r.lines))
	copy(out, r.lines)
	return out
}

// Process is a managed background command. Mutable fields (state, exitCode,
// endedAt) are guarded by mu; reads go through view().
type Process struct {
	id        string
	command   string
	args      []string
	dir       string
	pid       int
	startedAt time.Time

	cmd  *exec.Cmd
	logs *ringBuffer

	mu       sync.Mutex
	state    State
	exitCode int
	endedAt  time.Time
}

// ProcView is an immutable snapshot safe to marshal and hand out.
type ProcView struct {
	ID        string    `json:"id"`
	Command   string    `json:"command"`
	Args      []string  `json:"args"`
	Dir       string    `json:"dir"`
	PID       int       `json:"pid"`
	State     State     `json:"state"`
	ExitCode  int       `json:"exit_code"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
}

func (p *Process) setState(s State, code int, at time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state = s
	p.exitCode = code
	p.endedAt = at
}

func (p *Process) view() ProcView {
	p.mu.Lock()
	defer p.mu.Unlock()
	return ProcView{
		ID:        p.id,
		Command:   p.command,
		Args:      p.args,
		Dir:       p.dir,
		PID:       p.pid,
		State:     p.state,
		ExitCode:  p.exitCode,
		StartedAt: p.startedAt,
		EndedAt:   p.endedAt,
	}
}

// Manager owns the process table.
type Manager struct {
	mu      sync.Mutex
	procs   map[string]*Process
	workDir string
	nowFn   func() time.Time
	seq     int
}

// NewManager creates a Manager rooting commands at workDir.
func NewManager(workDir string) *Manager {
	return &Manager{
		procs:   map[string]*Process{},
		workDir: workDir,
		nowFn:   time.Now,
	}
}

// StartRequest describes a process to launch.
type StartRequest struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Dir     string            `json:"dir"`
	Env     map[string]string `json:"env"`
}

func (m *Manager) nextID() string {
	m.seq++
	// monotonic, collision-free without time/random.
	return "p" + itoa(m.seq)
}

// Start launches a background process and returns a snapshot of it.
func (m *Manager) Start(req StartRequest) (ProcView, error) {
	if req.Command == "" {
		return ProcView{}, errors.New("command required")
	}
	cmd := exec.Command(req.Command, req.Args...)
	cmd.Dir = m.workDir
	if req.Dir != "" {
		cmd.Dir = req.Dir
	}
	if len(req.Env) > 0 {
		env := []string{}
		for k, v := range req.Env {
			env = append(env, k+"="+v)
		}
		cmd.Env = append(cmd.Environ(), env...)
	}
	// own process group so we can kill children too.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return ProcView{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return ProcView{}, err
	}
	if err := cmd.Start(); err != nil {
		return ProcView{}, err
	}

	now := m.nowFn()
	p := &Process{
		command:   req.Command,
		args:      req.Args,
		dir:       cmd.Dir,
		pid:       cmd.Process.Pid,
		state:     StateRunning,
		startedAt: now,
		cmd:       cmd,
		logs:      &ringBuffer{max: 1000},
	}

	m.mu.Lock()
	p.id = m.nextID()
	m.procs[p.id] = p
	m.mu.Unlock()

	go pump(p, "stdout", stdout, m.nowFn)
	go pump(p, "stderr", stderr, m.nowFn)
	go func() {
		err := cmd.Wait()
		code := 0
		if err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				code = ee.ExitCode()
			} else {
				code = -1
			}
		}
		p.setState(StateExited, code, m.nowFn())
	}()

	return p.view(), nil
}

func pump(p *Process, stream string, r io.Reader, now func() time.Time) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		p.logs.add(stream, sc.Text(), now())
	}
}

// find returns the live process under the table lock.
func (m *Manager) find(id string) (*Process, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.procs[id]
	if !ok {
		return nil, ErrNotFound
	}
	return p, nil
}

// List returns snapshots of all processes.
func (m *Manager) List() []ProcView {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ProcView, 0, len(m.procs))
	for _, p := range m.procs {
		out = append(out, p.view())
	}
	return out
}

// Get returns a snapshot of one process by id.
func (m *Manager) Get(id string) (ProcView, error) {
	p, err := m.find(id)
	if err != nil {
		return ProcView{}, err
	}
	return p.view(), nil
}

// Logs returns the buffered log lines for a process.
func (m *Manager) Logs(id string) ([]LogLine, error) {
	p, err := m.find(id)
	if err != nil {
		return nil, err
	}
	return p.logs.snapshot(), nil
}

// Stop sends SIGTERM to the process group of id.
func (m *Manager) Stop(id string) error {
	p, err := m.find(id)
	if err != nil {
		return err
	}
	if p.view().State != StateRunning {
		return nil
	}
	// negative pid → whole group.
	return syscall.Kill(-p.pid, syscall.SIGTERM)
}

// Remove stops (if running) and drops the process from the table.
func (m *Manager) Remove(id string) error {
	if err := m.Stop(id); err != nil && !errors.Is(err, ErrNotFound) {
		// ignore kill errors (already dead), still remove.
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.procs[id]; !ok {
		return ErrNotFound
	}
	delete(m.procs, id)
	return nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

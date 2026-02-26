package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"
	"time"
)

// ProcessStarter abstracts subprocess creation for testability.
type ProcessStarter interface {
	Start(ctx context.Context, cmd string, args []string, env []string, dir string) (Process, error)
}

// Process abstracts a running subprocess.
type Process interface {
	Stdin() io.WriteCloser
	Stdout() io.ReadCloser
	Stderr() io.ReadCloser
	Wait() error
	Kill() error
}

type STDIOConfig struct {
	Command string
	Args    []string
	Env     []string
	Dir     string
}

type STDIOTransport struct {
	config  STDIOConfig
	starter ProcessStarter
	process Process
	mu      sync.Mutex
	pending map[string]chan json.RawMessage
	closed  bool
}

type defaultProcessStarter struct{}

func (d *defaultProcessStarter) Start(_ context.Context, cmd string, args []string, env []string, dir string) (Process, error) {
	c := exec.Command(cmd, args...)
	if len(env) > 0 {
		c.Env = env
	}
	if dir != "" {
		c.Dir = dir
	}

	stdin, err := c.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdio: stdin pipe: %w", err)
	}
	stdout, err := c.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdio: stdout pipe: %w", err)
	}
	stderr, err := c.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stdio: stderr pipe: %w", err)
	}

	if err := c.Start(); err != nil {
		return nil, fmt.Errorf("stdio: starting process: %w", err)
	}

	return &realProcess{cmd: c, stdin: stdin, stdout: stdout, stderr: stderr}, nil
}

type realProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
}

func (p *realProcess) Stdin() io.WriteCloser  { return p.stdin }
func (p *realProcess) Stdout() io.ReadCloser  { return p.stdout }
func (p *realProcess) Stderr() io.ReadCloser  { return p.stderr }
func (p *realProcess) Wait() error            { return p.cmd.Wait() }
func (p *realProcess) Kill() error            { return p.cmd.Process.Kill() }

func NewSTDIO(cfg STDIOConfig, opts ...func(*STDIOTransport)) (*STDIOTransport, error) {
	if cfg.Command == "" {
		return nil, fmt.Errorf("stdio: command is required")
	}
	if _, err := exec.LookPath(cfg.Command); err != nil {
		return nil, fmt.Errorf("stdio: command not found: %w", err)
	}
	t := &STDIOTransport{
		config:  cfg,
		starter: &defaultProcessStarter{},
		pending: make(map[string]chan json.RawMessage),
	}
	for _, o := range opts {
		o(t)
	}
	return t, nil
}

func WithProcessStarter(s ProcessStarter) func(*STDIOTransport) {
	return func(t *STDIOTransport) { t.starter = s }
}

func (t *STDIOTransport) Open(ctx context.Context) error {
	proc, err := t.starter.Start(ctx, t.config.Command, t.config.Args, t.config.Env, t.config.Dir)
	if err != nil {
		return err
	}
	t.process = proc

	// Read stdout, route responses by JSON-RPC ID
	go func() {
		scanner := bufio.NewScanner(proc.Stdout())
		for scanner.Scan() {
			line := scanner.Bytes()
			var env struct {
				ID json.RawMessage `json:"id"`
			}
			if err := json.Unmarshal(line, &env); err != nil {
				slog.Warn("stdio: invalid JSON from subprocess", "err", err)
				continue
			}
			idKey := string(env.ID)

			t.mu.Lock()
			ch, ok := t.pending[idKey]
			if ok {
				delete(t.pending, idKey)
			}
			t.mu.Unlock()

			if ok {
				ch <- json.RawMessage(append([]byte(nil), line...))
			}
		}
		// Close all pending channels on EOF
		t.mu.Lock()
		for _, ch := range t.pending {
			close(ch)
		}
		t.pending = make(map[string]chan json.RawMessage)
		t.mu.Unlock()
	}()

	// Log stderr
	go func() {
		scanner := bufio.NewScanner(proc.Stderr())
		for scanner.Scan() {
			slog.Debug("stdio: subprocess stderr", "line", scanner.Text())
		}
	}()

	return nil
}

func (t *STDIOTransport) Send(ctx context.Context, payload []byte) ([]byte, error) {
	// Extract ID from payload
	var env struct {
		ID json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(payload, &env); err != nil {
		return nil, fmt.Errorf("stdio: parsing request id: %w", err)
	}
	idKey := string(env.ID)

	ch := make(chan json.RawMessage, 1)
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil, fmt.Errorf("stdio: transport closed")
	}
	t.pending[idKey] = ch
	t.mu.Unlock()

	// Write payload + newline to stdin
	line := append(payload, '\n')
	if _, err := t.process.Stdin().Write(line); err != nil {
		t.mu.Lock()
		delete(t.pending, idKey)
		t.mu.Unlock()
		return nil, fmt.Errorf("stdio: writing to subprocess: %w", err)
	}

	// Wait for response — caller controls timeout via context
	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("stdio: subprocess exited before responding")
		}
		return []byte(resp), nil
	case <-ctx.Done():
		t.mu.Lock()
		delete(t.pending, idKey)
		t.mu.Unlock()
		return nil, ctx.Err()
	}
}

func (t *STDIOTransport) Close() error {
	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()

	if t.process == nil {
		return nil
	}

	t.process.Stdin().Close()

	done := make(chan error, 1)
	go func() { done <- t.process.Wait() }()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.process.Kill()
		<-done
	}
	return nil
}

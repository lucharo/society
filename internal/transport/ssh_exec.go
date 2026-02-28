package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/luischavesdev/society/internal/models"
	"golang.org/x/crypto/ssh"
)

// SSHExecDialer abstracts SSH connection creation for testability.
type SSHExecDialer interface {
	Dial(network, addr string, config *ssh.ClientConfig) (SSHExecClient, error)
}

// SSHExecClient abstracts an SSH client that can create sessions.
type SSHExecClient interface {
	NewSession() (SSHSession, error)
	Close() error
}

// SSHSession abstracts an SSH session for running commands.
type SSHSession interface {
	Run(cmd string) error
	SetStdout(w io.Writer)
	SetStderr(w io.Writer)
	Close() error
}

type SSHExecConfig struct {
	Host    string
	User    string
	Port    int
	KeyPath string
	Command string
	Args    []string
}

type SSHExecTransport struct {
	config SSHExecConfig
	dialer SSHExecDialer
	client SSHExecClient
}

type defaultSSHExecDialer struct{}

func (d *defaultSSHExecDialer) Dial(network, addr string, config *ssh.ClientConfig) (SSHExecClient, error) {
	c, err := ssh.Dial(network, addr, config)
	if err != nil {
		return nil, err
	}
	return &realSSHExecClient{c}, nil
}

type realSSHExecClient struct{ *ssh.Client }

func (c *realSSHExecClient) NewSession() (SSHSession, error) {
	s, err := c.Client.NewSession()
	if err != nil {
		return nil, err
	}
	return &realSSHSession{s}, nil
}

func (c *realSSHExecClient) Close() error {
	return c.Client.Close()
}

type realSSHSession struct{ *ssh.Session }

func (s *realSSHSession) Run(cmd string) error {
	return s.Session.Run(cmd)
}

func (s *realSSHSession) SetStdout(w io.Writer) {
	s.Session.Stdout = w
}

func (s *realSSHSession) SetStderr(w io.Writer) {
	s.Session.Stderr = w
}

func (s *realSSHSession) Close() error {
	return s.Session.Close()
}

func NewSSHExec(cfg SSHExecConfig, opts ...func(*SSHExecTransport)) (*SSHExecTransport, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("ssh-exec: host is required")
	}
	if cfg.User == "" {
		return nil, fmt.Errorf("ssh-exec: user is required")
	}
	if cfg.KeyPath == "" {
		return nil, fmt.Errorf("ssh-exec: key_path is required")
	}
	if cfg.Command == "" {
		return nil, fmt.Errorf("ssh-exec: command is required")
	}
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	t := &SSHExecTransport{
		config: cfg,
		dialer: &defaultSSHExecDialer{},
	}
	for _, o := range opts {
		o(t)
	}
	return t, nil
}

func WithSSHExecDialer(d SSHExecDialer) func(*SSHExecTransport) {
	return func(t *SSHExecTransport) { t.dialer = d }
}

func (t *SSHExecTransport) Open(ctx context.Context) error {
	keyData, err := os.ReadFile(t.config.KeyPath)
	if err != nil {
		return fmt.Errorf("ssh-exec: reading key: %w", err)
	}
	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return fmt.Errorf("ssh-exec: parsing key: %w", err)
	}

	sshCfg := &ssh.ClientConfig{
		User:            t.config.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: SSHHostKeyCallback(),
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", t.config.Host, t.config.Port)
	client, err := t.dialer.Dial("tcp", addr, sshCfg)
	if err != nil {
		return fmt.Errorf("ssh-exec: connecting to %s: %w", addr, err)
	}
	t.client = client
	return nil
}

func (t *SSHExecTransport) Send(ctx context.Context, payload []byte) ([]byte, error) {
	// Parse JSON-RPC request to extract user text
	var req struct {
		JSONRPC string `json:"jsonrpc"`
		ID      any    `json:"id"`
		Params  models.SendTaskParams
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("ssh-exec: parsing request: %w", err)
	}

	var userText string
	for _, p := range req.Params.Message.Parts {
		if p.Type == "text" {
			if userText != "" {
				userText += "\n"
			}
			userText += p.Text
		}
	}

	// Build command string
	cmdParts := []string{t.config.Command}
	cmdParts = append(cmdParts, t.config.Args...)
	cmdParts = append(cmdParts, shellEscape(userText))
	cmdStr := strings.Join(cmdParts, " ")

	// Create session and run
	sess, err := t.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("ssh-exec: creating session: %w", err)
	}
	defer sess.Close()

	var stdout, stderr bytes.Buffer
	sess.SetStdout(&stdout)
	sess.SetStderr(&stderr)

	// Run with context cancellation
	done := make(chan error, 1)
	go func() { done <- sess.Run(cmdStr) }()

	select {
	case err = <-done:
		// command completed
	case <-ctx.Done():
		sess.Close()
		return nil, ctx.Err()
	}

	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return marshalTaskResponse(req.ID, req.Params.ID, models.TaskStateFailed, errMsg, "")
	}

	response := parseCliResponse(stdout.String())
	return marshalTaskResponse(req.ID, req.Params.ID, models.TaskStateCompleted, "", response)
}

func (t *SSHExecTransport) Close() error {
	if t.client != nil {
		return t.client.Close()
	}
	return nil
}

// shellEscape wraps a string in single quotes for safe shell interpolation.
func shellEscape(s string) string {
	// Replace single quotes: ' -> '\''
	escaped := strings.ReplaceAll(s, "'", "'\\''")
	return "'" + escaped + "'"
}

// parseCliResponse extracts text from CLI output, handling Claude's JSON format.
// Mirrors parseResponse in internal/agent/exec.go.
func parseCliResponse(stdout string) string {
	stdout = strings.TrimSpace(stdout)
	var claudeResp struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout), &claudeResp); err == nil && claudeResp.Result != "" {
		return claudeResp.Result
	}
	return stdout
}

// marshalTaskResponse builds a JSON-RPC response containing a Task.
func marshalTaskResponse(reqID any, taskID string, state models.TaskState, errMsg, text string) ([]byte, error) {
	task := models.Task{
		ID:     taskID,
		Status: models.TaskStatus{State: state, Message: errMsg},
	}
	if text != "" {
		task.Artifacts = []models.Artifact{
			{Parts: []models.Part{{Type: "text", Text: text}}},
		}
	}
	resp := models.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      reqID,
		Result:  task,
	}
	return json.Marshal(resp)
}

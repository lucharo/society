package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luischavesdev/society/internal/models"
	"golang.org/x/crypto/ssh"
)

// --- Mock types ---

type mockSSHExecDialer struct {
	client SSHExecClient
	err    error
}

func (m *mockSSHExecDialer) Dial(network, addr string, config *ssh.ClientConfig) (SSHExecClient, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.client, nil
}

type mockSSHExecClient struct {
	sessions []*mockSSHExecSession
	idx      int
	closed   bool
}

func (m *mockSSHExecClient) NewSession() (SSHSession, error) {
	if m.idx >= len(m.sessions) {
		return nil, fmt.Errorf("no more mock sessions")
	}
	s := m.sessions[m.idx]
	m.idx++
	return s, nil
}

func (m *mockSSHExecClient) Close() error {
	m.closed = true
	return nil
}

type mockSSHExecSession struct {
	stdout  string
	stderr  string
	exitErr error
	lastCmd string
	stdoutW io.Writer
	stderrW io.Writer
}

func (m *mockSSHExecSession) Run(cmd string) error {
	m.lastCmd = cmd
	if m.stdoutW != nil {
		m.stdoutW.Write([]byte(m.stdout))
	}
	if m.stderrW != nil {
		m.stderrW.Write([]byte(m.stderr))
	}
	return m.exitErr
}

func (m *mockSSHExecSession) SetStdout(w io.Writer) { m.stdoutW = w }
func (m *mockSSHExecSession) SetStderr(w io.Writer) { m.stderrW = w }
func (m *mockSSHExecSession) Close() error          { return nil }

func writeTestKeyFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_test")
	os.WriteFile(keyPath, []byte("not-a-real-key"), 0600)
	return keyPath
}

// --- Constructor tests ---

func TestSSHExecTransport_Validation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     SSHExecConfig
		wantErr string
	}{
		{"missing host", SSHExecConfig{User: "u", KeyPath: "/k", Command: "c"}, "host is required"},
		{"missing user", SSHExecConfig{Host: "h", KeyPath: "/k", Command: "c"}, "user is required"},
		{"missing key", SSHExecConfig{Host: "h", User: "u", Command: "c"}, "key_path is required"},
		{"missing command", SSHExecConfig{Host: "h", User: "u", KeyPath: "/k"}, "command is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSSHExec(tt.cfg)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q should contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestSSHExecTransport_DefaultPort(t *testing.T) {
	tr, err := NewSSHExec(SSHExecConfig{
		Host: "h", User: "u", KeyPath: "/k", Command: "c",
	})
	if err != nil {
		t.Fatal(err)
	}
	if tr.config.Port != 22 {
		t.Errorf("default port = %d, want 22", tr.config.Port)
	}
}

// --- Send tests ---

func makePayload(t *testing.T, text string) []byte {
	t.Helper()
	req := models.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "test-1",
		Method:  "tasks/send",
		Params: models.SendTaskParams{
			ID: "task-1",
			Message: models.Message{
				Role:  "user",
				Parts: []models.Part{{Type: "text", Text: text}},
			},
		},
	}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSSHExecTransport_Send_JSONResponse(t *testing.T) {
	sess := &mockSSHExecSession{
		stdout: `{"result": "Hello from Claude!"}`,
	}
	client := &mockSSHExecClient{sessions: []*mockSSHExecSession{sess}}

	tr := &SSHExecTransport{
		config: SSHExecConfig{Command: "claude", Args: []string{"-p", "--output-format", "json"}},
		client: client,
	}

	resp, err := tr.Send(context.Background(), makePayload(t, "hi"))
	if err != nil {
		t.Fatal(err)
	}

	var jsonResp models.JSONRPCResponse
	if err := json.Unmarshal(resp, &jsonResp); err != nil {
		t.Fatal(err)
	}

	// Extract task from result
	resultBytes, _ := json.Marshal(jsonResp.Result)
	var task models.Task
	json.Unmarshal(resultBytes, &task)

	if task.Status.State != models.TaskStateCompleted {
		t.Errorf("state = %q, want completed", task.Status.State)
	}
	if len(task.Artifacts) == 0 || task.Artifacts[0].Parts[0].Text != "Hello from Claude!" {
		t.Errorf("unexpected artifact text: %+v", task.Artifacts)
	}

	// Verify command was built correctly
	if !strings.Contains(sess.lastCmd, "claude") {
		t.Errorf("command should contain 'claude', got %q", sess.lastCmd)
	}
	if !strings.Contains(sess.lastCmd, "-p") {
		t.Errorf("command should contain args, got %q", sess.lastCmd)
	}
}

func TestSSHExecTransport_Send_PlainTextResponse(t *testing.T) {
	sess := &mockSSHExecSession{
		stdout: "Just plain text response",
	}
	client := &mockSSHExecClient{sessions: []*mockSSHExecSession{sess}}

	tr := &SSHExecTransport{
		config: SSHExecConfig{Command: "codex"},
		client: client,
	}

	resp, err := tr.Send(context.Background(), makePayload(t, "hello"))
	if err != nil {
		t.Fatal(err)
	}

	var jsonResp models.JSONRPCResponse
	json.Unmarshal(resp, &jsonResp)
	resultBytes, _ := json.Marshal(jsonResp.Result)
	var task models.Task
	json.Unmarshal(resultBytes, &task)

	if task.Artifacts[0].Parts[0].Text != "Just plain text response" {
		t.Errorf("text = %q, want plain text", task.Artifacts[0].Parts[0].Text)
	}
}

func TestSSHExecTransport_Send_CommandFailure(t *testing.T) {
	sess := &mockSSHExecSession{
		stderr:  "command not found: claude",
		exitErr: fmt.Errorf("exit status 127"),
	}
	client := &mockSSHExecClient{sessions: []*mockSSHExecSession{sess}}

	tr := &SSHExecTransport{
		config: SSHExecConfig{Command: "claude"},
		client: client,
	}

	resp, err := tr.Send(context.Background(), makePayload(t, "hello"))
	if err != nil {
		t.Fatal(err)
	}

	var jsonResp models.JSONRPCResponse
	json.Unmarshal(resp, &jsonResp)
	resultBytes, _ := json.Marshal(jsonResp.Result)
	var task models.Task
	json.Unmarshal(resultBytes, &task)

	if task.Status.State != models.TaskStateFailed {
		t.Errorf("state = %q, want failed", task.Status.State)
	}
	if !strings.Contains(task.Status.Message, "command not found") {
		t.Errorf("message = %q, want to contain 'command not found'", task.Status.Message)
	}
}

func TestSSHExecTransport_Send_ShellEscaping(t *testing.T) {
	sess := &mockSSHExecSession{stdout: "ok"}
	client := &mockSSHExecClient{sessions: []*mockSSHExecSession{sess}}

	tr := &SSHExecTransport{
		config: SSHExecConfig{Command: "claude", Args: []string{"-p"}},
		client: client,
	}

	// Message with single quotes, backticks, and dollar signs
	_, err := tr.Send(context.Background(), makePayload(t, "it's a $HOME `test`"))
	if err != nil {
		t.Fatal(err)
	}

	// The command should be wrapped in bash -l -c '...' for login shell
	cmd := sess.lastCmd
	if !strings.HasPrefix(cmd, "bash -l -c ") {
		t.Errorf("command should start with 'bash -l -c ', got %q", cmd)
	}
	// Should contain the escaped single quote pattern (from "it's")
	if !strings.Contains(cmd, "'\\''") {
		t.Errorf("single quotes should be escaped with '\\'' pattern, got %q", cmd)
	}
	// $HOME and backticks should survive both escaping layers (not expanded)
	if !strings.Contains(cmd, "$HOME") {
		t.Errorf("$HOME should be preserved (not expanded) in escaped command, got %q", cmd)
	}
	if !strings.Contains(cmd, "`test`") {
		t.Errorf("backticks should be preserved in escaped command, got %q", cmd)
	}
}

func TestSSHExecTransport_Send_AbsolutePathSkipsLoginShell(t *testing.T) {
	sess := &mockSSHExecSession{stdout: "ok"}
	client := &mockSSHExecClient{sessions: []*mockSSHExecSession{sess}}

	tr := &SSHExecTransport{
		config: SSHExecConfig{Command: "/usr/local/bin/claude", Args: []string{"-p"}},
		client: client,
	}

	_, err := tr.Send(context.Background(), makePayload(t, "hello"))
	if err != nil {
		t.Fatal(err)
	}

	cmd := sess.lastCmd
	// Absolute path: should NOT be wrapped in bash -l -c
	if strings.HasPrefix(cmd, "bash -l -c ") {
		t.Errorf("absolute path command should not use login shell wrapper, got %q", cmd)
	}
	// Should start with the escaped absolute path directly
	if !strings.HasPrefix(cmd, "'/usr/local/bin/claude'") {
		t.Errorf("command should start with escaped absolute path, got %q", cmd)
	}
}

func TestSSHExecTransport_Close(t *testing.T) {
	client := &mockSSHExecClient{}
	tr := &SSHExecTransport{client: client}

	tr.Close()
	if !client.closed {
		t.Error("expected client to be closed")
	}
}

// --- shellEscape tests ---

func TestShellEscape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "'hello'"},
		{"it's", "'it'\\''s'"},
		{"$HOME", "'$HOME'"},
		{"`cmd`", "'`cmd`'"},
		{"a\"b", "'a\"b'"},
		{"", "''"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := shellEscape(tt.input)
			if got != tt.want {
				t.Errorf("shellEscape(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- limitedWriter tests ---

func TestLimitedWriter(t *testing.T) {
	tests := []struct {
		name      string
		limit     int64
		writes    []string
		wantBuf   string
		wantTrunc bool
	}{
		{"within limit", 100, []string{"hello"}, "hello", false},
		{"exact limit", 5, []string{"hello"}, "hello", false},
		{"exceeds on single write", 3, []string{"hello"}, "hel", true},
		{"exceeds across writes", 5, []string{"hel", "lo world"}, "hello", true},
		{"all discarded after limit", 3, []string{"abc", "def", "ghi"}, "abc", true},
		{"zero limit", 0, []string{"hello"}, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			lw := &limitedWriter{w: &buf, n: tt.limit}
			for _, s := range tt.writes {
				lw.Write([]byte(s))
			}
			if got := buf.String(); got != tt.wantBuf {
				t.Errorf("buffer = %q, want %q", got, tt.wantBuf)
			}
			if lw.truncated != tt.wantTrunc {
				t.Errorf("truncated = %v, want %v", lw.truncated, tt.wantTrunc)
			}
		})
	}
}

// NOTE: parseCliResponse tests moved to internal/cliparse/cliparse_test.go

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/luischavesdev/society/internal/models"
	"github.com/luischavesdev/society/internal/thread"
)

// mockRunner implements CommandRunner for testing.
type mockRunner struct {
	stdout string
	stderr string
	err    error
	// recorded args from last call
	lastCmd  string
	lastArgs []string
}

func (m *mockRunner) Run(_ context.Context, cmd string, args []string, _ []string, _ string) (string, string, error) {
	m.lastCmd = cmd
	m.lastArgs = args
	return m.stdout, m.stderr, m.err
}

func newTestExecHandler(t *testing.T, runner *mockRunner) (*ExecHandler, *thread.Store) {
	t.Helper()
	store := thread.NewStore(t.TempDir())
	backend := &models.BackendConfig{
		Command:     "claude",
		Args:        []string{"-p", "--output-format", "json"},
		SessionFlag: "--session-id",
	}
	h := NewExecHandlerWithRunner("claude", backend, store, runner)
	return h, store
}

func makeParams(id, text string) *models.SendTaskParams {
	return &models.SendTaskParams{
		ID: id,
		Message: models.Message{
			Role:  "user",
			Parts: []models.Part{{Type: "text", Text: text}},
		},
	}
}

func TestExecHandler_Success_JSON(t *testing.T) {
	resp, _ := json.Marshal(map[string]string{"result": "Hello from Claude!"})
	runner := &mockRunner{stdout: string(resp)}
	h, _ := newTestExecHandler(t, runner)

	task, err := h.Handle(context.Background(), makeParams("task-1", "say hello"))
	if err != nil {
		t.Fatal(err)
	}
	if task.Status.State != models.TaskStateCompleted {
		t.Errorf("got state %s, want completed", task.Status.State)
	}
	if len(task.Artifacts) == 0 || task.Artifacts[0].Parts[0].Text != "Hello from Claude!" {
		t.Errorf("unexpected response: %+v", task.Artifacts)
	}
	if runner.lastCmd != "claude" {
		t.Errorf("expected command claude, got %s", runner.lastCmd)
	}
}

func TestExecHandler_Success_RawText(t *testing.T) {
	runner := &mockRunner{stdout: "plain text response\n"}
	h, _ := newTestExecHandler(t, runner)

	task, err := h.Handle(context.Background(), makeParams("task-2", "hello"))
	if err != nil {
		t.Fatal(err)
	}
	if task.Artifacts[0].Parts[0].Text != "plain text response" {
		t.Errorf("got %q", task.Artifacts[0].Parts[0].Text)
	}
}

func TestExecHandler_Failure(t *testing.T) {
	runner := &mockRunner{
		stderr: "command not found: claude\n",
		err:    fmt.Errorf("exit status 1"),
	}
	h, _ := newTestExecHandler(t, runner)

	task, err := h.Handle(context.Background(), makeParams("task-3", "hello"))
	if err != nil {
		t.Fatal(err)
	}
	if task.Status.State != models.TaskStateFailed {
		t.Errorf("got state %s, want failed", task.Status.State)
	}
	if task.Status.Message != "command not found: claude" {
		t.Errorf("got message %q", task.Status.Message)
	}
}

func TestExecHandler_ThreadCreation(t *testing.T) {
	resp, _ := json.Marshal(map[string]string{"result": "hi"})
	runner := &mockRunner{stdout: string(resp)}
	h, store := newTestExecHandler(t, runner)

	_, err := h.Handle(context.Background(), makeParams("thread-1", "hello"))
	if err != nil {
		t.Fatal(err)
	}

	th, err := store.Load("thread-1")
	if err != nil {
		t.Fatal(err)
	}
	if th == nil {
		t.Fatal("thread should have been created")
	}
	if th.Agent != "claude" {
		t.Errorf("got agent %q", th.Agent)
	}
	if th.SessionID == "" {
		t.Error("session_id should be set")
	}
	if len(th.Messages) != 2 {
		t.Errorf("got %d messages, want 2", len(th.Messages))
	}
	if th.Messages[0].Role != "user" || th.Messages[0].Text != "hello" {
		t.Errorf("first message: %+v", th.Messages[0])
	}
	if th.Messages[1].Role != "assistant" || th.Messages[1].Text != "hi" {
		t.Errorf("second message: %+v", th.Messages[1])
	}
}

func TestExecHandler_ThreadContinuation(t *testing.T) {
	resp, _ := json.Marshal(map[string]string{"result": "response"})
	runner := &mockRunner{stdout: string(resp)}
	h, store := newTestExecHandler(t, runner)

	// First message creates thread
	_, err := h.Handle(context.Background(), makeParams("thread-2", "first"))
	if err != nil {
		t.Fatal(err)
	}

	th, _ := store.Load("thread-2")
	sessionID := th.SessionID

	// Second message continues thread
	_, err = h.Handle(context.Background(), makeParams("thread-2", "second"))
	if err != nil {
		t.Fatal(err)
	}

	th, _ = store.Load("thread-2")
	if th.SessionID != sessionID {
		t.Error("session_id should be preserved across messages")
	}
	if len(th.Messages) != 4 {
		t.Errorf("got %d messages, want 4", len(th.Messages))
	}

	// Verify --session-id was passed
	found := false
	for i, arg := range runner.lastArgs {
		if arg == "--session-id" && i+1 < len(runner.lastArgs) && runner.lastArgs[i+1] == sessionID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("--session-id %s not found in args: %v", sessionID, runner.lastArgs)
	}
}

func TestExecHandler_MultipleTextParts(t *testing.T) {
	runner := &mockRunner{stdout: "ok"}
	h, _ := newTestExecHandler(t, runner)

	params := &models.SendTaskParams{
		ID: "multi",
		Message: models.Message{
			Role: "user",
			Parts: []models.Part{
				{Type: "text", Text: "line one"},
				{Type: "text", Text: "line two"},
			},
		},
	}

	_, err := h.Handle(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the combined text was passed
	lastArg := runner.lastArgs[len(runner.lastArgs)-3] // before --session-id <id>
	if lastArg != "line one\nline two" {
		t.Errorf("got combined text %q", lastArg)
	}
}

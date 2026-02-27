package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/luischavesdev/society/internal/models"
	"github.com/luischavesdev/society/internal/thread"
)

// CommandRunner abstracts command execution for testability.
type CommandRunner interface {
	Run(ctx context.Context, cmd string, args []string, env []string, stdin string) (stdout, stderr string, err error)
}

// defaultRunner executes real OS commands.
type defaultRunner struct{}

func (r *defaultRunner) Run(ctx context.Context, cmd string, args []string, env []string, stdin string) (string, string, error) {
	c := exec.CommandContext(ctx, cmd, args...)
	if len(env) > 0 {
		c.Env = append(c.Environ(), env...)
	}
	if stdin != "" {
		c.Stdin = strings.NewReader(stdin)
	}
	var stdout, stderr strings.Builder
	c.Stdout = &stdout
	c.Stderr = &stderr
	err := c.Run()
	return stdout.String(), stderr.String(), err
}

type ExecHandler struct {
	backend *models.BackendConfig
	threads *thread.Store
	agent   string
	runner  CommandRunner
}

func NewExecHandler(agent string, backend *models.BackendConfig, store *thread.Store) *ExecHandler {
	return &ExecHandler{
		backend: backend,
		threads: store,
		agent:   agent,
		runner:  &defaultRunner{},
	}
}

// NewExecHandlerWithRunner creates an ExecHandler with a custom CommandRunner (for testing).
func NewExecHandlerWithRunner(agent string, backend *models.BackendConfig, store *thread.Store, runner CommandRunner) *ExecHandler {
	return &ExecHandler{
		backend: backend,
		threads: store,
		agent:   agent,
		runner:  runner,
	}
}

func (h *ExecHandler) Handle(ctx context.Context, params *models.SendTaskParams) (*models.Task, error) {
	// Extract user text
	var userText string
	for _, p := range params.Message.Parts {
		if p.Type == "text" {
			if userText != "" {
				userText += "\n"
			}
			userText += p.Text
		}
	}

	// Look up or create thread
	th, err := h.threads.Load(params.ID)
	if err != nil {
		return nil, fmt.Errorf("loading thread: %w", err)
	}
	if th == nil {
		th = &thread.Thread{
			ID:        params.ID,
			Agent:     h.agent,
			SessionID: uuid.New().String(),
			CreatedAt: time.Now(),
		}
	}

	// Build command args
	args := make([]string, len(h.backend.Args))
	copy(args, h.backend.Args)
	args = append(args, userText)
	if h.backend.SessionFlag != "" && th.SessionID != "" {
		args = append(args, h.backend.SessionFlag, th.SessionID)
	}

	// Execute
	stdout, stderr, err := h.runner.Run(ctx, h.backend.Command, args, h.backend.Env, "")
	if err != nil {
		// Save thread even on failure (preserves conversation history)
		th.Messages = append(th.Messages, thread.Entry{Role: "user", Text: userText})
		_ = h.threads.Save(th)

		errMsg := strings.TrimSpace(stderr)
		if errMsg == "" {
			errMsg = err.Error()
		}
		return &models.Task{
			ID:     params.ID,
			Status: models.TaskStatus{State: models.TaskStateFailed, Message: errMsg},
		}, nil
	}

	// Parse response
	response := parseResponse(stdout)

	// Update thread
	th.Messages = append(th.Messages,
		thread.Entry{Role: "user", Text: userText},
		thread.Entry{Role: "assistant", Text: response},
	)
	if err := h.threads.Save(th); err != nil {
		return nil, fmt.Errorf("saving thread: %w", err)
	}

	return &models.Task{
		ID:     params.ID,
		Status: models.TaskStatus{State: models.TaskStateCompleted},
		Artifacts: []models.Artifact{
			{Parts: []models.Part{{Type: "text", Text: response}}},
		},
	}, nil
}

// parseResponse tries to extract the result from Claude's JSON output,
// falling back to raw stdout for non-JSON backends.
func parseResponse(stdout string) string {
	stdout = strings.TrimSpace(stdout)

	// Try Claude JSON format: {"result": "..."}
	var claudeResp struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout), &claudeResp); err == nil && claudeResp.Result != "" {
		return claudeResp.Result
	}

	return stdout
}

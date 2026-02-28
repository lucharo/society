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
	backend      *models.BackendConfig
	threads      *thread.Store
	agent        string
	systemPrompt string
	runner       CommandRunner
}

func NewExecHandler(agent string, backend *models.BackendConfig, systemPrompt string, store *thread.Store) *ExecHandler {
	return &ExecHandler{
		backend:      backend,
		threads:      store,
		agent:        agent,
		systemPrompt: systemPrompt,
		runner:       &defaultRunner{},
	}
}

// NewExecHandlerWithRunner creates an ExecHandler with a custom CommandRunner (for testing).
func NewExecHandlerWithRunner(agent string, backend *models.BackendConfig, systemPrompt string, store *thread.Store, runner CommandRunner) *ExecHandler {
	return &ExecHandler{
		backend:      backend,
		threads:      store,
		agent:        agent,
		systemPrompt: systemPrompt,
		runner:       runner,
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

	// Build command args: treat as follow-up only if there's a successful exchange
	// (has assistant reply). A thread with only user messages means the previous
	// attempt failed before the backend created a session.
	isFollowUp := false
	for _, m := range th.Messages {
		if m.Role == "assistant" {
			isFollowUp = true
			break
		}
	}
	args := make([]string, len(h.backend.Args))
	copy(args, h.backend.Args)
	if h.systemPrompt != "" && h.backend.SystemPromptFlag != "" {
		args = append(args, h.backend.SystemPromptFlag, h.systemPrompt)
	}
	args = append(args, userText)
	if isFollowUp && h.backend.ResumeFlag != "" && th.SessionID != "" {
		// Follow-up: resume existing session
		args = append(args, h.backend.ResumeFlag, th.SessionID)
	} else if h.backend.SessionFlag != "" && th.SessionID != "" {
		// First message: create session with specific ID
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
	output := parseResponse(stdout)

	// Update thread
	th.Messages = append(th.Messages,
		thread.Entry{Role: "user", Text: userText},
		thread.Entry{Role: "assistant", Text: output.Result},
	)
	if err := h.threads.Save(th); err != nil {
		return nil, fmt.Errorf("saving thread: %w", err)
	}

	task := &models.Task{
		ID:     params.ID,
		Status: models.TaskStatus{State: models.TaskStateCompleted},
		Artifacts: []models.Artifact{
			{Parts: []models.Part{{Type: "text", Text: output.Result}}},
		},
	}
	if output.Verbose != nil {
		task.Artifacts = append(task.Artifacts, models.Artifact{
			Name:  "trace",
			Parts: []models.Part{{Type: "data", Data: output.Verbose}},
		})
	}
	return task, nil
}

// cliOutput holds the parsed result from CLI output.
type cliOutput struct {
	Result  string          // extracted result text
	Verbose json.RawMessage // filtered verbose events (nil if not verbose)
}

// parseResponse extracts text from CLI output, handling both plain
// JSON ({"result":"..."}) and verbose array format ([{type:"system",...}, ...]).
// For verbose output, system/init entries are filtered out and the remaining
// events (tool calls, usage, cost) are preserved in Verbose.
func parseResponse(stdout string) cliOutput {
	stdout = strings.TrimSpace(stdout)

	// Try single-object format: {"result": "..."}
	var claudeResp struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout), &claudeResp); err == nil && claudeResp.Result != "" {
		return cliOutput{Result: claudeResp.Result}
	}

	// Try verbose array format: [{type: "system", ...}, ...]
	var events []json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &events); err == nil && len(events) > 0 {
		var filtered []json.RawMessage
		var result string
		for _, ev := range events {
			var entry struct {
				Type   string `json:"type"`
				Result string `json:"result"`
			}
			if json.Unmarshal(ev, &entry) != nil {
				continue
			}
			if entry.Type == "system" {
				continue
			}
			filtered = append(filtered, ev)
			if entry.Type == "result" && entry.Result != "" {
				result = entry.Result
			}
		}
		if result != "" {
			verbose, _ := json.Marshal(filtered)
			return cliOutput{Result: result, Verbose: verbose}
		}
	}

	return cliOutput{Result: stdout}
}

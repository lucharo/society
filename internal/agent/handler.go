package agent

import (
	"context"
	"fmt"

	"github.com/luischavesdev/society/internal/models"
	"github.com/luischavesdev/society/internal/thread"
)

type Handler interface {
	Handle(ctx context.Context, params *models.SendTaskParams) (*models.Task, error)
}

func NewHandler(cfg *models.AgentConfig) (Handler, error) {
	switch cfg.Handler {
	case "echo":
		return &EchoHandler{}, nil
	case "greeter":
		return &GreeterHandler{}, nil
	case "exec":
		if cfg.Backend == nil {
			return nil, fmt.Errorf("exec handler requires backend config")
		}
		return NewExecHandler(cfg.Name, cfg.Backend, thread.DefaultStore()), nil
	default:
		return nil, fmt.Errorf("unknown handler: %s", cfg.Handler)
	}
}

// EchoHandler returns input text as a completed task artifact.
type EchoHandler struct{}

func (h *EchoHandler) Handle(_ context.Context, params *models.SendTaskParams) (*models.Task, error) {
	return &models.Task{
		ID:     params.ID,
		Status: models.TaskStatus{State: models.TaskStateCompleted},
		Artifacts: []models.Artifact{
			{Parts: params.Message.Parts},
		},
	}, nil
}

// GreeterHandler prepends "Hello! You said: " to each text part.
type GreeterHandler struct{}

func (h *GreeterHandler) Handle(_ context.Context, params *models.SendTaskParams) (*models.Task, error) {
	var parts []models.Part
	for _, p := range params.Message.Parts {
		if p.Type == "text" {
			parts = append(parts, models.Part{
				Type: "text",
				Text: "Hello! You said: " + p.Text,
			})
		} else {
			parts = append(parts, p)
		}
	}
	return &models.Task{
		ID:     params.ID,
		Status: models.TaskStatus{State: models.TaskStateCompleted},
		Artifacts: []models.Artifact{
			{Parts: parts},
		},
	}, nil
}

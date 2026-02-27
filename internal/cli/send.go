package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/luischavesdev/society/internal/agent"
	"github.com/luischavesdev/society/internal/client"
	"github.com/luischavesdev/society/internal/models"
	"github.com/luischavesdev/society/internal/registry"
)

func Send(registryPath, name, message string, out io.Writer, threadID ...string) error {
	reg, err := registry.Load(registryPath)
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}

	card, err := reg.Get(name)
	if err != nil {
		return fmt.Errorf("agent not found: %w", err)
	}

	// Spawn-on-demand: agent has config_path, handle locally
	if card.ConfigPath != "" {
		return sendLocal(card, message, out, threadID...)
	}

	// Remote: use transport
	return sendRemote(reg, name, message, out, threadID...)
}

func sendLocal(card models.AgentCard, message string, out io.Writer, threadID ...string) error {
	cfg, err := agent.LoadConfig(card.ConfigPath)
	if err != nil {
		return fmt.Errorf("loading agent config: %w", err)
	}

	h, err := agent.NewHandler(cfg)
	if err != nil {
		return err
	}

	tid := ""
	if len(threadID) > 0 {
		tid = threadID[0]
	}
	if tid == "" {
		tid = fmt.Sprintf("%x", time.Now().UnixNano())
	}

	params := &models.SendTaskParams{
		ID: tid,
		Message: models.Message{
			Role:  "user",
			Parts: []models.Part{{Type: "text", Text: message}},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	task, err := h.Handle(ctx, params)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "\n  Thread %s\n", task.ID)
	fmt.Fprintf(out, "  Status: %s\n", task.Status.State)
	if task.Status.Message != "" {
		fmt.Fprintf(out, "  Error: %s\n", task.Status.Message)
	}
	for _, a := range task.Artifacts {
		for _, p := range a.Parts {
			if p.Type == "text" {
				fmt.Fprintf(out, "  %s\n", strings.TrimSpace(p.Text))
			}
		}
	}
	return nil
}

func sendRemote(reg *registry.Registry, name, message string, out io.Writer, threadID ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c := client.New(reg)
	task, err := c.Send(ctx, name, message, threadID...)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "\n  Thread %s\n", task.ID)
	fmt.Fprintf(out, "  Status: %s\n", task.Status.State)
	for _, a := range task.Artifacts {
		for _, p := range a.Parts {
			if p.Type == "text" {
				fmt.Fprintf(out, "  %s\n", strings.TrimSpace(p.Text))
			}
		}
	}
	return nil
}

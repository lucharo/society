package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/luischavesdev/society/internal/client"
	"github.com/luischavesdev/society/internal/registry"
)

func Send(registryPath, name, message string, out io.Writer, threadID ...string) error {
	reg, err := registry.Load(registryPath)
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	c := client.New(reg)
	task, err := c.Send(ctx, name, message, threadID...)
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

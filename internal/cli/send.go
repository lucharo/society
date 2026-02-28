package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/luischavesdev/society/internal/client"
	"github.com/luischavesdev/society/internal/registry"
)

func Send(registryPath, name, message string, out io.Writer, showTrace bool, threadID ...string) error {
	reg, err := registry.Load(registryPath)
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}

	if _, err := reg.Get(name); err != nil {
		return fmt.Errorf("agent %q not found — run 'society list' to see registered agents or 'society onboard' to add one", name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	c := client.New(reg)
	task, err := c.Send(ctx, name, message, threadID...)
	if err != nil {
		return err
	}

	// Print response
	for _, a := range task.Artifacts {
		for _, p := range a.Parts {
			if p.Type == "text" {
				fmt.Fprintln(out, strings.TrimSpace(p.Text))
			}
		}
	}

	// Print trace data if requested
	if showTrace {
		for _, a := range task.Artifacts {
			if a.Name == "trace" {
				for _, p := range a.Parts {
					if p.Type == "data" && p.Data != nil {
						raw, err := json.MarshalIndent(p.Data, "", "  ")
						if err == nil {
							fmt.Fprintf(out, "\n%s--- trace ---%s\n%s\n", dim, reset, raw)
						}
					}
				}
			}
		}
	}

	// Print thread/status info in dim
	if task.Status.State == "failed" {
		fmt.Fprintf(out, "\n%s✗ %s%s\n", red, task.Status.Message, reset)
	}
	if task.ID != "" {
		fmt.Fprintf(out, "\n%sthread: %s%s\n", dim, task.ID, reset)
	}

	return nil
}

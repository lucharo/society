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

func Ping(registryPath, name string, out io.Writer) error {
	reg, err := registry.Load(registryPath)
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}

	card, err := reg.Get(name)
	if err != nil {
		return fmt.Errorf("agent %q not found — run 'society list' to see registered agents or 'society onboard' to add one", name)
	}

	transport := "http"
	if card.Transport != nil {
		transport = card.Transport.Type
	}
	fmt.Fprintf(out, "\n  Connecting to %s%s%s via %s...\n", bold, name, reset, transport)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	c := client.New(reg)
	result, err := c.Ping(ctx, name)
	elapsed := time.Since(start)

	if err != nil {
		fmt.Fprintf(out, "  %s✗ Failed%s: %v\n", "\033[31m", reset, err)
		return err
	}

	var skillIDs []string
	for _, s := range result.Skills {
		skillIDs = append(skillIDs, s.ID)
	}

	fmt.Fprintf(out, "  %s✓ %s is responding%s %s(%dms)%s\n", green, result.Name, reset, dim, elapsed.Milliseconds(), reset)
	if len(skillIDs) > 0 {
		fmt.Fprintf(out, "  Skills: %s\n", strings.Join(skillIDs, ", "))
	}
	return nil
}

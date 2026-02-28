package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/luischavesdev/society/internal/registry"
)

func List(registryPath string, out io.Writer) error {
	reg, err := registry.Load(registryPath)
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}

	agents := reg.List()
	if len(agents) == 0 {
		fmt.Fprintf(out, "\nNo agents registered. Run %ssociety onboard%s to get started.\n", bold, reset)
		return nil
	}

	fmt.Fprintln(out)
	fmt.Fprintf(out, "  %s%-14s %-10s %-34s %s%s\n", bold, "NAME", "TRANSPORT", "ENDPOINT", "SKILLS", reset)
	fmt.Fprintf(out, "  %-14s %-10s %-34s %s\n", "────", "─────────", "────────", "──────")

	for _, a := range agents {
		transport := "http"
		endpoint := a.URL
		if a.Transport != nil {
			transport = a.Transport.Type
			cfg := a.Transport.Config
			switch transport {
			case "ssh":
				endpoint = fmt.Sprintf("%s@%s:%s → :%s",
					cfg["user"], cfg["host"], cfg["port"], cfg["forward_port"])
			case "ssh-exec":
				endpoint = fmt.Sprintf("%s@%s → %s",
					cfg["user"], cfg["host"], cfg["command"])
			case "docker":
				endpoint = fmt.Sprintf("%s:%s", cfg["container"], cfg["agent_port"])
			case "stdio":
				cmd := cfg["command"]
				if args := cfg["args"]; args != "" {
					cmd += " " + args
				}
				endpoint = cmd
			}
		}

		var skillIDs []string
		for _, s := range a.Skills {
			skillIDs = append(skillIDs, s.ID)
		}

		fmt.Fprintf(out, "  %-14s %s%-10s%s %-34s %s\n",
			a.Name, dim, transport, reset, truncate(endpoint, 34), strings.Join(skillIDs, ", "))
	}

	fmt.Fprintf(out, "\n  %s%d agents registered%s\n", dim, len(agents), reset)
	return nil
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-3]) + "..."
}

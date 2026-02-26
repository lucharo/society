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
		fmt.Fprintln(out, "  No agents registered")
		return nil
	}

	fmt.Fprintln(out)
	fmt.Fprintf(out, "  %-12s %-12s %-30s %s\n", "NAME", "TRANSPORT", "ENDPOINT", "SKILLS")

	for _, a := range agents {
		transport := "http"
		endpoint := a.URL
		if a.Transport != nil {
			transport = a.Transport.Type
			switch transport {
			case "ssh":
				cfg := a.Transport.Config
				endpoint = fmt.Sprintf("ssh://%s@%s:%s→:%s",
					cfg["user"], cfg["host"], cfg["port"], cfg["forward_port"])
			case "docker":
				cfg := a.Transport.Config
				endpoint = fmt.Sprintf("docker://%s:%s", cfg["container"], cfg["agent_port"])
			case "stdio":
				cfg := a.Transport.Config
				cmd := cfg["command"]
				if args := cfg["args"]; args != "" {
					cmd += " " + args
				}
				endpoint = fmt.Sprintf("stdio://%s", cmd)
			}
		}

		var skillIDs []string
		for _, s := range a.Skills {
			skillIDs = append(skillIDs, s.ID)
		}

		fmt.Fprintf(out, "  %-12s %-12s %-30s %s\n",
			a.Name, transport, truncate(endpoint, 30), strings.Join(skillIDs, ", "))
	}

	fmt.Fprintf(out, "\n  %d agents registered\n", len(agents))
	return nil
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-3]) + "..."
}

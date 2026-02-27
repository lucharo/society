package cli

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/luischavesdev/society/internal/models"
	"github.com/luischavesdev/society/internal/registry"
)

// OnboardAuto scans for available agents and lets the user pick which to register.
func OnboardAuto(registryPath string, in io.Reader, out io.Writer) error {
	r := bufio.NewReader(in)

	fmt.Fprintf(out, "\n%sScanning for agents...%s\n", bold, reset)

	candidates := ScanAll()

	// Load registry to filter already-registered agents
	reg, err := registry.Load(registryPath)
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}

	// Group by source for display
	cliCandidates := filterBySource(candidates, "cli")
	dockerCandidates := filterBySource(candidates, "docker")
	sshCandidates := filterBySource(candidates, "ssh")
	a2aCandidates := filterBySource(candidates, "a2a")

	if len(cliCandidates) > 0 {
		fmt.Fprintf(out, "  %s✓%s %d CLIs: %s\n", green, reset, len(cliCandidates), candidateNames(cliCandidates))
	}
	if len(dockerCandidates) > 0 {
		fmt.Fprintf(out, "  %s✓%s %d Docker containers: %s\n", green, reset, len(dockerCandidates), candidateNames(dockerCandidates))
	}
	if len(sshCandidates) > 0 {
		fmt.Fprintf(out, "  %s✓%s %d SSH hosts: %s\n", green, reset, len(sshCandidates), candidateNames(sshCandidates))
	}
	if len(a2aCandidates) > 0 {
		fmt.Fprintf(out, "  %s✓%s %d A2A agents: %s\n", green, reset, len(a2aCandidates), candidateNames(a2aCandidates))
	}

	if len(candidates) == 0 {
		fmt.Fprintf(out, "\nNo agents detected. Use '%ssociety onboard --manual%s' for manual setup.\n", bold, reset)
		return nil
	}

	// Filter out already-registered
	var available []Candidate
	for _, c := range candidates {
		if !reg.Has(c.Name) {
			available = append(available, c)
		}
	}

	if len(available) == 0 {
		fmt.Fprintln(out, "\nAll detected agents are already registered.")
		return nil
	}

	// Display numbered list
	fmt.Fprintf(out, "\n%sAvailable agents:%s\n\n", bold, reset)
	for i, c := range available {
		sourceLabel := strings.ToUpper(c.Source)
		fmt.Fprintf(out, "  %s%d.%s %-20s %s%-8s%s %s%s%s\n",
			cyan, i+1, reset,
			c.Name,
			dim, sourceLabel, reset,
			dim, c.Description, reset)
	}

	fmt.Fprintln(out)
	selection := prompt(r, out, "Select agents to register (comma-separated numbers, or 'all')", "all")

	var selected []Candidate
	if strings.ToLower(strings.TrimSpace(selection)) == "all" {
		selected = available
	} else {
		for _, s := range strings.Split(selection, ",") {
			s = strings.TrimSpace(s)
			idx, err := strconv.Atoi(s)
			if err != nil || idx < 1 || idx > len(available) {
				fmt.Fprintf(out, "  skipping invalid selection: %s\n", s)
				continue
			}
			selected = append(selected, available[idx-1])
		}
	}

	if len(selected) == 0 {
		fmt.Fprintln(out, "\nNo agents selected.")
		return nil
	}

	// Register each selected candidate
	fmt.Fprintln(out)
	registered := 0
	for _, c := range selected {
		card, needsInput := candidateToCard(c)

		// SSH candidates need the remote agent port
		if needsInput && c.Source == "ssh" {
			agentPort := prompt(r, out, fmt.Sprintf("Agent port on %s (remote host)", c.Name), "8080")
			card.Transport.Config["forward_port"] = agentPort
			card.URL = fmt.Sprintf("http://localhost:%s", agentPort)
		}

		if err := reg.Add(card); err != nil {
			fmt.Fprintf(out, "  skipping %s: %v\n", c.Name, err)
			continue
		}
		fmt.Fprintf(out, "  %s✓%s %s\n", green, reset, c.Name)
		registered++
	}

	if registered == 0 {
		return nil
	}

	if err := reg.Save(); err != nil {
		return fmt.Errorf("saving registry: %w", err)
	}

	fmt.Fprintf(out, "\n%s%d agent(s) added to registry.%s\n", green, registered, reset)
	return nil
}

func filterBySource(candidates []Candidate, source string) []Candidate {
	var result []Candidate
	for _, c := range candidates {
		if c.Source == source {
			result = append(result, c)
		}
	}
	return result
}

func candidateNames(candidates []Candidate) string {
	names := make([]string, len(candidates))
	for i, c := range candidates {
		names[i] = c.Name
	}
	return strings.Join(names, ", ")
}

func candidateToCard(c Candidate) (models.AgentCard, bool) {
	card := models.AgentCard{
		Name:        c.Name,
		Description: c.Description,
	}

	needsInput := false

	switch c.Transport {
	case "http":
		card.URL = c.Config["url"]

	case "ssh":
		card.URL = fmt.Sprintf("http://localhost:%s", c.Config["forward_port"])
		card.Transport = &models.TransportConfig{
			Type: "ssh",
			Config: map[string]string{
				"host":         c.Config["host"],
				"user":         c.Config["user"],
				"port":         c.Config["port"],
				"key_path":     c.Config["key_path"],
				"forward_port": c.Config["forward_port"],
			},
		}
		needsInput = true

	case "docker":
		card.URL = fmt.Sprintf("http://%s:%s", c.Config["container"], c.Config["agent_port"])
		card.Transport = &models.TransportConfig{
			Type: "docker",
			Config: map[string]string{
				"container":   c.Config["container"],
				"network":     c.Config["network"],
				"agent_port":  c.Config["agent_port"],
				"socket_path": c.Config["socket_path"],
			},
		}

	case "stdio":
		card.URL = fmt.Sprintf("stdio://%s", c.Config["command"])
		card.Transport = &models.TransportConfig{
			Type: "stdio",
			Config: map[string]string{
				"command": c.Config["command"],
			},
		}
	}

	return card, needsInput
}

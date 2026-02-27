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

	fmt.Fprintln(out, "\nScanning for agents...")

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
		fmt.Fprintf(out, "  found %d CLIs: %s\n", len(cliCandidates), candidateNames(cliCandidates))
	}
	if len(dockerCandidates) > 0 {
		fmt.Fprintf(out, "  found %d Docker containers: %s\n", len(dockerCandidates), candidateNames(dockerCandidates))
	}
	if len(sshCandidates) > 0 {
		fmt.Fprintf(out, "  found %d SSH hosts: %s\n", len(sshCandidates), candidateNames(sshCandidates))
	}
	if len(a2aCandidates) > 0 {
		fmt.Fprintf(out, "  found %d A2A agents: %s\n", len(a2aCandidates), candidateNames(a2aCandidates))
	}

	if len(candidates) == 0 {
		fmt.Fprintln(out, "\n  No agents detected. Use 'society onboard' for manual setup.")
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
		fmt.Fprintln(out, "\n  All detected agents are already registered.")
		return nil
	}

	// Display numbered list
	fmt.Fprintln(out, "\nAvailable agents:")
	for i, c := range available {
		sourceLabel := strings.ToUpper(c.Source)
		fmt.Fprintf(out, "  %d. %s (%s) — %s\n", i+1, c.Name, sourceLabel, c.Description)
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
		fmt.Fprintln(out, "\n  No agents selected.")
		return nil
	}

	// Register each selected candidate
	registered := 0
	for _, c := range selected {
		card, needsInput := candidateToCard(c)

		// SSH candidates need the remote agent port
		if needsInput && c.Source == "ssh" {
			agentPort := prompt(r, out, fmt.Sprintf("  Agent port on %s (remote host)", c.Name), "8080")
			card.Transport.Config["forward_port"] = agentPort
			card.URL = fmt.Sprintf("http://localhost:%s", agentPort)
		}

		if err := reg.Add(card); err != nil {
			fmt.Fprintf(out, "  skipping %s: %v\n", c.Name, err)
			continue
		}
		fmt.Fprintf(out, "  registered %s\n", c.Name)
		registered++
	}

	if registered == 0 {
		return nil
	}

	if err := reg.Save(); err != nil {
		return fmt.Errorf("saving registry: %w", err)
	}

	fmt.Fprintf(out, "\n  %d agent(s) added to registry.\n", registered)
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

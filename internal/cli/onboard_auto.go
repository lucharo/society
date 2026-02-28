package cli

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/luischavesdev/society/internal/models"
	"github.com/luischavesdev/society/internal/registry"
)

// OnboardAuto scans for available agents and lets the user pick which to register.
func OnboardAuto(registryPath string, opts ScanOptions, in io.Reader, out io.Writer) error {
	r := bufio.NewReader(in)

	if opts.Deep {
		fmt.Fprintf(out, "\n%sScanning for agents (deep mode — probing SSH/Docker hosts)...%s\n", bold, reset)
	} else {
		fmt.Fprintf(out, "\n%sScanning for agents...%s\n", bold, reset)
	}

	opts.Progress = func(msg string) {
		fmt.Fprintf(out, "  %s%s%s\n", dim, msg, reset)
	}
	candidates := ScanAll(opts)

	// Load registry to filter already-registered agents
	reg, err := registry.Load(registryPath)
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}

	// Show scan summary
	cliCandidates := filterBySource(candidates, "cli")
	dockerCandidates := filterBySource(candidates, "docker")
	sshCandidates := filterBySource(candidates, "ssh")
	a2aCandidates := filterBySource(candidates, "a2a")

	if len(cliCandidates) > 0 {
		fmt.Fprintf(out, "  %s✓%s Found %d CLI tools: %s\n", green, reset, len(cliCandidates), candidateNames(cliCandidates))
	}
	if len(dockerCandidates) > 0 {
		v := countVerified(dockerCandidates)
		if v > 0 {
			fmt.Fprintf(out, "  %s✓%s Found %d Docker containers (%d with live agents): %s\n", green, reset, len(dockerCandidates), v, candidateNames(dockerCandidates))
		} else {
			fmt.Fprintf(out, "  %s✓%s Found %d Docker containers: %s\n", green, reset, len(dockerCandidates), candidateNames(dockerCandidates))
		}
	}
	if len(sshCandidates) > 0 {
		v := countVerified(sshCandidates)
		if v > 0 {
			fmt.Fprintf(out, "  %s✓%s Found %d SSH hosts (%d with live agents): %s\n", green, reset, len(sshCandidates), v, candidateNames(sshCandidates))
		} else {
			fmt.Fprintf(out, "  %s✓%s Found %d SSH hosts: %s\n", green, reset, len(sshCandidates), candidateNames(sshCandidates))
		}
	}
	if len(a2aCandidates) > 0 {
		fmt.Fprintf(out, "  %s✓%s Found %d A2A agents: %s\n", green, reset, len(a2aCandidates), candidateNames(a2aCandidates))
	}
	sshCLICandidates := filterBySource(candidates, "ssh-cli")
	if len(sshCLICandidates) > 0 {
		fmt.Fprintf(out, "  %s✓%s Found %d remote CLI tools via SSH: %s\n", green, reset, len(sshCLICandidates), candidateNames(sshCLICandidates))
	}
	tailscaleCandidates := filterBySource(candidates, "tailscale")
	if len(tailscaleCandidates) > 0 {
		fmt.Fprintf(out, "  %s✓%s Found %d Tailscale peers: %s\n", green, reset, len(tailscaleCandidates), candidateNames(tailscaleCandidates))
	}

	if len(candidates) == 0 {
		fmt.Fprintf(out, "  No agents detected.\n\n")
		fmt.Fprintf(out, "Use %ssociety onboard --manual%s to add one manually.\n", bold, reset)
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
		fmt.Fprintf(out, "\nAll detected agents are already registered. Run %ssociety list%s to see them.\n", bold, reset)
		return nil
	}

	// Group SSH candidates that point to the same machine
	singles, groups := GroupByMachine(available)

	// Filter out groups where all candidates are already registered
	var availableGroups []MachineGroup
	for _, g := range groups {
		var unregistered []Candidate
		for _, c := range g.Candidates {
			if !reg.Has(c.Name) {
				unregistered = append(unregistered, c)
			}
		}
		if len(unregistered) > 0 {
			g.Candidates = unregistered
			availableGroups = append(availableGroups, g)
		}
	}

	// Build a unified display list: each item is either a single candidate or a group
	type displayItem struct {
		candidate *Candidate    // non-nil for singles
		group     *MachineGroup // non-nil for groups
	}
	var items []displayItem
	for i := range singles {
		items = append(items, displayItem{candidate: &singles[i]})
	}
	for i := range availableGroups {
		items = append(items, displayItem{group: &availableGroups[i]})
	}

	if len(items) == 0 {
		fmt.Fprintf(out, "\nAll detected agents are already registered. Run %ssociety list%s to see them.\n", bold, reset)
		return nil
	}

	// Display numbered list with transport details
	fmt.Fprintf(out, "\n%sAvailable to register:%s\n\n", bold, reset)
	for i, item := range items {
		if item.candidate != nil {
			transport := candidateTransportDesc(*item.candidate)
			fmt.Fprintf(out, "  %s%d.%s %s%-16s%s %s\n",
				cyan, i+1, reset,
				bold, item.candidate.Name, reset,
				transport)
		} else {
			fmt.Fprintf(out, "  %s%d.%s %s%-16s%s %s%d routes available%s\n",
				cyan, i+1, reset,
				bold, item.group.Name, reset,
				dim, len(item.group.Candidates), reset)
		}
	}

	fmt.Fprintln(out)
	selection := prompt(r, out, "Register which agents? (numbers, or 'all')", "all")

	var selectedItems []displayItem
	if strings.ToLower(strings.TrimSpace(selection)) == "all" {
		selectedItems = items
	} else {
		for _, s := range strings.Split(selection, ",") {
			s = strings.TrimSpace(s)
			idx, err := strconv.Atoi(s)
			if err != nil || idx < 1 || idx > len(items) {
				fmt.Fprintf(out, "  skipping invalid selection: %s\n", s)
				continue
			}
			selectedItems = append(selectedItems, items[idx-1])
		}
	}

	if len(selectedItems) == 0 {
		fmt.Fprintln(out, "\nNo agents selected.")
		return nil
	}

	// Resolve groups: prompt user to pick a route for each selected group
	var selected []Candidate
	for _, item := range selectedItems {
		if item.candidate != nil {
			selected = append(selected, *item.candidate)
			continue
		}

		g := item.group
		if len(g.Candidates) == 1 {
			selected = append(selected, g.Candidates[0])
			continue
		}

		// Show route selection sub-prompt
		fmt.Fprintf(out, "\n%s%s%s has %d connection routes:\n", bold, g.Name, reset, len(g.Candidates))
		routes := g.Candidates
		if len(routes) > 26 {
			fmt.Fprintf(out, "  %s(showing first 26 of %d routes)%s\n", dim, len(routes), reset)
			routes = routes[:26]
		}
		labels := "abcdefghijklmnopqrstuvwxyz"
		for i, c := range routes {
			label := string(labels[i])
			desc := candidateRouteDesc(c)
			fmt.Fprintf(out, "  %s%s)%s %-16s %s\n", cyan, label, reset, c.Name, desc)
		}

		options := make([]string, len(routes))
		for i := range routes {
			options[i] = string(labels[i])
		}
		choice := promptChoice(r, out, "Route", options, "a")

		// Find selected route
		picked := 0
		for i, opt := range options {
			if strings.EqualFold(choice, opt) {
				picked = i
				break
			}
		}
		selected = append(selected, g.Candidates[picked])
	}

	// Register each selected candidate, collecting results for summary
	fmt.Fprintln(out)
	type registeredAgent struct {
		name      string
		transport string
		endpoint  string
	}
	var results []registeredAgent

	for _, c := range selected {
		card, needsInput := candidateToCard(c)

		// SSH candidates need the remote agent port (skip if deep scan verified it)
		if needsInput && c.Source == "ssh" && !c.Verified {
			agentPort := prompt(r, out, fmt.Sprintf("Agent port on %s", c.Name), "8080")
			card.Transport.Config["forward_port"] = agentPort
			card.URL = fmt.Sprintf("http://localhost:%s", agentPort)
		}

		if err := reg.Add(card); err != nil {
			fmt.Fprintf(out, "  %sskipping %s: %v%s\n", dim, c.Name, err, reset)
			continue
		}

		transport := transportLabel(c)
		endpoint := card.URL
		results = append(results, registeredAgent{c.Name, transport, endpoint})
	}

	if len(results) == 0 {
		return nil
	}

	if err := reg.Save(); err != nil {
		return fmt.Errorf("saving registry: %w", err)
	}

	// Print summary table
	fmt.Fprintf(out, "\n%sRegistered %d agent(s):%s\n\n", green, len(results), reset)
	fmt.Fprintf(out, "  %-18s %-10s %s\n", "NAME", "TRANSPORT", "ENDPOINT")
	fmt.Fprintf(out, "  %-18s %-10s %s\n", "────", "─────────", "────────")
	for _, r := range results {
		fmt.Fprintf(out, "  %-18s %-10s %s\n", r.name, r.transport, r.endpoint)
	}

	// Next steps
	fmt.Fprintf(out, "\n%sNext steps:%s\n", bold, reset)
	first := results[0]
	fmt.Fprintf(out, "  society send %s \"hello\"    %sSend a message%s\n", first.name, dim, reset)
	fmt.Fprintf(out, "  society ping %s            %sHealth check%s\n", first.name, dim, reset)
	fmt.Fprintf(out, "  society list                    %sSee all agents%s\n", dim, reset)
	fmt.Fprintf(out, "  society daemon start             %sStart all agents%s\n\n", dim, reset)

	return nil
}

// candidateTransportDesc returns a human-readable description of how the agent connects.
func candidateTransportDesc(c Candidate) string {
	switch c.Transport {
	case "stdio":
		return fmt.Sprintf("%sCLI tool → runs as subprocess%s", dim, reset)
	case "ssh":
		host := c.Config["host"]
		user := c.Config["user"]
		if user != "" {
			return fmt.Sprintf("%sSSH → %s@%s%s", dim, user, host, reset)
		}
		return fmt.Sprintf("%sSSH → %s%s", dim, host, reset)
	case "ssh-exec":
		host := c.Config["host"]
		user := c.Config["user"]
		cmd := filepath.Base(c.Config["command"])
		if user != "" {
			return fmt.Sprintf("%sSSH exec → %s@%s (%s)%s", dim, user, host, cmd, reset)
		}
		return fmt.Sprintf("%sSSH exec → %s (%s)%s", dim, host, cmd, reset)
	case "docker":
		container := c.Config["container"]
		return fmt.Sprintf("%sDocker → container %s%s", dim, container, reset)
	case "http":
		url := c.Config["url"]
		return fmt.Sprintf("%sHTTP → %s%s", dim, url, reset)
	default:
		return fmt.Sprintf("%s%s%s", dim, c.Description, reset)
	}
}

// transportLabel returns a short label for the summary table.
func transportLabel(c Candidate) string {
	switch c.Transport {
	case "stdio":
		return "stdio"
	case "ssh":
		return "ssh"
	case "docker":
		return "docker"
	case "http":
		return "http"
	default:
		return c.Transport
	}
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

func countVerified(candidates []Candidate) int {
	n := 0
	for _, c := range candidates {
		if c.Verified {
			n++
		}
	}
	return n
}

// candidateRouteDesc returns a short description of an SSH route for the route picker.
func candidateRouteDesc(c Candidate) string {
	user := c.Config["user"]
	host := c.Config["host"]
	port := c.Config["port"]
	if port == "" {
		port = "22"
	}
	suffix := ""
	if c.Source == "tailscale" {
		suffix = " (Tailscale)"
	}
	if user != "" {
		return fmt.Sprintf("%s%s@%s:%s%s%s", dim, user, host, port, suffix, reset)
	}
	return fmt.Sprintf("%s%s:%s%s%s", dim, host, port, suffix, reset)
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

	case "ssh-exec":
		card.URL = fmt.Sprintf("ssh-exec://%s/%s", c.Config["host"], filepath.Base(c.Config["command"]))
		card.Transport = &models.TransportConfig{
			Type: "ssh-exec",
			Config: map[string]string{
				"host":     c.Config["host"],
				"user":     c.Config["user"],
				"port":     c.Config["port"],
				"key_path": c.Config["key_path"],
				"command":  c.Config["command"],
				"args":     c.Config["args"],
			},
		}
	}

	return card, needsInput
}

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/luischavesdev/society/internal/models"
	"github.com/luischavesdev/society/internal/registry"
)

func Discover(registryPath, rawURL string, in io.Reader, out io.Writer) error {
	card, err := fetchAgentCard(rawURL)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "\n  Found agent:\n")
	fmt.Fprintf(out, "    Name: %s\n", card.Name)
	if card.Description != "" {
		fmt.Fprintf(out, "    Description: %s\n", card.Description)
	}
	if len(card.Skills) > 0 {
		var ids []string
		for _, s := range card.Skills {
			ids = append(ids, s.ID)
		}
		fmt.Fprintf(out, "    Skills: %s\n", strings.Join(ids, ", "))
	}
	if card.URL != "" {
		fmt.Fprintf(out, "    URL: %s\n", card.URL)
	}

	r := bufio.NewReader(in)
	if !promptYN(r, out, "\n  Add to registry?", true) {
		fmt.Fprintln(out, "  Skipped")
		return nil
	}

	// Ask for transport config
	transportType := promptChoice(r, out, "  Transport", []string{"http", "ssh", "docker", "stdio"}, "http")

	if card.URL == "" {
		card.URL = rawURL
	}

	switch transportType {
	case "http":
		// No extra config needed
	case "ssh":
		card.Transport = &models.TransportConfig{
			Type: "ssh",
			Config: map[string]string{
				"host":         prompt(r, out, "  Host", ""),
				"user":         prompt(r, out, "  User", "claude"),
				"port":         prompt(r, out, "  Port", "22"),
				"key_path":     prompt(r, out, "  Key path", "~/.ssh/id_rsa"),
				"forward_port": prompt(r, out, "  Agent port", "8080"),
			},
		}
	case "docker":
		card.Transport = &models.TransportConfig{
			Type: "docker",
			Config: map[string]string{
				"container":   prompt(r, out, "  Container", ""),
				"network":     prompt(r, out, "  Network", "bridge"),
				"agent_port":  prompt(r, out, "  Agent port", "8080"),
				"socket_path": prompt(r, out, "  Socket path", "/var/run/docker.sock"),
			},
		}
	case "stdio":
		card.Transport = &models.TransportConfig{
			Type: "stdio",
			Config: map[string]string{
				"command": prompt(r, out, "  Command", ""),
				"args":    prompt(r, out, "  Args", ""),
			},
		}
	}

	reg, err := registry.Load(registryPath)
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}

	if err := reg.Add(*card); err != nil {
		return err
	}
	if err := reg.Save(); err != nil {
		return err
	}

	fmt.Fprintf(out, "  ✓ Added %q to registry\n", card.Name)
	return nil
}

func fetchAgentCard(rawURL string) (*models.AgentCard, error) {
	httpClient := &http.Client{Timeout: 10 * time.Second}

	// Try .well-known first if URL doesn't end in .json
	urls := []string{rawURL}
	if !strings.HasSuffix(rawURL, ".json") {
		wellKnown := strings.TrimRight(rawURL, "/") + "/.well-known/agent.json"
		urls = []string{wellKnown, rawURL}
	}

	var lastErr error
	for _, u := range urls {
		card, err := tryFetchCard(httpClient, u)
		if err != nil {
			lastErr = err
			continue
		}
		return card, nil
	}

	return nil, fmt.Errorf("discovering agent: %w", lastErr)
}

func tryFetchCard(client *http.Client, u string) (*models.AgentCard, error) {
	resp, err := client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GET %s: status %d", u, resp.StatusCode)
	}

	var card models.AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, fmt.Errorf("parsing response from %s: %w", u, err)
	}

	if card.Name == "" {
		return nil, fmt.Errorf("invalid agent card from %s: missing name", u)
	}

	return &card, nil
}

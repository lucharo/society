package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

// Candidate represents a detected agent that can be onboarded.
type Candidate struct {
	Name        string
	Description string
	Transport   string // "http", "ssh", "docker", "stdio"
	Source      string // "cli", "docker", "ssh", "a2a"
	Config      map[string]string
}

// knownCLIs maps CLI tool names to descriptions.
var knownCLIs = map[string]string{
	"claude":   "Claude Code agent",
	"codex":    "OpenAI Codex agent",
	"ollama":   "Ollama local LLM",
	"aider":    "Aider AI coding assistant",
	"opencode": "OpenCode agent",
	"droid":    "Droid AI agent",
	"goose":    "Goose AI agent",
}

// ScanAll runs all detection functions and returns candidates.
func ScanAll() []Candidate {
	var all []Candidate
	all = append(all, scanCLIs()...)
	all = append(all, scanDocker()...)
	all = append(all, scanSSH()...)
	all = append(all, scanA2A()...)
	return all
}

// scanCLIs checks PATH for known AI CLI tools.
func scanCLIs() []Candidate {
	var candidates []Candidate
	for name, desc := range knownCLIs {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		candidates = append(candidates, Candidate{
			Name:        name,
			Description: desc,
			Transport:   "stdio",
			Source:       "cli",
			Config: map[string]string{
				"command": path,
			},
		})
	}
	return candidates
}

// scanDocker lists running Docker containers via the Docker socket.
func scanDocker() []Candidate {
	socketPath := "/var/run/docker.sock"
	if _, err := os.Stat(socketPath); err != nil {
		return nil
	}

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
		Timeout: 3 * time.Second,
	}

	resp, err := client.Get("http://localhost/containers/json")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil
	}

	var containers []struct {
		ID    string `json:"Id"`
		Names []string
		State string
		Ports []struct {
			PrivatePort int `json:"PrivatePort"`
			PublicPort  int `json:"PublicPort"`
		}
		Image string
	}
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		return nil
	}

	var candidates []Candidate
	for _, c := range containers {
		if c.State != "running" {
			continue
		}

		name := c.ID[:12]
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		port := "8080"
		for _, p := range c.Ports {
			if p.PublicPort > 0 {
				port = fmt.Sprintf("%d", p.PublicPort)
				break
			}
			if p.PrivatePort > 0 {
				port = fmt.Sprintf("%d", p.PrivatePort)
			}
		}

		candidates = append(candidates, Candidate{
			Name:        name,
			Description: fmt.Sprintf("Docker container (%s)", c.Image),
			Transport:   "docker",
			Source:       "docker",
			Config: map[string]string{
				"container":   name,
				"agent_port":  port,
				"socket_path": socketPath,
				"network":     "bridge",
			},
		})
	}
	return candidates
}

// scanSSH parses ~/.ssh/config for known hosts and detects available SSH keys.
func scanSSH() []Candidate {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	sshDir := filepath.Join(home, ".ssh")
	if _, err := os.Stat(sshDir); err != nil {
		return nil
	}

	// Find available SSH keys
	keys := findSSHKeys(sshDir)
	if len(keys) == 0 {
		return nil
	}

	// Parse SSH config for hosts
	configPath := filepath.Join(sshDir, "config")
	hosts := parseSSHConfig(configPath)

	var candidates []Candidate
	for _, h := range hosts {
		keyPath := h.keyPath
		if keyPath == "" && len(keys) > 0 {
			keyPath = keys[0] // default to first available key
		}

		sshUser := h.user
		if sshUser == "" {
			if u, err := user.Current(); err == nil {
				sshUser = u.Username
			}
		}

		port := h.port
		if port == "" {
			port = "22"
		}

		candidates = append(candidates, Candidate{
			Name:        h.name,
			Description: fmt.Sprintf("SSH host %s", h.hostname),
			Transport:   "ssh",
			Source:       "ssh",
			Config: map[string]string{
				"host":         h.hostname,
				"user":         sshUser,
				"port":         port,
				"key_path":     keyPath,
				"forward_port": "8080",
			},
		})
	}
	return candidates
}

type sshHost struct {
	name     string
	hostname string
	user     string
	port     string
	keyPath  string
}

func findSSHKeys(sshDir string) []string {
	entries, err := os.ReadDir(sshDir)
	if err != nil {
		return nil
	}

	var keys []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "id_") && !strings.HasSuffix(name, ".pub") {
			keys = append(keys, filepath.Join(sshDir, name))
		}
	}
	return keys
}

func parseSSHConfig(path string) []sshHost {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var hosts []sshHost
	var current *sshHost

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			// Try tab-separated
			parts = strings.SplitN(line, "\t", 2)
			if len(parts) != 2 {
				continue
			}
		}

		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])

		switch key {
		case "host":
			// Handle multi-value Host (e.g., "Host foo bar")
			// Use only the first name, skip wildcards
			hostNames := strings.Fields(val)
			if len(hostNames) == 0 {
				current = nil
				continue
			}
			firstName := hostNames[0]
			if strings.Contains(firstName, "*") || strings.Contains(firstName, "?") {
				current = nil
				continue
			}
			hosts = append(hosts, sshHost{name: firstName})
			current = &hosts[len(hosts)-1]
		case "match":
			// Skip Match blocks — their directives shouldn't apply to previous Host
			current = nil
		case "hostname":
			if current != nil {
				current.hostname = val
			}
		case "user":
			if current != nil {
				current.user = val
			}
		case "port":
			if current != nil {
				current.port = val
			}
		case "identityfile":
			if current != nil {
				// Expand ~ to home dir
				if strings.HasPrefix(val, "~/") {
					home, _ := os.UserHomeDir()
					val = filepath.Join(home, val[2:])
				}
				current.keyPath = val
			}
		}
	}

	// Filter out hosts without a hostname
	var result []sshHost
	for _, h := range hosts {
		if h.hostname != "" {
			result = append(result, h)
		}
	}
	return result
}

// scanA2A probes common local ports for A2A agents.
func scanA2A() []Candidate {
	var candidates []Candidate
	ports := []int{8001, 8002, 8003, 8004, 8005, 8006, 8007, 8008, 8009, 8010}

	client := &http.Client{Timeout: 200 * time.Millisecond}

	for _, port := range ports {
		if c, ok := probeA2APort(client, port); ok {
			candidates = append(candidates, c)
		}
	}
	return candidates
}

func probeA2APort(client *http.Client, port int) (Candidate, bool) {
	url := fmt.Sprintf("http://localhost:%d/.well-known/agent-card.json", port)
	resp, err := client.Get(url)
	if err != nil {
		return Candidate{}, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return Candidate{}, false
	}

	var card struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return Candidate{}, false
	}

	name := card.Name
	if name == "" {
		name = fmt.Sprintf("agent-%d", port)
	}

	return Candidate{
		Name:        name,
		Description: card.Description,
		Transport:   "http",
		Source:      "a2a",
		Config: map[string]string{
			"url":  fmt.Sprintf("http://localhost:%d", port),
			"port": fmt.Sprintf("%d", port),
		},
	}, true
}

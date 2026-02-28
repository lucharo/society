package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// sshHostKeyCallback returns an ssh.HostKeyCallback that verifies against
// ~/.ssh/known_hosts. Falls back to InsecureIgnoreHostKey if the file is missing.
func sshHostKeyCallback() ssh.HostKeyCallback {
	home, err := os.UserHomeDir()
	if err != nil {
		slog.Warn("ssh scan: cannot determine home directory, host key verification disabled")
		return ssh.InsecureIgnoreHostKey()
	}
	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")
	cb, err := knownhosts.New(knownHostsPath)
	if err != nil {
		slog.Warn("ssh scan: cannot load known_hosts, host key verification disabled",
			"path", knownHostsPath, "err", err)
		return ssh.InsecureIgnoreHostKey()
	}
	return cb
}

// ScanOptions configures how agent scanning behaves.
type ScanOptions struct {
	Deep bool // Probe SSH/Docker hosts for live A2A agents
}

// Candidate represents a detected agent that can be onboarded.
type Candidate struct {
	Name        string
	Description string
	Transport   string // "http", "ssh", "ssh-exec", "docker", "stdio"
	Source      string // "cli", "docker", "ssh", "ssh-cli", "a2a"
	Config      map[string]string
	Verified    bool // true = confirmed live A2A agent via probe
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

// knownCLIArgs maps CLI tool names to default arguments for remote execution.
var knownCLIArgs = map[string]string{
	"claude": "-p --output-format json",
	"codex":  "--quiet",
}

// ScanAll runs all detection functions and returns candidates.
func ScanAll(opts ScanOptions) []Candidate {
	var all []Candidate
	all = append(all, scanCLIs()...)
	all = append(all, scanDocker()...)
	all = append(all, scanSSH()...)
	all = append(all, scanA2A()...)
	if opts.Deep {
		deep := append(scanDockerDeep(), scanSSHDeep()...)
		deep = append(deep, scanSSHDeepCLIs()...)
		all = dedup(all, deep)
	}
	return all
}

// dedup merges deep (verified) candidates into the shallow list.
// Deep candidates replace shallow ones with matching transport and host/container.
func dedup(shallow, deep []Candidate) []Candidate {
	replaced := make(map[int]bool)
	for _, d := range deep {
		for i, s := range shallow {
			if s.Transport != d.Transport {
				continue
			}
			match := false
			switch s.Transport {
			case "docker":
				match = s.Config["container"] == d.Config["container"]
			case "ssh":
				match = s.Config["host"] == d.Config["host"]
			}
			if match {
				replaced[i] = true
			}
		}
	}

	var result []Candidate
	for i, s := range shallow {
		if !replaced[i] {
			result = append(result, s)
		}
	}
	result = append(result, deep...)
	return result
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
	name, desc, ok := probeA2AEndpoint(client, "localhost", port)
	if !ok {
		return Candidate{}, false
	}
	if name == "" {
		name = fmt.Sprintf("agent-%d", port)
	}
	return Candidate{
		Name:        name,
		Description: desc,
		Transport:   "http",
		Source:      "a2a",
		Config: map[string]string{
			"url":  fmt.Sprintf("http://localhost:%d", port),
			"port": fmt.Sprintf("%d", port),
		},
	}, true
}

// probeA2AEndpoint probes a host:port for an A2A agent card.
// Tries both the spec path and legacy path.
func probeA2AEndpoint(client *http.Client, host string, port int) (name, description string, ok bool) {
	paths := []string{"/.well-known/agent-card.json", "/.well-known/agent.json"}
	var resp *http.Response
	for _, path := range paths {
		u := fmt.Sprintf("http://%s:%d%s", host, port, path)
		r, err := client.Get(u)
		if err != nil {
			continue
		}
		if r.StatusCode != 200 {
			r.Body.Close()
			continue
		}
		resp = r
		break
	}
	if resp == nil {
		return "", "", false
	}
	defer resp.Body.Close()

	var card struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return "", "", false
	}
	return card.Name, card.Description, true
}

// deepProbePorts is the set of ports probed during deep scanning.
var deepProbePorts = []int{8001, 8002, 8003, 8004, 8005, 8006, 8007, 8008, 8009, 8010, 8080}

// scanDockerDeep probes running Docker containers for live A2A agents.
func scanDockerDeep() []Candidate {
	socketPath := "/var/run/docker.sock"
	if _, err := os.Stat(socketPath); err != nil {
		return nil
	}

	socketClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
		Timeout: 3 * time.Second,
	}

	// List running containers
	resp, err := socketClient.Get("http://localhost/containers/json")
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
		Image string
	}
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		return nil
	}

	probeClient := &http.Client{Timeout: 500 * time.Millisecond}
	var candidates []Candidate

	for _, c := range containers {
		if c.State != "running" {
			continue
		}

		containerName := c.ID[:12]
		if len(c.Names) > 0 {
			containerName = strings.TrimPrefix(c.Names[0], "/")
		}

		// Inspect container to get IP
		ip := inspectContainerIP(socketClient, containerName)
		if ip == "" {
			continue
		}

		// Probe ports for agent card
		for _, port := range deepProbePorts {
			name, desc, ok := probeA2AEndpoint(probeClient, ip, port)
			if !ok {
				continue
			}
			if name == "" {
				name = containerName
			}
			if desc == "" {
				desc = fmt.Sprintf("Docker container (%s)", c.Image)
			}
			candidates = append(candidates, Candidate{
				Name:        name,
				Description: desc,
				Transport:   "docker",
				Source:      "docker",
				Verified:    true,
				Config: map[string]string{
					"container":   containerName,
					"agent_port":  fmt.Sprintf("%d", port),
					"socket_path": socketPath,
					"network":     "bridge",
				},
			})
			break // one agent per container
		}
	}
	return candidates
}

// inspectContainerIP gets the first network IP of a Docker container.
func inspectContainerIP(socketClient *http.Client, name string) string {
	resp, err := socketClient.Get(fmt.Sprintf("http://localhost/containers/%s/json", url.PathEscape(name)))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}

	var raw struct {
		NetworkSettings struct {
			Networks map[string]struct {
				IPAddress string `json:"IPAddress"`
			} `json:"Networks"`
		} `json:"NetworkSettings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return ""
	}

	for _, net := range raw.NetworkSettings.Networks {
		if net.IPAddress != "" {
			return net.IPAddress
		}
	}
	return ""
}

// scanSSHDeep probes SSH hosts for live A2A agents through SSH tunnels.
func scanSSHDeep() []Candidate {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	sshDir := filepath.Join(home, ".ssh")
	keys := findSSHKeys(sshDir)
	if len(keys) == 0 {
		return nil
	}

	hosts := parseSSHConfig(filepath.Join(sshDir, "config"))
	var candidates []Candidate

	for _, h := range hosts {
		keyPath := h.keyPath
		if keyPath == "" && len(keys) > 0 {
			keyPath = keys[0]
		}

		sshUser := h.user
		if sshUser == "" {
			if u, err := user.Current(); err == nil {
				sshUser = u.Username
			}
		}

		port := 22
		if h.port != "" {
			if p, err := strconv.Atoi(h.port); err == nil {
				port = p
			}
		}

		found := probeViaSSH(h.name, h.hostname, sshUser, keyPath, port)
		candidates = append(candidates, found...)
	}
	return candidates
}

// probeViaSSH connects to an SSH host and probes for A2A agents on common ports.
func probeViaSSH(hostAlias, hostname, sshUser, keyPath string, sshPort int) []Candidate {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		slog.Debug("ssh deep scan: reading key", "host", hostAlias, "err", err)
		return nil
	}
	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		slog.Debug("ssh deep scan: parsing key", "host", hostAlias, "err", err)
		return nil
	}

	sshCfg := &ssh.ClientConfig{
		User:            sshUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: sshHostKeyCallback(),
		Timeout:         3 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", hostname, sshPort)
	slog.Info("ssh deep scan: probing host for A2A agents", "host", hostAlias, "addr", addr)
	client, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		slog.Debug("ssh deep scan: dial failed", "host", hostAlias, "addr", addr, "err", err)
		return nil
	}
	defer client.Close()

	// Create HTTP client that dials through the SSH tunnel
	probeClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, dialAddr string) (net.Conn, error) {
				return client.Dial("tcp", dialAddr)
			},
		},
		Timeout: 2 * time.Second,
	}

	var candidates []Candidate
	for _, port := range deepProbePorts {
		name, desc, ok := probeA2AEndpoint(probeClient, "127.0.0.1", port)
		if !ok {
			continue
		}
		if name == "" {
			name = fmt.Sprintf("%s-agent-%d", hostAlias, port)
		}
		if desc == "" {
			desc = fmt.Sprintf("A2A agent on %s", hostAlias)
		}

		candidates = append(candidates, Candidate{
			Name:        name,
			Description: desc,
			Transport:   "ssh",
			Source:      "ssh",
			Verified:    true,
			Config: map[string]string{
				"host":         hostname,
				"user":         sshUser,
				"port":         fmt.Sprintf("%d", sshPort),
				"key_path":     keyPath,
				"forward_port": fmt.Sprintf("%d", port),
			},
		})
	}
	return candidates
}

// scanSSHDeepCLIs probes SSH hosts for known CLI tools (claude, codex, etc.).
func scanSSHDeepCLIs() []Candidate {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	sshDir := filepath.Join(home, ".ssh")
	keys := findSSHKeys(sshDir)
	if len(keys) == 0 {
		return nil
	}

	hosts := parseSSHConfig(filepath.Join(sshDir, "config"))
	var candidates []Candidate

	for _, h := range hosts {
		keyPath := h.keyPath
		if keyPath == "" && len(keys) > 0 {
			keyPath = keys[0]
		}

		sshUser := h.user
		if sshUser == "" {
			if u, err := user.Current(); err == nil {
				sshUser = u.Username
			}
		}

		port := 22
		if h.port != "" {
			if p, err := strconv.Atoi(h.port); err == nil {
				port = p
			}
		}

		found := probeSSHCLIs(h.name, h.hostname, sshUser, keyPath, port)
		candidates = append(candidates, found...)
	}
	return candidates
}

// probeSSHCLIs connects to an SSH host and checks for known CLI tools.
func probeSSHCLIs(hostAlias, hostname, sshUser, keyPath string, sshPort int) []Candidate {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		slog.Debug("ssh cli scan: reading key", "host", hostAlias, "err", err)
		return nil
	}
	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		slog.Debug("ssh cli scan: parsing key", "host", hostAlias, "err", err)
		return nil
	}

	sshCfg := &ssh.ClientConfig{
		User:            sshUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: sshHostKeyCallback(),
		Timeout:         3 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", hostname, sshPort)
	slog.Info("ssh cli scan: probing host for CLI tools", "host", hostAlias, "addr", addr)
	client, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		slog.Debug("ssh cli scan: dial failed", "host", hostAlias, "addr", addr, "err", err)
		return nil
	}
	defer client.Close()

	var candidates []Candidate
	for name, desc := range knownCLIs {
		sess, err := client.NewSession()
		if err != nil {
			continue
		}
		// name comes from the hardcoded knownCLIs map — safe for shell use.
		output, err := sess.CombinedOutput("command -v " + name)
		sess.Close()
		if err != nil {
			continue
		}

		remotePath := strings.TrimSpace(string(output))
		if remotePath == "" {
			continue
		}

		candidateName := hostAlias + "-" + name
		cfg := map[string]string{
			"host":     hostname,
			"user":     sshUser,
			"port":     fmt.Sprintf("%d", sshPort),
			"key_path": keyPath,
			"command":  remotePath,
		}
		if defaultArgs, ok := knownCLIArgs[name]; ok {
			cfg["args"] = defaultArgs
		}

		candidates = append(candidates, Candidate{
			Name:        candidateName,
			Description: desc + " on " + hostAlias,
			Transport:   "ssh-exec",
			Source:   "ssh-cli",
			Verified: false, // binary exists but not confirmed as live agent
			Config:      cfg,
		})
	}
	return candidates
}

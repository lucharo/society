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

	"github.com/luischavesdev/society/internal/transport"
	"golang.org/x/crypto/ssh"
)

// ScanOptions configures how agent scanning behaves.
type ScanOptions struct {
	Deep     bool             // Probe SSH/Docker hosts for live A2A agents
	Progress func(msg string) // Optional callback for progress messages
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

// commonCLIPaths lists directories where CLI tools are commonly installed
// but may not be in the SSH session's PATH (non-login shells often miss these).
var commonCLIPaths = []string{
	"~/.local/bin",
	"/usr/local/bin",
	"/opt/homebrew/bin",
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
	progress := opts.Progress
	if progress == nil {
		progress = func(string) {}
	}

	progress("Scanning local CLI tools...")
	var all []Candidate
	all = append(all, scanCLIs()...)

	progress("Scanning Docker containers...")
	if _, err := os.Stat("/var/run/docker.sock"); err != nil {
		progress("  (skipped: Docker socket not found)")
	} else {
		all = append(all, scanDocker()...)
	}

	progress("Scanning SSH config...")
	home, homeErr := os.UserHomeDir()
	sshDir := filepath.Join(home, ".ssh")
	if homeErr != nil {
		progress("  (skipped: cannot determine home directory)")
	} else if _, err := os.Stat(sshDir); err != nil {
		progress("  (skipped: no ~/.ssh directory)")
	} else if keys := findSSHKeys(sshDir); len(keys) == 0 {
		progress("  (skipped: no SSH keys found in ~/.ssh)")
	} else {
		all = append(all, scanSSH()...)
	}

	progress("Scanning Tailscale peers...")
	tsPeers := parseTailscaleStatus()
	all = append(all, scanTailscale(tsPeers)...)

	progress("Scanning local A2A ports...")
	all = append(all, scanA2A()...)

	if opts.Deep {
		progress("Deep scan: probing Docker containers for A2A agents...")
		deep := scanDockerDeep()
		progress("Deep scan: probing SSH hosts for A2A agents...")
		deep = append(deep, scanSSHDeep()...)
		progress("Deep scan: probing SSH hosts for CLI tools...")
		deep = append(deep, scanSSHDeepCLIs()...)
		progress("Deep scan: probing Tailscale peers for A2A agents (HTTP)...")
		deep = append(deep, scanTailscaleDeepHTTP(tsPeers)...)
		progress("Deep scan: probing Tailscale peers for CLI tools (SSH)...")
		deep = append(deep, scanTailscaleDeepCLIs(tsPeers)...)
		all = dedup(all, deep)
	}
	return all
}

// dedup merges deep (verified) candidates into the shallow list.
// Deep candidates replace shallow ones with matching transport and host/container.
// Verified HTTP candidates also replace unverified SSH candidates for the same host
// (e.g., when a Tailscale peer has a live A2A agent reachable directly over HTTP,
// the SSH tunnel candidate becomes redundant).
func dedup(shallow, deep []Candidate) []Candidate {
	replaced := make(map[int]bool)
	for _, d := range deep {
		for i, s := range shallow {
			match := false
			if s.Transport == d.Transport {
				switch s.Transport {
				case "docker":
					match = s.Config["container"] == d.Config["container"]
				case "ssh":
					match = s.Config["host"] == d.Config["host"]
				case "ssh-exec":
					match = s.Config["host"] == d.Config["host"] && s.Config["command"] == d.Config["command"]
				}
			} else if d.Verified && d.Transport == "http" && s.Transport == "ssh" && !s.Verified {
				// A verified HTTP agent replaces an unverified SSH candidate for the same host
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
	sshCfg, err := transport.BuildSSHClientConfig(sshUser, keyPath)
	if err != nil {
		slog.Debug("ssh deep scan: config", "host", hostAlias, "err", err)
		return nil
	}
	sshCfg.Timeout = 3 * time.Second // shorter timeout for scanning

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
// It first tries `command -v` (which only finds CLIs in PATH), then falls
// back to checking common install directories for any CLIs not found.
func probeSSHCLIs(hostAlias, hostname, sshUser, keyPath string, sshPort int) []Candidate {
	sshCfg, err := transport.BuildSSHClientConfig(sshUser, keyPath)
	if err != nil {
		slog.Debug("ssh cli scan: config", "host", hostAlias, "err", err)
		return nil
	}
	sshCfg.Timeout = 3 * time.Second // shorter timeout for scanning

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
		remotePath := ""

		// First try command -v (finds CLIs in PATH).
		// name comes from the hardcoded knownCLIs map — safe for shell use.
		sess, err := client.NewSession()
		if err != nil {
			continue
		}
		output, err := sess.CombinedOutput("command -v " + name)
		sess.Close()
		if err == nil {
			remotePath = strings.TrimSpace(string(output))
		}

		// If command -v missed it, check common install directories.
		if remotePath == "" {
			for _, dir := range commonCLIPaths {
				// Expand ~ to $HOME (avoids eval for safety).
				expanded := strings.Replace(dir, "~", "$HOME", 1)
				candidate := expanded + "/" + name
				sess2, err := client.NewSession()
				if err != nil {
					continue
				}
				// Check executable and print the resolved path.
				cmd := fmt.Sprintf("p=\"%s\" && test -x \"$p\" && echo \"$p\"", candidate)
				out, err := sess2.CombinedOutput(cmd)
				sess2.Close()
				if err == nil {
					if resolved := strings.TrimSpace(string(out)); resolved != "" {
						remotePath = resolved
						break
					}
				}
			}
		}

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
			Source:      "ssh-cli",
			Verified:    false, // binary exists but not confirmed as live agent
			Config:      cfg,
		})
	}
	return candidates
}

// tailscalePeer holds information about a Tailscale peer parsed from `tailscale status --json`.
type tailscalePeer struct {
	hostName     string   // e.g. "MacBookPro"
	dnsName      string   // e.g. "macbookpro.tailacf9ef.ts.net."
	os           string   // e.g. "macOS", "linux", "iOS"
	tailscaleIPs []string // e.g. ["100.65.152.84", "fd7a:..."]
}

// sshCapableOS returns true if the OS is likely to support SSH connections.
func sshCapableOS(os string) bool {
	switch strings.ToLower(os) {
	case "linux", "macos", "windows", "freebsd", "openbsd":
		return true
	default:
		return false
	}
}

// parseTailscaleStatus runs `tailscale status --json` and returns online,
// SSH-capable peers (excluding self and mobile devices).
func parseTailscaleStatus() []tailscalePeer {
	out, err := exec.Command("tailscale", "status", "--json").Output()
	if err != nil {
		slog.Debug("tailscale: status failed", "err", err)
		return nil
	}

	var status struct {
		Self struct {
			HostName string `json:"HostName"`
		} `json:"Self"`
		Peer map[string]struct {
			HostName     string   `json:"HostName"`
			DNSName      string   `json:"DNSName"`
			OS           string   `json:"OS"`
			TailscaleIPs []string `json:"TailscaleIPs"`
			Online       bool     `json:"Online"`
		} `json:"Peer"`
	}
	if err := json.Unmarshal(out, &status); err != nil {
		slog.Debug("tailscale: parse failed", "err", err)
		return nil
	}

	var peers []tailscalePeer
	for _, p := range status.Peer {
		if !p.Online || !sshCapableOS(p.OS) {
			continue
		}
		peers = append(peers, tailscalePeer{
			hostName:     p.HostName,
			dnsName:      p.DNSName,
			os:           p.OS,
			tailscaleIPs: p.TailscaleIPs,
		})
	}
	return peers
}

// tailscaleHostname returns the lowercase hostname for use as an SSH target.
// Tailscale DNS names work as hostnames when Tailscale is running.
func tailscaleHostname(p tailscalePeer) string {
	return strings.ToLower(p.hostName)
}

// scanTailscale discovers Tailscale peers and returns them as candidates.
// In the regular (non-deep) scan these are SSH tunnel candidates.
// Peers that already appear in ~/.ssh/config are skipped (scanSSH handles those).
func scanTailscale(peers []tailscalePeer) []Candidate {
	if len(peers) == 0 {
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		slog.Debug("tailscale scan: cannot determine home directory", "err", err)
	}

	// Load SSH config hosts to skip duplicates
	var sshHostnames map[string]bool
	if home != "" {
		sshConfigHosts := parseSSHConfig(filepath.Join(home, ".ssh", "config"))
		sshHostnames = make(map[string]bool)
		for _, h := range sshConfigHosts {
			sshHostnames[strings.ToLower(h.name)] = true
			sshHostnames[strings.ToLower(h.hostname)] = true
		}
	}

	var keys []string
	if home != "" {
		keys = findSSHKeys(filepath.Join(home, ".ssh"))
	}

	currentUser := ""
	if u, err := user.Current(); err == nil {
		currentUser = u.Username
	}

	var candidates []Candidate
	for _, p := range peers {
		hostname := tailscaleHostname(p)
		if sshHostnames[hostname] {
			continue // already covered by SSH config
		}

		// With SSH keys available, offer as SSH tunnel candidate
		if len(keys) > 0 {
			candidates = append(candidates, Candidate{
				Name:        hostname,
				Description: fmt.Sprintf("Tailscale peer (%s)", p.os),
				Transport:   "ssh",
				Source:      "tailscale",
				Config: map[string]string{
					"host":         hostname,
					"user":         currentUser,
					"port":         "22",
					"key_path":     keys[0],
					"forward_port": "8080",
				},
			})
		}
	}
	return candidates
}

// tailscaleSSHHosts returns sshHost entries for Tailscale-only peers (not in SSH
// config). These are used for deep probing via SSH.
func tailscaleSSHHosts(peers []tailscalePeer) []sshHost {
	if len(peers) == 0 {
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	keys := findSSHKeys(filepath.Join(home, ".ssh"))
	if len(keys) == 0 {
		return nil
	}

	// Load SSH config to use its settings (user, key, port) when available
	sshConfigHosts := parseSSHConfig(filepath.Join(home, ".ssh", "config"))
	sshConfigByName := make(map[string]sshHost)
	for _, h := range sshConfigHosts {
		sshConfigByName[strings.ToLower(h.name)] = h
		sshConfigByName[strings.ToLower(h.hostname)] = h
	}

	currentUser := ""
	if u, err := user.Current(); err == nil {
		currentUser = u.Username
	}

	var hosts []sshHost
	for _, p := range peers {
		hostname := tailscaleHostname(p)

		// If this peer is already in SSH config, skip — scanSSHDeep/scanSSHDeepCLIs
		// already covers it. We only want Tailscale-only peers here.
		if _, ok := sshConfigByName[hostname]; ok {
			continue
		}

		hosts = append(hosts, sshHost{
			name:     hostname,
			hostname: hostname,
			user:     currentUser,
			port:     "22",
			keyPath:  keys[0],
		})
	}
	return hosts
}

// scanTailscaleDeepHTTP probes all Tailscale peers for live A2A agents via direct
// HTTP — no SSH tunnel needed since peers are directly reachable over Tailscale.
func scanTailscaleDeepHTTP(peers []tailscalePeer) []Candidate {
	if len(peers) == 0 {
		return nil
	}

	probeClient := &http.Client{Timeout: 2 * time.Second}
	var candidates []Candidate

	for _, p := range peers {
		hostname := tailscaleHostname(p)
		for _, port := range deepProbePorts {
			name, desc, ok := probeA2AEndpoint(probeClient, hostname, port)
			if !ok {
				continue
			}
			if name == "" {
				name = fmt.Sprintf("%s-agent-%d", hostname, port)
			}
			if desc == "" {
				desc = fmt.Sprintf("A2A agent on %s (Tailscale)", hostname)
			}
			candidates = append(candidates, Candidate{
				Name:        name,
				Description: desc,
				Transport:   "http",
				Source:      "tailscale",
				Verified:    true,
				Config: map[string]string{
					"url":  fmt.Sprintf("http://%s:%d", hostname, port),
					"host": hostname,
					"port": fmt.Sprintf("%d", port),
				},
			})
		}
	}
	return candidates
}

// scanTailscaleDeepCLIs probes Tailscale peers (not in SSH config) for CLI tools.
func scanTailscaleDeepCLIs(peers []tailscalePeer) []Candidate {
	var candidates []Candidate
	for _, h := range tailscaleSSHHosts(peers) {
		port := 22
		if h.port != "" {
			if p, err := strconv.Atoi(h.port); err == nil {
				port = p
			}
		}
		found := probeSSHCLIs(h.name, h.hostname, h.user, h.keyPath, port)
		candidates = append(candidates, found...)
	}
	return candidates
}

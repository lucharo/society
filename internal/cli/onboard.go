package cli

import (
	"bufio"
	"fmt"
	"io"
	"os/user"
	"strings"

	"github.com/luischavesdev/society/internal/models"
	"github.com/luischavesdev/society/internal/registry"
)

func Onboard(registryPath string, in io.Reader, out io.Writer) error {
	r := bufio.NewReader(in)

	fmt.Fprintln(out)
	name := prompt(r, out, "Agent name", "")
	if name == "" {
		return fmt.Errorf("agent name is required")
	}

	description := prompt(r, out, "Description (optional)", "")

	transportType := promptChoice(r, out, "Transport", []string{"http", "ssh", "docker", "stdio"}, "http")

	var url string
	var tc *models.TransportConfig

	switch transportType {
	case "http":
		url = prompt(r, out, "URL", "")
		if url == "" {
			return fmt.Errorf("URL is required for HTTP transport")
		}

	case "ssh":
		fmt.Fprintln(out, "\n  SSH Configuration:")
		host := prompt(r, out, "  Host", "")
		if host == "" {
			return fmt.Errorf("host is required for SSH transport")
		}

		defaultUser := "claude"
		if u, err := user.Current(); err == nil {
			defaultUser = u.Username
		}
		sshUser := prompt(r, out, "  User", defaultUser)
		port := prompt(r, out, "  Port", "22")
		keyPath := prompt(r, out, "  SSH key path", "~/.ssh/id_rsa")
		agentPort := prompt(r, out, "  Agent port on remote host", "8080")

		url = fmt.Sprintf("http://localhost:%s", agentPort)
		tc = &models.TransportConfig{
			Type: "ssh",
			Config: map[string]string{
				"host":         host,
				"user":         sshUser,
				"port":         port,
				"key_path":     keyPath,
				"forward_port": agentPort,
			},
		}

	case "docker":
		fmt.Fprintln(out, "\n  Docker Configuration:")
		container := prompt(r, out, "  Container name", "")
		if container == "" {
			return fmt.Errorf("container name is required for Docker transport")
		}
		network := prompt(r, out, "  Network", "bridge")
		agentPort := prompt(r, out, "  Agent port", "8080")
		socketPath := prompt(r, out, "  Socket path", "/var/run/docker.sock")

		url = fmt.Sprintf("http://%s:%s", container, agentPort)
		tc = &models.TransportConfig{
			Type: "docker",
			Config: map[string]string{
				"container":   container,
				"network":     network,
				"agent_port":  agentPort,
				"socket_path": socketPath,
			},
		}

	case "stdio":
		fmt.Fprintln(out, "\n  STDIO Configuration:")
		command := prompt(r, out, "  Command", "")
		if command == "" {
			return fmt.Errorf("command is required for STDIO transport")
		}
		args := prompt(r, out, "  Args (space-separated)", "")

		url = fmt.Sprintf("stdio://%s", command)
		tc = &models.TransportConfig{
			Type: "stdio",
			Config: map[string]string{
				"command": command,
				"args":    args,
			},
		}
	}

	skillsStr := prompt(r, out, "Skills (comma-separated IDs, or empty to skip)", "")
	var skills []models.Skill
	if skillsStr != "" {
		for _, s := range strings.Split(skillsStr, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				skills = append(skills, models.Skill{ID: s, Name: s})
			}
		}
	}

	card := models.AgentCard{
		Name:        name,
		Description: description,
		URL:         url,
		Skills:      skills,
		Transport:   tc,
	}

	if err := models.ValidateAgentCard(card); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	if tc != nil {
		if err := models.ValidateTransportConfig(tc); err != nil {
			return fmt.Errorf("transport validation failed: %w", err)
		}
	}

	reg, err := registry.Load(registryPath)
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}
	if reg.Has(name) {
		return fmt.Errorf("agent %q already exists in registry", name)
	}

	if err := reg.Add(card); err != nil {
		return err
	}
	if err := reg.Save(); err != nil {
		return err
	}

	fmt.Fprintf(out, "\n  ✓ Added %q to registry\n", name)
	return nil
}

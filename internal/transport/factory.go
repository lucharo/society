package transport

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/luischavesdev/society/internal/models"
)

func New(agentURL string, tc *models.TransportConfig) (Transport, error) {
	if tc == nil || tc.Type == "" || tc.Type == "http" {
		return NewHTTP(agentURL)
	}

	switch tc.Type {
	case "ssh":
		cfg := SSHConfig{
			Host:        configString(tc.Config, "host", ""),
			User:        configString(tc.Config, "user", ""),
			KeyPath:     configString(tc.Config, "key_path", ""),
			Port:        configInt(tc.Config, "port", 22),
			ForwardPort: configInt(tc.Config, "forward_port", 8080),
		}
		return NewSSH(cfg)

	case "docker":
		cfg := DockerConfig{
			Container:  configString(tc.Config, "container", ""),
			Network:    configString(tc.Config, "network", ""),
			SocketPath: configString(tc.Config, "socket_path", "/var/run/docker.sock"),
			AgentPort:  configInt(tc.Config, "agent_port", 8080),
		}
		return NewDocker(cfg)

	case "stdio":
		args := configString(tc.Config, "args", "")
		var argList []string
		if args != "" {
			argList = strings.Fields(args)
		}
		cfg := STDIOConfig{
			Command: configString(tc.Config, "command", ""),
			Args:    argList,
		}
		return NewSTDIO(cfg)

	default:
		return nil, fmt.Errorf("unknown transport type: %q", tc.Type)
	}
}

func configString(m map[string]string, key, defaultVal string) string {
	if m == nil {
		return defaultVal
	}
	if v, ok := m[key]; ok && v != "" {
		return v
	}
	return defaultVal
}

func configInt(m map[string]string, key string, defaultVal int) int {
	if m == nil {
		return defaultVal
	}
	v, ok := m[key]
	if !ok || v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}

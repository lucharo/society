package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// DockerAPI abstracts Docker Engine API calls for testability.
type DockerAPI interface {
	InspectContainer(ctx context.Context, name string) (*ContainerInfo, error)
}

type ContainerInfo struct {
	ID       string
	State    string
	Networks map[string]NetworkInfo
}

type NetworkInfo struct {
	IPAddress string
}

type DockerConfig struct {
	Container  string
	Network    string
	SocketPath string
	AgentPort  int
}

type DockerTransport struct {
	config      DockerConfig
	api         DockerAPI
	containerIP string
	httpClient  *http.Client
}

type defaultDockerAPI struct {
	client     *http.Client
	socketPath string
}

func (d *defaultDockerAPI) InspectContainer(ctx context.Context, name string) (*ContainerInfo, error) {
	url := fmt.Sprintf("http://localhost/containers/%s/json", name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("docker: creating request: %w", err)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("docker: inspecting container: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("docker: container %q not found", name)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("docker: inspect returned %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		ID    string `json:"Id"`
		State struct {
			Status string `json:"Status"`
		} `json:"State"`
		NetworkSettings struct {
			Networks map[string]struct {
				IPAddress string `json:"IPAddress"`
			} `json:"Networks"`
		} `json:"NetworkSettings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("docker: parsing response: %w", err)
	}

	info := &ContainerInfo{
		ID:       raw.ID,
		State:    raw.State.Status,
		Networks: make(map[string]NetworkInfo),
	}
	for name, net := range raw.NetworkSettings.Networks {
		info.Networks[name] = NetworkInfo{IPAddress: net.IPAddress}
	}
	return info, nil
}

func NewDocker(cfg DockerConfig, opts ...func(*DockerTransport)) (*DockerTransport, error) {
	if cfg.Container == "" {
		return nil, fmt.Errorf("docker: container is required")
	}
	if cfg.SocketPath == "" {
		cfg.SocketPath = "/var/run/docker.sock"
	}
	if cfg.AgentPort == 0 {
		cfg.AgentPort = 8080
	}

	socketClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", cfg.SocketPath)
			},
		},
		Timeout: 10 * time.Second,
	}

	t := &DockerTransport{
		config:     cfg,
		api:        &defaultDockerAPI{client: socketClient, socketPath: cfg.SocketPath},
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, o := range opts {
		o(t)
	}
	return t, nil
}

func WithDockerAPI(api DockerAPI) func(*DockerTransport) {
	return func(t *DockerTransport) { t.api = api }
}

func (t *DockerTransport) Open(ctx context.Context) error {
	info, err := t.api.InspectContainer(ctx, t.config.Container)
	if err != nil {
		return err
	}

	if info.State != "running" {
		return fmt.Errorf("docker: container %q is %s, not running", t.config.Container, info.State)
	}

	if t.config.Network != "" {
		net, ok := info.Networks[t.config.Network]
		if !ok {
			return fmt.Errorf("docker: container not on network %q", t.config.Network)
		}
		if net.IPAddress == "" {
			return fmt.Errorf("docker: no IP on network %q", t.config.Network)
		}
		t.containerIP = net.IPAddress
	} else {
		// Use first available network
		for _, net := range info.Networks {
			if net.IPAddress != "" {
				t.containerIP = net.IPAddress
				break
			}
		}
		if t.containerIP == "" {
			return fmt.Errorf("docker: container has no network IP")
		}
	}

	return nil
}

func (t *DockerTransport) Send(ctx context.Context, payload []byte) ([]byte, error) {
	url := fmt.Sprintf("http://%s:%d", t.containerIP, t.config.AgentPort)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("docker: creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("docker: sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("docker: reading response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("docker: server returned %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func (t *DockerTransport) Close() error { return nil }

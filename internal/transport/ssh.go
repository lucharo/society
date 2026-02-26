package transport

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHDialer abstracts SSH connection creation for testability.
type SSHDialer interface {
	Dial(network, addr string, config *ssh.ClientConfig) (SSHClient, error)
}

// SSHClient abstracts an SSH client connection for testability.
type SSHClient interface {
	Dial(network, addr string) (net.Conn, error)
	Close() error
}

type SSHConfig struct {
	Host        string
	User        string
	Port        int
	KeyPath     string
	ForwardPort int
}

type SSHTransport struct {
	config     SSHConfig
	dialer     SSHDialer
	client     SSHClient
	listener   net.Listener
	localPort  int
	httpClient *http.Client
}

type defaultSSHDialer struct{}

func (d *defaultSSHDialer) Dial(network, addr string, config *ssh.ClientConfig) (SSHClient, error) {
	c, err := ssh.Dial(network, addr, config)
	if err != nil {
		return nil, err
	}
	return &realSSHClient{c}, nil
}

type realSSHClient struct{ *ssh.Client }

func (c *realSSHClient) Dial(network, addr string) (net.Conn, error) {
	return c.Client.Dial(network, addr)
}

func (c *realSSHClient) Close() error {
	return c.Client.Close()
}

func NewSSH(cfg SSHConfig, opts ...func(*SSHTransport)) (*SSHTransport, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("ssh: host is required")
	}
	if cfg.User == "" {
		return nil, fmt.Errorf("ssh: user is required")
	}
	if cfg.KeyPath == "" {
		return nil, fmt.Errorf("ssh: key_path is required")
	}
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	t := &SSHTransport{
		config:     cfg,
		dialer:     &defaultSSHDialer{},
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, o := range opts {
		o(t)
	}
	return t, nil
}

func WithSSHDialer(d SSHDialer) func(*SSHTransport) {
	return func(t *SSHTransport) { t.dialer = d }
}

func (t *SSHTransport) Open(ctx context.Context) error {
	keyData, err := os.ReadFile(t.config.KeyPath)
	if err != nil {
		return fmt.Errorf("ssh: reading key: %w", err)
	}
	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return fmt.Errorf("ssh: parsing key: %w", err)
	}

	// TODO: support known_hosts verification via knownhosts.New()
	slog.Warn("ssh: host key verification disabled — vulnerable to MITM attacks",
		"host", t.config.Host)
	sshCfg := &ssh.ClientConfig{
		User:            t.config.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", t.config.Host, t.config.Port)
	client, err := t.dialer.Dial("tcp", addr, sshCfg)
	if err != nil {
		return fmt.Errorf("ssh: connecting to %s: %w", addr, err)
	}
	t.client = client

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		client.Close()
		return fmt.Errorf("ssh: local listener: %w", err)
	}
	t.listener = ln
	t.localPort = ln.Addr().(*net.TCPAddr).Port

	// Forward connections through SSH tunnel
	go func() {
		for {
			local, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				remote, err := client.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", t.config.ForwardPort))
				if err != nil {
					local.Close()
					return
				}
				go func() { io.Copy(remote, local); remote.Close() }()
				io.Copy(local, remote)
				local.Close()
			}()
		}
	}()

	return nil
}

func (t *SSHTransport) Send(ctx context.Context, payload []byte) ([]byte, error) {
	url := fmt.Sprintf("http://127.0.0.1:%d", t.localPort)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("ssh: creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ssh: sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ssh: reading response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ssh: server returned %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func (t *SSHTransport) Close() error {
	if t.listener != nil {
		t.listener.Close()
	}
	if t.client != nil {
		t.client.Close()
	}
	return nil
}

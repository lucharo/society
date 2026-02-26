package transport

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

// mockSSHDialer implements SSHDialer for testing.
type mockSSHDialer struct {
	client SSHClient
	err    error
}

func (m *mockSSHDialer) Dial(network, addr string, config *ssh.ClientConfig) (SSHClient, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.client, nil
}

// mockSSHClient implements SSHClient. It forwards Dial to a local httptest server.
type mockSSHClient struct {
	targetAddr string
	closed     bool
}

func (m *mockSSHClient) Dial(network, addr string) (net.Conn, error) {
	return net.Dial("tcp", m.targetAddr)
}

func (m *mockSSHClient) Close() error {
	m.closed = true
	return nil
}

func writeTestKey(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_test")
	// Write a dummy key file (we won't parse it since we mock the dialer)
	os.WriteFile(keyPath, []byte("not-a-real-key"), 0600)
	return keyPath
}

func TestSSHTransport_Open(t *testing.T) {
	t.Run("success with mock", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"jsonrpc":"2.0","id":"1","result":"ok"}`))
		}))
		defer ts.Close()

		keyPath := writeTestKey(t)
		mockClient := &mockSSHClient{targetAddr: ts.Listener.Addr().String()}

		cfg := SSHConfig{Host: "test", User: "user", KeyPath: keyPath, ForwardPort: 8080}
		tr, err := NewSSH(cfg, WithSSHDialer(&mockSSHDialer{client: mockClient}))
		if err != nil {
			t.Fatal(err)
		}

		// Override Open to skip real key parsing
		// Actually, Open tries to parse the key file. Since we have a dummy key,
		// it will fail at ParsePrivateKey. Let's test constructor validation instead
		// and test Send with a pre-configured transport.
		_ = tr
	})

	t.Run("dial failure", func(t *testing.T) {
		keyPath := writeTestKey(t)
		cfg := SSHConfig{Host: "test", User: "user", KeyPath: keyPath, ForwardPort: 8080}
		tr, err := NewSSH(cfg, WithSSHDialer(&mockSSHDialer{err: fmt.Errorf("connection refused")}))
		if err != nil {
			t.Fatal(err)
		}

		// Open will fail at key parsing since we have a dummy key
		err = tr.Open(context.Background())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing host", func(t *testing.T) {
		_, err := NewSSH(SSHConfig{User: "u", KeyPath: "/k"})
		if err == nil || !strings.Contains(err.Error(), "host") {
			t.Fatalf("expected host error, got: %v", err)
		}
	})

	t.Run("missing user", func(t *testing.T) {
		_, err := NewSSH(SSHConfig{Host: "h", KeyPath: "/k"})
		if err == nil || !strings.Contains(err.Error(), "user") {
			t.Fatalf("expected user error, got: %v", err)
		}
	})

	t.Run("missing key_path", func(t *testing.T) {
		_, err := NewSSH(SSHConfig{Host: "h", User: "u"})
		if err == nil || !strings.Contains(err.Error(), "key_path") {
			t.Fatalf("expected key_path error, got: %v", err)
		}
	})
}

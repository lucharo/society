package transport

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type mockDockerAPI struct {
	info *ContainerInfo
	err  error
}

func (m *mockDockerAPI) InspectContainer(ctx context.Context, name string) (*ContainerInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.info, nil
}

func TestDockerTransport_Open(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		api := &mockDockerAPI{info: &ContainerInfo{
			ID:    "abc123",
			State: "running",
			Networks: map[string]NetworkInfo{
				"bridge": {IPAddress: "172.17.0.2"},
			},
		}}

		tr, err := NewDocker(DockerConfig{Container: "test"}, WithDockerAPI(api))
		if err != nil {
			t.Fatal(err)
		}

		if err := tr.Open(context.Background()); err != nil {
			t.Fatal(err)
		}
		if tr.containerIP != "172.17.0.2" {
			t.Errorf("got IP %q, want 172.17.0.2", tr.containerIP)
		}
	})

	t.Run("container not found", func(t *testing.T) {
		api := &mockDockerAPI{err: fmt.Errorf("docker: container %q not found", "missing")}
		tr, _ := NewDocker(DockerConfig{Container: "missing"}, WithDockerAPI(api))

		err := tr.Open(context.Background())
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("expected not found error, got: %v", err)
		}
	})

	t.Run("container not running", func(t *testing.T) {
		api := &mockDockerAPI{info: &ContainerInfo{
			ID:       "abc",
			State:    "exited",
			Networks: map[string]NetworkInfo{"bridge": {IPAddress: "172.17.0.2"}},
		}}
		tr, _ := NewDocker(DockerConfig{Container: "test"}, WithDockerAPI(api))

		err := tr.Open(context.Background())
		if err == nil || !strings.Contains(err.Error(), "not running") {
			t.Fatalf("expected not running error, got: %v", err)
		}
	})

	t.Run("no matching network", func(t *testing.T) {
		api := &mockDockerAPI{info: &ContainerInfo{
			ID:       "abc",
			State:    "running",
			Networks: map[string]NetworkInfo{"bridge": {IPAddress: "172.17.0.2"}},
		}}
		tr, _ := NewDocker(DockerConfig{Container: "test", Network: "custom"}, WithDockerAPI(api))

		err := tr.Open(context.Background())
		if err == nil || !strings.Contains(err.Error(), "not on network") {
			t.Fatalf("expected network error, got: %v", err)
		}
	})

	t.Run("missing container config", func(t *testing.T) {
		_, err := NewDocker(DockerConfig{})
		if err == nil || !strings.Contains(err.Error(), "container is required") {
			t.Fatalf("expected container required error, got: %v", err)
		}
	})
}

func TestDockerTransport_Send(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"jsonrpc":"2.0","id":"1","result":"ok"}`))
	}))
	defer ts.Close()

	api := &mockDockerAPI{info: &ContainerInfo{
		ID:       "abc",
		State:    "running",
		Networks: map[string]NetworkInfo{"bridge": {IPAddress: "127.0.0.1"}},
	}}

	// Extract port from test server
	addr := ts.Listener.Addr().String()
	parts := strings.Split(addr, ":")
	port := 0
	fmt.Sscanf(parts[len(parts)-1], "%d", &port)

	tr, _ := NewDocker(DockerConfig{Container: "test", AgentPort: port}, WithDockerAPI(api))
	if err := tr.Open(context.Background()); err != nil {
		t.Fatal(err)
	}

	resp, err := tr.Send(context.Background(), []byte(`{"jsonrpc":"2.0","id":"1","method":"test"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(resp), `"result":"ok"`) {
		t.Errorf("unexpected response: %s", resp)
	}
}

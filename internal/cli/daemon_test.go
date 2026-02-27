package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/luischavesdev/society/internal/models"
)

func writeAgentYAML(t *testing.T, dir, name string, port int) string {
	t.Helper()
	content := fmt.Sprintf("name: %s\ndescription: test agent\nport: %d\nhandler: echo\n", name, port)
	path := filepath.Join(dir, name+".yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDiscoverConfigs(t *testing.T) {
	dir := t.TempDir()
	writeAgentYAML(t, dir, "alpha", 9001)
	writeAgentYAML(t, dir, "beta", 9002)

	configs, err := discoverConfigs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(configs))
	}

	names := map[string]bool{}
	for _, cfg := range configs {
		names[cfg.Name] = true
	}
	if !names["alpha"] || !names["beta"] {
		t.Errorf("expected alpha and beta, got %v", names)
	}
}

func TestDiscoverConfigs_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	_, err := discoverConfigs(dir)
	if err == nil {
		t.Fatal("expected error for empty dir")
	}
	if !strings.Contains(err.Error(), "no agent configs") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDiscoverConfigs_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	// Missing required name field.
	os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("handler: echo\nport: 9999\n"), 0644)
	_, err := discoverConfigs(dir)
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
}

func TestFilterConfigs(t *testing.T) {
	configs := []*models.AgentConfig{
		{Name: "alpha", Port: 9001, Handler: "echo"},
		{Name: "beta", Port: 9002, Handler: "echo"},
		{Name: "gamma", Port: 9003, Handler: "echo"},
	}

	tests := []struct {
		name    string
		filter  []string
		want    int
		wantErr bool
	}{
		{"single match", []string{"alpha"}, 1, false},
		{"multi match", []string{"alpha", "gamma"}, 2, false},
		{"all match", []string{"alpha", "beta", "gamma"}, 3, false},
		{"unknown name", []string{"missing"}, 0, true},
		{"partial unknown", []string{"alpha", "missing"}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := filterConfigs(configs, tt.filter)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tt.want {
				t.Errorf("got %d configs, want %d", len(got), tt.want)
			}
		})
	}
}

func TestCheckPortConflicts(t *testing.T) {
	tests := []struct {
		name    string
		configs []*models.AgentConfig
		wantErr bool
	}{
		{
			"no conflicts",
			[]*models.AgentConfig{
				{Name: "a", Port: 9001},
				{Name: "b", Port: 9002},
			},
			false,
		},
		{
			"conflict",
			[]*models.AgentConfig{
				{Name: "a", Port: 9001},
				{Name: "b", Port: 9001},
			},
			true,
		},
		{
			"single agent",
			[]*models.AgentConfig{
				{Name: "a", Port: 9001},
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkPortConflicts(tt.configs)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestDaemonState_RoundTrip(t *testing.T) {
	// Override societyDir for testing by writing to a temp dir.
	origState := &DaemonState{
		PID:       12345,
		Agents:    []string{"echo", "greeter"},
		Ports:     []int{8001, 8002},
		AgentsDir: "/tmp/agents",
		StartedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.json")
	data, err := json.MarshalIndent(origState, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	readData, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got DaemonState
	if err := json.Unmarshal(readData, &got); err != nil {
		t.Fatal(err)
	}

	if got.PID != origState.PID {
		t.Errorf("PID: got %d, want %d", got.PID, origState.PID)
	}
	if len(got.Agents) != len(origState.Agents) {
		t.Errorf("Agents: got %v, want %v", got.Agents, origState.Agents)
	}
	if len(got.Ports) != len(origState.Ports) {
		t.Errorf("Ports: got %v, want %v", got.Ports, origState.Ports)
	}
	if got.AgentsDir != origState.AgentsDir {
		t.Errorf("AgentsDir: got %s, want %s", got.AgentsDir, origState.AgentsDir)
	}
	if !got.StartedAt.Equal(origState.StartedAt) {
		t.Errorf("StartedAt: got %v, want %v", got.StartedAt, origState.StartedAt)
	}
}

func TestDaemonStatus_NotRunning(t *testing.T) {
	// Ensure no PID file exists by using a non-default home.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	out := &bytes.Buffer{}
	if err := DaemonStatus(out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "not running") {
		t.Errorf("expected 'not running', got: %s", out.String())
	}
}

func TestDaemonStatus_StalePID(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Write a PID file with a PID that definitely doesn't exist.
	dir := filepath.Join(tmpHome, ".society")
	os.MkdirAll(dir, 0755)
	state := DaemonState{PID: 999999999, Agents: []string{"test"}, Ports: []int{9999}, StartedAt: time.Now()}
	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(filepath.Join(dir, "daemon.json"), data, 0644)

	out := &bytes.Buffer{}
	if err := DaemonStatus(out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "stale PID file") {
		t.Errorf("expected stale PID message, got: %s", out.String())
	}
}

// freePort returns a port that is currently available.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

func TestDaemonRun_StartsAndStops(t *testing.T) {
	dir := t.TempDir()
	port1 := freePort(t)
	port2 := freePort(t)
	writeAgentYAML(t, dir, "echo1", port1)
	writeAgentYAML(t, dir, "echo2", port2)

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	ctx, cancel := context.WithCancel(context.Background())

	out := &bytes.Buffer{}
	errCh := make(chan error, 1)
	go func() {
		// Use runAgents directly since DaemonRun sets up its own signal context.
		configs, err := discoverConfigs(dir)
		if err != nil {
			errCh <- err
			return
		}
		errCh <- runAgents(ctx, configs, dir, out)
	}()

	// Wait for servers to start.
	time.Sleep(300 * time.Millisecond)

	// Verify PID file was written.
	pidPath := filepath.Join(tmpHome, ".society", "daemon.json")
	if _, err := os.Stat(pidPath); os.IsNotExist(err) {
		t.Fatal("PID file was not created")
	}

	// Read state.
	data, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatal(err)
	}
	var state DaemonState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	if len(state.Agents) != 2 {
		t.Errorf("expected 2 agents in state, got %d", len(state.Agents))
	}

	// Stop the daemon.
	cancel()
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}

	// Verify PID file was cleaned up.
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("PID file should have been cleaned up")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{5 * time.Second, "5s"},
		{90 * time.Second, "1m 30s"},
		{2*time.Hour + 15*time.Minute, "2h 15m"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

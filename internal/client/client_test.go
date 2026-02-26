package client

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luischavesdev/society/internal/agent"
	"github.com/luischavesdev/society/internal/models"
	"github.com/luischavesdev/society/internal/registry"
)

func setupTestAgent(t *testing.T) (*httptest.Server, *registry.Registry) {
	t.Helper()

	card := models.AgentCard{Name: "echo", URL: "http://placeholder", Description: "test echo"}
	srv := agent.NewServer(card, &agent.EchoHandler{})
	ts := httptest.NewServer(srv)

	// Write registry with test agent URL
	regPath := filepath.Join(t.TempDir(), "reg.json")
	agentCard := models.AgentCard{Name: "echo", URL: ts.URL}
	rf := models.RegistryFile{Agents: []models.AgentCard{agentCard}}
	data, _ := json.MarshalIndent(rf, "", "  ")
	os.WriteFile(regPath, data, 0644)

	reg, err := registry.Load(regPath)
	if err != nil {
		ts.Close()
		t.Fatal(err)
	}
	return ts, reg
}

func TestClient_Send_Success(t *testing.T) {
	ts, reg := setupTestAgent(t)
	defer ts.Close()

	c := New(reg)
	task, err := c.Send(context.Background(), "echo", "hello world")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status.State != models.TaskStateCompleted {
		t.Errorf("got state %s, want completed", task.Status.State)
	}
	if len(task.Artifacts) == 0 || len(task.Artifacts[0].Parts) == 0 {
		t.Fatal("expected artifacts with parts")
	}
	if task.Artifacts[0].Parts[0].Text != "hello world" {
		t.Errorf("got %q, want hello world", task.Artifacts[0].Parts[0].Text)
	}
}

func TestClient_Send_AgentNotFound(t *testing.T) {
	reg, _ := registry.Load(filepath.Join(t.TempDir(), "empty.json"))
	c := New(reg)

	_, err := c.Send(context.Background(), "nonexistent", "hi")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

func TestClient_Ping_Success(t *testing.T) {
	ts, reg := setupTestAgent(t)
	defer ts.Close()

	c := New(reg)
	card, err := c.Ping(context.Background(), "echo")
	if err != nil {
		t.Fatal(err)
	}
	if card.Name != "echo" {
		t.Errorf("got name %q, want echo", card.Name)
	}
}

func TestClient_Ping_AgentNotFound(t *testing.T) {
	reg, _ := registry.Load(filepath.Join(t.TempDir(), "empty.json"))
	c := New(reg)

	_, err := c.Ping(context.Background(), "nonexistent")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

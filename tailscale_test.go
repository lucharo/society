//go:build tailscale

package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/luischavesdev/society/internal/client"
	"github.com/luischavesdev/society/internal/models"
	"github.com/luischavesdev/society/internal/registry"
)

// These tests require:
// - Tailscale running with arch-lenovo reachable as "arch"
// - SSH access to arch with key at ~/.ssh/id_ed25519
// - A society agent running on arch (e.g., echo on port 8001 or greeter on port 8002)

func TestTailscale_SSH_Echo(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	regPath := filepath.Join(t.TempDir(), "reg.json")
	rf := models.RegistryFile{Agents: []models.AgentCard{{
		Name: "arch-echo",
		URL:  "http://localhost:8001",
		Transport: &models.TransportConfig{
			Type: "ssh",
			Config: map[string]string{
				"host":         "arch",
				"user":         "luis",
				"key_path":     filepath.Join(homeDir, ".ssh", "id_ed25519"),
				"port":         "22",
				"forward_port": "8001",
			},
		},
	}}}
	data, _ := json.MarshalIndent(rf, "", "  ")
	mustWriteFile(t, regPath, data)

	reg, _ := registry.Load(regPath)
	c := client.New(reg)

	task, err := c.Send(context.Background(), "arch-echo", "tailscale echo test")
	if err != nil {
		t.Fatal(err)
	}

	if task.Status.State != models.TaskStateCompleted {
		t.Errorf("got state %s", task.Status.State)
	}
	if task.Artifacts[0].Parts[0].Text != "tailscale echo test" {
		t.Errorf("got %q", task.Artifacts[0].Parts[0].Text)
	}
}

func TestTailscale_SSH_Greeter(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	regPath := filepath.Join(t.TempDir(), "reg.json")
	rf := models.RegistryFile{Agents: []models.AgentCard{{
		Name: "arch-greeter",
		URL:  "http://localhost:8002",
		Transport: &models.TransportConfig{
			Type: "ssh",
			Config: map[string]string{
				"host":         "arch",
				"user":         "luis",
				"key_path":     filepath.Join(homeDir, ".ssh", "id_ed25519"),
				"port":         "22",
				"forward_port": "8002",
			},
		},
	}}}
	data, _ := json.MarshalIndent(rf, "", "  ")
	mustWriteFile(t, regPath, data)

	reg, _ := registry.Load(regPath)
	c := client.New(reg)

	task, err := c.Send(context.Background(), "arch-greeter", "tailscale")
	if err != nil {
		t.Fatal(err)
	}

	if task.Status.State != models.TaskStateCompleted {
		t.Errorf("got state %s", task.Status.State)
	}
	want := "Hello! You said: tailscale"
	if task.Artifacts[0].Parts[0].Text != want {
		t.Errorf("got %q, want %q", task.Artifacts[0].Parts[0].Text, want)
	}
}

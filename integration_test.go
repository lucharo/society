//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luischavesdev/society/internal/agent"
	"github.com/luischavesdev/society/internal/cli"
	"github.com/luischavesdev/society/internal/client"
	"github.com/luischavesdev/society/internal/models"
	"github.com/luischavesdev/society/internal/registry"
)

func TestFullFlow_HTTP(t *testing.T) {
	// Start echo agent
	card := models.AgentCard{Name: "echo", URL: "http://placeholder"}
	srv := agent.NewServer(card, &agent.EchoHandler{})
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Build registry
	regPath := filepath.Join(t.TempDir(), "reg.json")
	agentCard := models.AgentCard{Name: "echo", URL: ts.URL}
	rf := models.RegistryFile{Agents: []models.AgentCard{agentCard}}
	data, _ := json.MarshalIndent(rf, "", "  ")
	os.WriteFile(regPath, data, 0644)

	// Send
	reg, _ := registry.Load(regPath)
	c := client.New(reg)
	task, err := c.Send(context.Background(), "echo", "hello integration")
	if err != nil {
		t.Fatal(err)
	}

	if task.Status.State != models.TaskStateCompleted {
		t.Errorf("got state %s", task.Status.State)
	}
	if task.Artifacts[0].Parts[0].Text != "hello integration" {
		t.Errorf("got %q", task.Artifacts[0].Parts[0].Text)
	}
}

func TestFullFlow_Onboard_And_Send(t *testing.T) {
	// Start echo agent
	card := models.AgentCard{Name: "test-echo", URL: "http://placeholder"}
	srv := agent.NewServer(card, &agent.EchoHandler{})
	ts := httptest.NewServer(srv)
	defer ts.Close()

	regPath := filepath.Join(t.TempDir(), "reg.json")

	// Onboard via simulated stdin
	input := "test-echo\nEcho test\nhttp\n" + ts.URL + "\n\n"
	out := &bytes.Buffer{}
	if err := cli.Onboard(regPath, strings.NewReader(input), out); err != nil {
		t.Fatal(err)
	}

	// Send
	sendOut := &bytes.Buffer{}
	if err := cli.Send(regPath, "test-echo", "integration test", sendOut); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(sendOut.String(), "integration test") {
		t.Errorf("output: %s", sendOut.String())
	}
}

func TestFullFlow_Export_Import(t *testing.T) {
	// Create registry with agents
	regPath := filepath.Join(t.TempDir(), "reg.json")
	rf := models.RegistryFile{Agents: []models.AgentCard{
		{Name: "a", URL: "http://a"},
		{Name: "b", URL: "http://b"},
	}}
	data, _ := json.MarshalIndent(rf, "", "  ")
	os.WriteFile(regPath, data, 0644)

	// Export
	exportPath := filepath.Join(t.TempDir(), "export.json")
	out := &bytes.Buffer{}
	if err := cli.Export(regPath, exportPath, out); err != nil {
		t.Fatal(err)
	}

	// Import to new registry
	newRegPath := filepath.Join(t.TempDir(), "new.json")
	importOut := &bytes.Buffer{}
	if err := cli.Import(newRegPath, exportPath, strings.NewReader(""), importOut); err != nil {
		t.Fatal(err)
	}

	// Verify
	reg, _ := registry.Load(newRegPath)
	if !reg.Has("a") || !reg.Has("b") {
		t.Error("imported registry should have both agents")
	}
}

func TestFullFlow_Discover(t *testing.T) {
	card := models.AgentCard{
		Name:        "discoverable",
		Description: "Found via discovery",
		URL:         "http://example.com/a2a",
		Skills:      []models.Skill{{ID: "test", Name: "Test"}},
	}

	srv := agent.NewServer(card, &agent.EchoHandler{})
	ts := httptest.NewServer(srv)
	defer ts.Close()

	regPath := filepath.Join(t.TempDir(), "reg.json")
	// Discover via .well-known, say yes, pick http
	input := "y\nhttp\n"
	out := &bytes.Buffer{}

	if err := cli.Discover(regPath, ts.URL, strings.NewReader(input), out); err != nil {
		t.Fatal(err)
	}

	reg, _ := registry.Load(regPath)
	if !reg.Has("discoverable") {
		t.Error("discovered agent should be in registry")
	}
}

//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luischavesdev/society/internal/agent"
	"github.com/luischavesdev/society/internal/cli"
	"github.com/luischavesdev/society/internal/client"
	"github.com/luischavesdev/society/internal/models"
	"github.com/luischavesdev/society/internal/registry"
)

// --- HTTP Transport Tests ---

func TestHTTP_Echo(t *testing.T) {
	card := models.AgentCard{Name: "echo", URL: "http://placeholder"}
	srv := agent.NewServer(card, &agent.EchoHandler{})
	ts := httptest.NewServer(srv)
	defer ts.Close()

	regPath := filepath.Join(t.TempDir(), "reg.json")
	rf := models.RegistryFile{Agents: []models.AgentCard{{Name: "echo", URL: ts.URL}}}
	data, _ := json.MarshalIndent(rf, "", "  ")
	os.WriteFile(regPath, data, 0644)

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

func TestHTTP_Greeter(t *testing.T) {
	card := models.AgentCard{Name: "greeter", URL: "http://placeholder"}
	srv := agent.NewServer(card, &agent.GreeterHandler{})
	ts := httptest.NewServer(srv)
	defer ts.Close()

	regPath := filepath.Join(t.TempDir(), "reg.json")
	rf := models.RegistryFile{Agents: []models.AgentCard{{Name: "greeter", URL: ts.URL}}}
	data, _ := json.MarshalIndent(rf, "", "  ")
	os.WriteFile(regPath, data, 0644)

	reg, _ := registry.Load(regPath)
	c := client.New(reg)
	task, err := c.Send(context.Background(), "greeter", "world")
	if err != nil {
		t.Fatal(err)
	}

	if task.Status.State != models.TaskStateCompleted {
		t.Errorf("got state %s", task.Status.State)
	}
	want := "Hello! You said: world"
	if task.Artifacts[0].Parts[0].Text != want {
		t.Errorf("got %q, want %q", task.Artifacts[0].Parts[0].Text, want)
	}
}

func TestHTTP_ThreadID(t *testing.T) {
	card := models.AgentCard{Name: "echo", URL: "http://placeholder"}
	srv := agent.NewServer(card, &agent.EchoHandler{})
	ts := httptest.NewServer(srv)
	defer ts.Close()

	regPath := filepath.Join(t.TempDir(), "reg.json")
	rf := models.RegistryFile{Agents: []models.AgentCard{{Name: "echo", URL: ts.URL}}}
	data, _ := json.MarshalIndent(rf, "", "  ")
	os.WriteFile(regPath, data, 0644)

	reg, _ := registry.Load(regPath)
	c := client.New(reg)
	task, err := c.Send(context.Background(), "echo", "test", "my-thread-id")
	if err != nil {
		t.Fatal(err)
	}
	if task.ID != "my-thread-id" {
		t.Errorf("got task ID %q, want my-thread-id", task.ID)
	}
}

// --- STDIO Transport Tests ---

func TestSTDIO_Echo(t *testing.T) {
	binPath := filepath.Join(t.TempDir(), "society")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/society")
	buildCmd.Dir = projectRoot(t)
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %s\n%s", err, out)
	}

	echoConfig := filepath.Join(projectRoot(t), "agents", "echo.yaml")
	regPath := filepath.Join(t.TempDir(), "reg.json")
	rf := models.RegistryFile{Agents: []models.AgentCard{{
		Name: "stdio-echo",
		URL:  "http://localhost:0",
		Transport: &models.TransportConfig{
			Type: "stdio",
			Config: map[string]string{
				"command": binPath,
				"args":    "run --config " + echoConfig + " --stdio",
			},
		},
	}}}
	data, _ := json.MarshalIndent(rf, "", "  ")
	os.WriteFile(regPath, data, 0644)

	reg, _ := registry.Load(regPath)
	c := client.New(reg)
	task, err := c.Send(context.Background(), "stdio-echo", "stdio test")
	if err != nil {
		t.Fatal(err)
	}

	if task.Status.State != models.TaskStateCompleted {
		t.Errorf("got state %s", task.Status.State)
	}
	if task.Artifacts[0].Parts[0].Text != "stdio test" {
		t.Errorf("got %q", task.Artifacts[0].Parts[0].Text)
	}
}

// --- Spawn-on-demand (local exec) ---

func TestSendLocal_SpawnOnDemand(t *testing.T) {
	regPath := filepath.Join(t.TempDir(), "reg.json")
	echoConfig := filepath.Join(projectRoot(t), "agents", "echo.yaml")
	rf := models.RegistryFile{Agents: []models.AgentCard{{
		Name:       "local-echo",
		ConfigPath: echoConfig,
	}}}
	data, _ := json.MarshalIndent(rf, "", "  ")
	os.WriteFile(regPath, data, 0644)

	out := &bytes.Buffer{}
	if err := cli.Send(regPath, "local-echo", "spawn test", out); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "spawn test") {
		t.Errorf("output: %s", out.String())
	}
	if !strings.Contains(out.String(), "Thread") {
		t.Errorf("should print thread ID, output: %s", out.String())
	}
}

// --- Onboard + Send Flow ---

func TestFullFlow_Onboard_And_Send(t *testing.T) {
	card := models.AgentCard{Name: "test-echo", URL: "http://placeholder"}
	srv := agent.NewServer(card, &agent.EchoHandler{})
	ts := httptest.NewServer(srv)
	defer ts.Close()

	regPath := filepath.Join(t.TempDir(), "reg.json")

	input := "test-echo\nEcho test\nhttp\n" + ts.URL + "\n\n"
	out := &bytes.Buffer{}
	if err := cli.Onboard(regPath, strings.NewReader(input), out); err != nil {
		t.Fatal(err)
	}

	sendOut := &bytes.Buffer{}
	if err := cli.Send(regPath, "test-echo", "integration test", sendOut); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(sendOut.String(), "integration test") {
		t.Errorf("output: %s", sendOut.String())
	}
}

// --- Export/Import Flow ---

func TestFullFlow_Export_Import(t *testing.T) {
	regPath := filepath.Join(t.TempDir(), "reg.json")
	rf := models.RegistryFile{Agents: []models.AgentCard{
		{Name: "a", URL: "http://a"},
		{Name: "b", URL: "http://b"},
	}}
	data, _ := json.MarshalIndent(rf, "", "  ")
	os.WriteFile(regPath, data, 0644)

	exportPath := filepath.Join(t.TempDir(), "export.json")
	out := &bytes.Buffer{}
	if err := cli.Export(regPath, exportPath, out); err != nil {
		t.Fatal(err)
	}

	newRegPath := filepath.Join(t.TempDir(), "new.json")
	importOut := &bytes.Buffer{}
	if err := cli.Import(newRegPath, exportPath, strings.NewReader(""), importOut); err != nil {
		t.Fatal(err)
	}

	reg, _ := registry.Load(newRegPath)
	if !reg.Has("a") || !reg.Has("b") {
		t.Error("imported registry should have both agents")
	}
}

// --- Discovery Flow ---

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

// --- Helpers ---

func projectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root")
		}
		dir = parent
	}
}

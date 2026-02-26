package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luischavesdev/society/internal/models"
)

func TestList_WithAgents(t *testing.T) {
	regPath := filepath.Join(t.TempDir(), "reg.json")
	rf := models.RegistryFile{Agents: []models.AgentCard{
		{Name: "echo", URL: "http://localhost:8001", Skills: []models.Skill{{ID: "echo", Name: "Echo"}}},
		{Name: "greeter", URL: "http://localhost:8002"},
	}}
	data, _ := json.MarshalIndent(rf, "", "  ")
	os.WriteFile(regPath, data, 0644)

	out := &bytes.Buffer{}
	if err := List(regPath, out); err != nil {
		t.Fatal(err)
	}

	output := out.String()
	if !strings.Contains(output, "echo") {
		t.Error("output should contain echo")
	}
	if !strings.Contains(output, "greeter") {
		t.Error("output should contain greeter")
	}
	if !strings.Contains(output, "2 agents registered") {
		t.Error("output should show count")
	}
}

func TestList_Empty(t *testing.T) {
	regPath := filepath.Join(t.TempDir(), "reg.json")
	out := &bytes.Buffer{}
	if err := List(regPath, out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "No agents registered") {
		t.Error("should show no agents message")
	}
}

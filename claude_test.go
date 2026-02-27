//go:build claude

package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luischavesdev/society/internal/cli"
	"github.com/luischavesdev/society/internal/models"
)

func TestClaude_ExecPong(t *testing.T) {
	regPath := filepath.Join(t.TempDir(), "reg.json")
	claudeConfig := filepath.Join(mustProjectRoot(t), "agents", "claude.yaml")
	rf := models.RegistryFile{Agents: []models.AgentCard{{
		Name:       "claude",
		ConfigPath: claudeConfig,
	}}}
	data, _ := json.MarshalIndent(rf, "", "  ")
	mustWriteFile(t, regPath, data)

	out := &bytes.Buffer{}
	err := cli.Send(regPath, "claude", "respond with exactly: PONG", out)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "PONG") {
		t.Errorf("expected PONG in output, got: %s", out.String())
	}
}

func TestClaude_ThreadContinuation(t *testing.T) {
	regPath := filepath.Join(t.TempDir(), "reg.json")
	claudeConfig := filepath.Join(mustProjectRoot(t), "agents", "claude.yaml")
	rf := models.RegistryFile{Agents: []models.AgentCard{{
		Name:       "claude",
		ConfigPath: claudeConfig,
	}}}
	data, _ := json.MarshalIndent(rf, "", "  ")
	mustWriteFile(t, regPath, data)

	threadID := "test-thread-123"

	// First message
	out1 := &bytes.Buffer{}
	err := cli.Send(regPath, "claude", "remember the word BANANA", out1, threadID)
	if err != nil {
		t.Fatal(err)
	}

	// Follow-up using same thread
	out2 := &bytes.Buffer{}
	err = cli.Send(regPath, "claude", "what word did I ask you to remember?", out2, threadID)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(strings.ToUpper(out2.String()), "BANANA") {
		t.Errorf("expected BANANA in follow-up, got: %s", out2.String())
	}
}

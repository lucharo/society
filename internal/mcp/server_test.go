package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luischavesdev/society/internal/models"
	"github.com/luischavesdev/society/internal/registry"
)

type mockSender struct {
	lastAgent   string
	lastMessage string
	lastThread  string
	task        *models.Task
	err         error
}

func (m *mockSender) Send(ctx context.Context, agentName, text string, threadID ...string) (*models.Task, error) {
	m.lastAgent = agentName
	m.lastMessage = text
	if len(threadID) > 0 {
		m.lastThread = threadID[0]
	}
	return m.task, m.err
}

func testRegistry(t *testing.T) (string, *registry.Registry) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.json")

	rf := models.RegistryFile{
		Agents: []models.AgentCard{
			{Name: "echo", Description: "Echoes messages back", URL: "http://localhost:8001"},
			{Name: "arch-claude", Description: "Claude on arch", URL: "http://localhost:8003"},
		},
	}
	data, _ := json.MarshalIndent(rf, "", "  ")
	os.WriteFile(path, data, 0644)

	reg, _ := registry.Load(path)
	return path, reg
}

func runRequest(t *testing.T, srv *Server, request string) map[string]any {
	t.Helper()
	in := strings.NewReader(request + "\n")
	out := &bytes.Buffer{}
	srv.in = in
	srv.out = out
	srv.Run(context.Background())

	var resp map[string]any
	if out.Len() > 0 {
		json.Unmarshal(out.Bytes(), &resp)
	}
	return resp
}

func TestInitialize(t *testing.T) {
	path, reg := testRegistry(t)
	sender := &mockSender{}
	srv := NewServer(path, reg, sender, nil, nil)

	resp := runRequest(t, srv, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)

	if resp["jsonrpc"] != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %v", resp["jsonrpc"])
	}

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatal("expected result to be a map")
	}
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("expected protocolVersion 2024-11-05, got %v", result["protocolVersion"])
	}

	serverInfo, ok := result["serverInfo"].(map[string]any)
	if !ok {
		t.Fatal("expected serverInfo to be a map")
	}
	if serverInfo["name"] != "society" {
		t.Errorf("expected server name society, got %v", serverInfo["name"])
	}

	caps, ok := result["capabilities"].(map[string]any)
	if !ok {
		t.Fatal("expected capabilities to be a map")
	}
	if _, ok := caps["tools"]; !ok {
		t.Error("expected tools capability")
	}
}

func TestNotificationIgnored(t *testing.T) {
	path, reg := testRegistry(t)
	sender := &mockSender{}
	srv := NewServer(path, reg, sender, nil, nil)

	// No id field — this is a notification.
	in := strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n")
	out := &bytes.Buffer{}
	srv.in = in
	srv.out = out
	srv.Run(context.Background())

	if out.Len() != 0 {
		t.Errorf("expected no response for notification, got %s", out.String())
	}
}

func TestToolsList(t *testing.T) {
	path, reg := testRegistry(t)
	sender := &mockSender{}
	srv := NewServer(path, reg, sender, nil, nil)

	resp := runRequest(t, srv, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatal("expected result to be a map")
	}

	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatal("expected tools to be an array")
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	// Check naming: echo -> send_echo, arch-claude -> send_arch_claude
	tool0 := tools[0].(map[string]any)
	tool1 := tools[1].(map[string]any)

	if tool0["name"] != "send_echo" {
		t.Errorf("expected send_echo, got %v", tool0["name"])
	}
	if tool1["name"] != "send_arch_claude" {
		t.Errorf("expected send_arch_claude, got %v", tool1["name"])
	}

	// Check description includes agent description.
	desc := tool0["description"].(string)
	if !strings.Contains(desc, "Echoes messages back") {
		t.Errorf("expected description to contain agent description, got %s", desc)
	}

	// Check inputSchema has required message field.
	schema := tool0["inputSchema"].(map[string]any)
	props := schema["properties"].(map[string]any)
	if _, ok := props["message"]; !ok {
		t.Error("expected message property in schema")
	}
	if _, ok := props["thread_id"]; !ok {
		t.Error("expected thread_id property in schema")
	}
}

func TestToolsCallSuccess(t *testing.T) {
	path, reg := testRegistry(t)
	sender := &mockSender{
		task: &models.Task{
			ID:     "task-1",
			Status: models.TaskStatus{State: models.TaskStateCompleted},
			Artifacts: []models.Artifact{
				{Parts: []models.Part{{Type: "text", Text: "Hello back!"}}},
			},
		},
	}
	srv := NewServer(path, reg, sender, nil, nil)

	// First populate the toolMap by listing tools.
	runRequest(t, srv, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)

	resp := runRequest(t, srv, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"send_echo","arguments":{"message":"hello"}}}`)

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatal("expected result to be a map")
	}

	content := result["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(content))
	}
	item := content[0].(map[string]any)
	if item["text"] != "Hello back!" {
		t.Errorf("expected 'Hello back!', got %v", item["text"])
	}
	if _, ok := result["isError"]; ok {
		t.Error("expected no isError on success")
	}
}

func TestToolsCallWithThreadID(t *testing.T) {
	path, reg := testRegistry(t)
	sender := &mockSender{
		task: &models.Task{
			ID:     "task-1",
			Status: models.TaskStatus{State: models.TaskStateCompleted},
			Artifacts: []models.Artifact{
				{Parts: []models.Part{{Type: "text", Text: "continued"}}},
			},
		},
	}
	srv := NewServer(path, reg, sender, nil, nil)

	runRequest(t, srv, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	runRequest(t, srv, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"send_echo","arguments":{"message":"hello","thread_id":"thread-123"}}}`)

	if sender.lastThread != "thread-123" {
		t.Errorf("expected thread_id thread-123, got %s", sender.lastThread)
	}
}

func TestToolsCallAgentError(t *testing.T) {
	path, reg := testRegistry(t)
	sender := &mockSender{
		err: fmt.Errorf("connection refused"),
	}
	srv := NewServer(path, reg, sender, nil, nil)

	runRequest(t, srv, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	resp := runRequest(t, srv, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"send_echo","arguments":{"message":"hello"}}}`)

	result := resp["result"].(map[string]any)
	if result["isError"] != true {
		t.Error("expected isError to be true")
	}
	content := result["content"].([]any)
	item := content[0].(map[string]any)
	if !strings.Contains(item["text"].(string), "connection refused") {
		t.Errorf("expected error message to contain 'connection refused', got %v", item["text"])
	}
}

func TestToolsCallFailedTask(t *testing.T) {
	path, reg := testRegistry(t)
	sender := &mockSender{
		task: &models.Task{
			ID:     "task-1",
			Status: models.TaskStatus{State: models.TaskStateFailed, Message: "agent crashed"},
		},
	}
	srv := NewServer(path, reg, sender, nil, nil)

	runRequest(t, srv, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	resp := runRequest(t, srv, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"send_echo","arguments":{"message":"hello"}}}`)

	result := resp["result"].(map[string]any)
	if result["isError"] != true {
		t.Error("expected isError to be true")
	}
	content := result["content"].([]any)
	item := content[0].(map[string]any)
	if item["text"] != "agent crashed" {
		t.Errorf("expected 'agent crashed', got %v", item["text"])
	}
}

func TestUnknownMethod(t *testing.T) {
	path, reg := testRegistry(t)
	sender := &mockSender{}
	srv := NewServer(path, reg, sender, nil, nil)

	resp := runRequest(t, srv, `{"jsonrpc":"2.0","id":1,"method":"foo/bar"}`)

	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatal("expected error object")
	}
	if errObj["code"].(float64) != -32601 {
		t.Errorf("expected error code -32601, got %v", errObj["code"])
	}
	if !strings.Contains(errObj["message"].(string), "method not found") {
		t.Errorf("expected 'method not found' in message, got %v", errObj["message"])
	}
}

func TestUnknownTool(t *testing.T) {
	path, reg := testRegistry(t)
	sender := &mockSender{}
	srv := NewServer(path, reg, sender, nil, nil)

	// Don't call tools/list first, so toolMap is empty.
	resp := runRequest(t, srv, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"send_nonexistent","arguments":{"message":"hi"}}}`)

	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatal("expected error object")
	}
	if errObj["code"].(float64) != -32602 {
		t.Errorf("expected error code -32602, got %v", errObj["code"])
	}
}

func TestMissingMessage(t *testing.T) {
	path, reg := testRegistry(t)
	sender := &mockSender{}
	srv := NewServer(path, reg, sender, nil, nil)

	runRequest(t, srv, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	resp := runRequest(t, srv, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"send_echo","arguments":{}}}`)

	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatal("expected error object")
	}
	if errObj["code"].(float64) != -32602 {
		t.Errorf("expected error code -32602, got %v", errObj["code"])
	}
	if !strings.Contains(errObj["message"].(string), "message is required") {
		t.Errorf("expected 'message is required' in message, got %v", errObj["message"])
	}
}

func TestParseError(t *testing.T) {
	path, reg := testRegistry(t)
	sender := &mockSender{}
	srv := NewServer(path, reg, sender, nil, nil)

	resp := runRequest(t, srv, `not valid json`)

	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatal("expected error object")
	}
	if errObj["code"].(float64) != -32700 {
		t.Errorf("expected error code -32700, got %v", errObj["code"])
	}
}

func TestStringID(t *testing.T) {
	path, reg := testRegistry(t)
	sender := &mockSender{}
	srv := NewServer(path, reg, sender, nil, nil)

	resp := runRequest(t, srv, `{"jsonrpc":"2.0","id":"abc-123","method":"initialize"}`)

	// The id should be preserved as a string.
	id, ok := resp["id"].(string)
	if !ok {
		t.Fatalf("expected string id, got %T: %v", resp["id"], resp["id"])
	}
	if id != "abc-123" {
		t.Errorf("expected id abc-123, got %s", id)
	}
}

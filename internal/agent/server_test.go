package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/luischavesdev/society/internal/models"
)

func testServer() *Server {
	card := models.AgentCard{
		Name:        "test",
		URL:         "http://localhost:9999",
		Description: "Test agent",
		Skills:      []models.Skill{{ID: "echo", Name: "Echo"}},
	}
	return NewServer(card, &EchoHandler{})
}

type failHandler struct{}

func (h *failHandler) Handle(_ context.Context, _ *models.SendTaskParams) (*models.Task, error) {
	return nil, fmt.Errorf("handler failed")
}

func TestServer_AgentCard(t *testing.T) {
	srv := testServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/.well-known/agent.json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("got status %d", resp.StatusCode)
	}

	var card models.AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		t.Fatal(err)
	}
	if card.Name != "test" {
		t.Errorf("got name %q, want test", card.Name)
	}
}

func postRPC(ts *httptest.Server, body string) (*http.Response, error) {
	return http.Post(ts.URL+"/", "application/json", strings.NewReader(body))
}

func decodeRPCResponse(t *testing.T, resp *http.Response) models.JSONRPCResponse {
	t.Helper()
	var rpc models.JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpc); err != nil {
		t.Fatal(err)
	}
	return rpc
}

func TestServer_ValidSendTask(t *testing.T) {
	srv := testServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := `{"jsonrpc":"2.0","id":"1","method":"tasks/send","params":{"id":"t1","message":{"role":"user","parts":[{"type":"text","text":"hello"}]}}}`
	resp, err := postRPC(ts, body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	rpc := decodeRPCResponse(t, resp)
	if rpc.Error != nil {
		t.Fatalf("unexpected error: %v", rpc.Error)
	}
	if rpc.Result == nil {
		t.Fatal("expected result")
	}
}

func TestServer_InvalidJSON(t *testing.T) {
	srv := testServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, _ := postRPC(ts, `{invalid`)
	defer resp.Body.Close()

	rpc := decodeRPCResponse(t, resp)
	if rpc.Error == nil || rpc.Error.Code != -32700 {
		t.Errorf("expected -32700, got %v", rpc.Error)
	}
}

func TestServer_InvalidRequest(t *testing.T) {
	srv := testServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Missing jsonrpc field
	resp, _ := postRPC(ts, `{"id":"1","method":"tasks/send"}`)
	defer resp.Body.Close()

	rpc := decodeRPCResponse(t, resp)
	if rpc.Error == nil || rpc.Error.Code != -32600 {
		t.Errorf("expected -32600, got %v", rpc.Error)
	}
}

func TestServer_UnknownMethod(t *testing.T) {
	srv := testServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, _ := postRPC(ts, `{"jsonrpc":"2.0","id":"1","method":"unknown"}`)
	defer resp.Body.Close()

	rpc := decodeRPCResponse(t, resp)
	if rpc.Error == nil || rpc.Error.Code != -32601 {
		t.Errorf("expected -32601, got %v", rpc.Error)
	}
}

func TestServer_InvalidParams(t *testing.T) {
	srv := testServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Missing task ID
	resp, _ := postRPC(ts, `{"jsonrpc":"2.0","id":"1","method":"tasks/send","params":{"message":{"role":"user","parts":[{"type":"text"}]}}}`)
	defer resp.Body.Close()

	rpc := decodeRPCResponse(t, resp)
	if rpc.Error == nil || rpc.Error.Code != -32602 {
		t.Errorf("expected -32602, got %v", rpc.Error)
	}
}

func TestServer_HandlerError(t *testing.T) {
	card := models.AgentCard{Name: "fail", URL: "http://localhost:9999"}
	srv := NewServer(card, &failHandler{})
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := `{"jsonrpc":"2.0","id":"1","method":"tasks/send","params":{"id":"t1","message":{"role":"user","parts":[{"type":"text","text":"hi"}]}}}`
	resp, _ := postRPC(ts, body)
	defer resp.Body.Close()

	rpc := decodeRPCResponse(t, resp)
	if rpc.Error == nil || rpc.Error.Code != -32603 {
		t.Errorf("expected -32603, got %v", rpc.Error)
	}
}

func TestServer_NotFound(t *testing.T) {
	srv := testServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/nonexistent")
	if resp.StatusCode != 404 {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestServer_MethodNotAllowed(t *testing.T) {
	srv := testServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/")
	if resp.StatusCode != 405 {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
}

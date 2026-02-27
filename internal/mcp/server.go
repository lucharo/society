package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/luischavesdev/society/internal/models"
	"github.com/luischavesdev/society/internal/registry"
)

// Sender abstracts sending messages to agents (client.Client implements this).
type Sender interface {
	Send(ctx context.Context, agentName, text string, threadID ...string) (*models.Task, error)
}

// Server implements an MCP server over stdio, exposing registered agents as tools.
type Server struct {
	registryPath string
	registry     *registry.Registry
	sender       Sender
	in           io.Reader
	out          io.Writer
	toolMap      map[string]string // "send_echo" -> "echo"
}

// NewServer creates a new MCP server.
func NewServer(registryPath string, reg *registry.Registry, sender Sender, in io.Reader, out io.Writer) *Server {
	s := &Server{
		registryPath: registryPath,
		registry:     reg,
		sender:       sender,
		in:           in,
		out:          out,
		toolMap:      make(map[string]string),
	}
	s.rebuildToolMap(reg)
	return s
}

// Run reads JSON-RPC 2.0 requests from stdin and writes responses to stdout.
func (s *Server) Run(ctx context.Context) error {
	scanner := bufio.NewScanner(s.in)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(line, &raw); err != nil {
			s.writeError(nil, -32700, "parse error")
			continue
		}

		// Check if this is a notification (no id field).
		_, hasID := raw["id"]

		var method string
		if m, ok := raw["method"]; ok {
			json.Unmarshal(m, &method)
		}

		if !hasID {
			// Notification — no response needed.
			continue
		}

		id := raw["id"]

		switch method {
		case "initialize":
			s.handleInitialize(id)
		case "tools/list":
			s.handleToolsList(id)
		case "tools/call":
			s.handleToolsCall(ctx, id, raw["params"])
		default:
			s.writeError(id, -32601, fmt.Sprintf("method not found: %s", method))
		}
	}
	return scanner.Err()
}

func (s *Server) handleInitialize(id json.RawMessage) {
	result := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "society",
			"version": "0.1.0",
		},
	}
	s.writeResult(id, result)
}

func (s *Server) rebuildToolMap(reg *registry.Registry) {
	s.toolMap = make(map[string]string)
	for _, a := range reg.List() {
		toolName := agentToToolName(a.Name)
		s.toolMap[toolName] = a.Name
	}
}

func agentToToolName(name string) string {
	return "send_" + strings.ReplaceAll(name, "-", "_")
}

func (s *Server) handleToolsList(id json.RawMessage) {
	reg, err := registry.Load(s.registryPath)
	if err != nil {
		s.writeError(id, -32603, fmt.Sprintf("loading registry: %v", err))
		return
	}
	s.registry = reg
	s.rebuildToolMap(reg)

	var tools []map[string]any
	seen := make(map[string]bool)
	for _, agent := range reg.List() {
		toolName := agentToToolName(agent.Name)
		if seen[toolName] {
			continue // skip collision — first agent wins
		}
		seen[toolName] = true

		desc := fmt.Sprintf("Send message to %s agent", agent.Name)
		if agent.Description != "" {
			desc += ": " + agent.Description
		}

		tool := map[string]any{
			"name":        toolName,
			"description": desc,
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"message": map[string]any{
						"type":        "string",
						"description": "The message to send to the agent",
					},
					"thread_id": map[string]any{
						"type":        "string",
						"description": "Optional thread ID to continue a conversation",
					},
				},
				"required": []string{"message"},
			},
		}
		tools = append(tools, tool)
	}

	s.writeResult(id, map[string]any{"tools": tools})
}

func (s *Server) handleToolsCall(ctx context.Context, id json.RawMessage, paramsRaw json.RawMessage) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(paramsRaw, &params); err != nil {
		s.writeError(id, -32602, "invalid params")
		return
	}

	agentName, ok := s.toolMap[params.Name]
	if !ok {
		s.writeError(id, -32602, fmt.Sprintf("unknown tool: %s", params.Name))
		return
	}

	var args struct {
		Message  string `json:"message"`
		ThreadID string `json:"thread_id"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		s.writeError(id, -32602, "invalid arguments")
		return
	}

	if args.Message == "" {
		s.writeError(id, -32602, "message is required")
		return
	}

	var threadID []string
	if args.ThreadID != "" {
		threadID = []string{args.ThreadID}
	}

	task, err := s.sender.Send(ctx, agentName, args.Message, threadID...)
	if err != nil {
		s.writeResult(id, map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
			},
			"isError": true,
		})
		return
	}

	var text string
	for _, a := range task.Artifacts {
		for _, p := range a.Parts {
			if p.Type == "text" {
				if text != "" {
					text += "\n"
				}
				text += p.Text
			}
		}
	}

	if task.Status.State == models.TaskStateFailed {
		msg := task.Status.Message
		if text != "" {
			msg = text
		}
		s.writeResult(id, map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": msg},
			},
			"isError": true,
		})
		return
	}

	s.writeResult(id, map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": text},
		},
	})
}

func (s *Server) writeResult(id json.RawMessage, result any) {
	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(id),
		"result":  result,
	}
	b, _ := json.Marshal(resp)
	fmt.Fprintln(s.out, string(b))
}

func (s *Server) writeError(id json.RawMessage, code int, msg string) {
	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      nil,
		"error": map[string]any{
			"code":    code,
			"message": msg,
		},
	}
	if id != nil {
		resp["id"] = json.RawMessage(id)
	}
	b, _ := json.Marshal(resp)
	fmt.Fprintln(s.out, string(b))
}

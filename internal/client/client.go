package client

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/luischavesdev/society/internal/models"
	"github.com/luischavesdev/society/internal/registry"
	"github.com/luischavesdev/society/internal/transport"
)

type Client struct {
	registry *registry.Registry
}

func New(reg *registry.Registry) *Client {
	return &Client{registry: reg}
}

func (c *Client) Send(ctx context.Context, agentName, text string, threadID ...string) (*models.Task, error) {
	card, err := c.registry.Get(agentName)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	tr, err := transport.New(card.URL, card.Transport)
	if err != nil {
		return nil, fmt.Errorf("creating transport: %w", err)
	}
	if err := tr.Open(ctx); err != nil {
		return nil, fmt.Errorf("opening transport: %w", err)
	}
	defer tr.Close()

	taskID := uuid.New().String()
	if len(threadID) > 0 && threadID[0] != "" {
		taskID = threadID[0]
	}
	req := models.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      taskID,
		Method:  "tasks/send",
		Params: models.SendTaskParams{
			ID: taskID,
			Message: models.Message{
				Role:  "user",
				Parts: []models.Part{{Type: "text", Text: text}},
			},
		},
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	respBytes, err := tr.Send(ctx, payload)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}

	var resp models.JSONRPCResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("agent error [%d]: %s", resp.Error.Code, resp.Error.Message)
	}

	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("marshaling result: %w", err)
	}
	var task models.Task
	if err := json.Unmarshal(resultBytes, &task); err != nil {
		return nil, fmt.Errorf("parsing task: %w", err)
	}
	return &task, nil
}

func (c *Client) Ping(ctx context.Context, agentName string) (*models.AgentCard, error) {
	card, err := c.registry.Get(agentName)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	tr, err := transport.New(card.URL, card.Transport)
	if err != nil {
		return nil, fmt.Errorf("creating transport: %w", err)
	}
	if err := tr.Open(ctx); err != nil {
		return nil, fmt.Errorf("opening transport: %w", err)
	}
	defer tr.Close()

	pingReq := models.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "ping",
		Method:  "tasks/send",
		Params: models.SendTaskParams{
			ID: "ping-" + uuid.New().String(),
			Message: models.Message{
				Role:  "user",
				Parts: []models.Part{{Type: "text", Text: "ping"}},
			},
		},
	}

	payload, err := json.Marshal(pingReq)
	if err != nil {
		return nil, fmt.Errorf("marshaling ping: %w", err)
	}

	_, err = tr.Send(ctx, payload)
	if err != nil {
		return nil, fmt.Errorf("ping failed: %w", err)
	}

	return &card, nil
}

package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/luischavesdev/society/internal/agent"
	"github.com/luischavesdev/society/internal/models"
)

func Run(configPath string, stdio bool, out io.Writer) error {
	cfg, err := agent.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	h, err := agent.NewHandler(cfg.Handler)
	if err != nil {
		return err
	}

	card := models.AgentCard{
		Name:        cfg.Name,
		Description: cfg.Description,
		URL:         fmt.Sprintf("http://localhost:%d", cfg.Port),
		Skills:      cfg.Skills,
	}

	if stdio {
		return runSTDIO(h, out)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	srv := agent.NewServer(card, h)
	addr := fmt.Sprintf(":%d", cfg.Port)
	fmt.Fprintf(out, "Starting agent %q on %s\n", cfg.Name, addr)
	return srv.Start(ctx, addr)
}

func runSTDIO(h agent.Handler, out io.Writer) error {
	slog.Info("running in STDIO mode")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Bytes()

		var req models.JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			writeSTDIOError(out, nil, -32700, "parse error")
			continue
		}

		if rpcErr := models.ValidateJSONRPCRequest(req); rpcErr != nil {
			writeSTDIOError(out, req.ID, rpcErr.Code, rpcErr.Message)
			continue
		}

		if req.Method != "tasks/send" {
			writeSTDIOError(out, req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method))
			continue
		}

		paramsBytes, _ := json.Marshal(req.Params)
		var params models.SendTaskParams
		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			writeSTDIOError(out, req.ID, -32602, "invalid params")
			continue
		}

		if rpcErr := models.ValidateSendTaskParams(params); rpcErr != nil {
			writeSTDIOError(out, req.ID, rpcErr.Code, rpcErr.Message)
			continue
		}

		task, err := h.Handle(context.Background(), &params)
		if err != nil {
			writeSTDIOError(out, req.ID, -32603, fmt.Sprintf("handler error: %v", err))
			continue
		}

		resp := models.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: task}
		b, _ := json.Marshal(resp)
		fmt.Fprintln(out, string(b))
	}
	return scanner.Err()
}

func writeSTDIOError(w io.Writer, id any, code int, msg string) {
	resp := models.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &models.JSONRPCError{Code: code, Message: msg},
	}
	b, _ := json.Marshal(resp)
	fmt.Fprintln(w, string(b))
}

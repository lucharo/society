package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/luischavesdev/society/internal/models"
)

type Server struct {
	card    models.AgentCard
	handler Handler
	srv     *http.Server
}

func NewServer(card models.AgentCard, h Handler) *Server {
	return &Server{card: card, handler: h}
}

func (s *Server) Start(ctx context.Context, addr string) error {
	s.srv = &http.Server{Addr: addr, Handler: s}
	slog.Info("agent starting", "name", s.card.Name, "addr", addr)

	go func() {
		<-ctx.Done()
		s.srv.Close()
	}()

	err := s.srv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/.well-known/agent.json" && r.Method == http.MethodGet:
		s.handleAgentCard(w)
	case r.URL.Path == "/" && r.Method == http.MethodPost:
		s.handleRPC(w, r)
	case r.URL.Path == "/" && r.Method != http.MethodPost:
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleAgentCard(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.card)
}

func (s *Server) handleRPC(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeRPCError(w, nil, -32700, "could not read body")
		return
	}

	var req models.JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeRPCError(w, nil, -32700, "parse error")
		return
	}

	if rpcErr := models.ValidateJSONRPCRequest(req); rpcErr != nil {
		writeRPCError(w, req.ID, rpcErr.Code, rpcErr.Message)
		return
	}

	switch req.Method {
	case "tasks/send":
		s.handleSendTask(w, r, req)
	default:
		writeRPCError(w, req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method))
	}
}

func (s *Server) handleSendTask(w http.ResponseWriter, r *http.Request, req models.JSONRPCRequest) {
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		writeRPCError(w, req.ID, -32602, "invalid params")
		return
	}

	var params models.SendTaskParams
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		writeRPCError(w, req.ID, -32602, "invalid params")
		return
	}

	if rpcErr := models.ValidateSendTaskParams(params); rpcErr != nil {
		writeRPCError(w, req.ID, rpcErr.Code, rpcErr.Message)
		return
	}

	task, err := s.handler.Handle(r.Context(), &params)
	if err != nil {
		writeRPCError(w, req.ID, -32603, fmt.Sprintf("handler error: %v", err))
		return
	}

	writeRPCResult(w, req.ID, task)
}

func writeRPCError(w http.ResponseWriter, id any, code int, msg string) {
	resp := models.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &models.JSONRPCError{Code: code, Message: msg},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func writeRPCResult(w http.ResponseWriter, id any, result any) {
	resp := models.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

package models

import (
	"strings"
	"testing"
)

func TestValidateRegistry(t *testing.T) {
	tests := []struct {
		name    string
		agents  []AgentCard
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid",
			agents: []AgentCard{
				{Name: "a", URL: "http://localhost:8001"},
				{Name: "b", URL: "http://localhost:8002"},
			},
		},
		{
			name:    "missing name",
			agents:  []AgentCard{{URL: "http://localhost:8001"}},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name:    "missing url and config_path",
			agents:  []AgentCard{{Name: "a"}},
			wantErr: true,
			errMsg:  "url or config_path is required",
		},
		{
			name:   "config_path instead of url",
			agents: []AgentCard{{Name: "a", ConfigPath: "agents/claude.yaml"}},
		},
		{
			name: "duplicate names",
			agents: []AgentCard{
				{Name: "a", URL: "http://localhost:8001"},
				{Name: "a", URL: "http://localhost:8002"},
			},
			wantErr: true,
			errMsg:  "duplicate name",
		},
		{
			name: "bad transport type",
			agents: []AgentCard{
				{Name: "a", URL: "http://localhost:8001", Transport: &TransportConfig{Type: "pigeons"}},
			},
			wantErr: true,
			errMsg:  "invalid transport type",
		},
		{
			name: "ssh missing host",
			agents: []AgentCard{
				{Name: "a", URL: "http://localhost:8001", Transport: &TransportConfig{
					Type:   "ssh",
					Config: map[string]string{"user": "u", "key_path": "/k"},
				}},
			},
			wantErr: true,
			errMsg:  "ssh transport requires host",
		},
		{
			name: "docker missing container",
			agents: []AgentCard{
				{Name: "a", URL: "http://localhost:8001", Transport: &TransportConfig{
					Type: "docker",
				}},
			},
			wantErr: true,
			errMsg:  "docker transport requires container",
		},
		{
			name: "stdio missing command",
			agents: []AgentCard{
				{Name: "a", URL: "http://localhost:8001", Transport: &TransportConfig{
					Type: "stdio",
				}},
			},
			wantErr: true,
			errMsg:  "stdio transport requires command",
		},
		{
			name: "multiple errors collected",
			agents: []AgentCard{
				{URL: "http://localhost:8001"},
				{Name: "b"},
			},
			wantErr: true,
			errMsg:  "name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRegistry(tt.agents)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateAgentCard(t *testing.T) {
	tests := []struct {
		name    string
		card    AgentCard
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid",
			card: AgentCard{Name: "a", URL: "http://localhost:8001"},
		},
		{
			name:    "missing name",
			card:    AgentCard{URL: "http://localhost:8001"},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name:    "missing url and config_path",
			card:    AgentCard{Name: "a"},
			wantErr: true,
			errMsg:  "url or config_path is required",
		},
		{
			name: "config_path instead of url",
			card: AgentCard{Name: "a", ConfigPath: "agents/claude.yaml"},
		},
		{
			name: "valid with transport",
			card: AgentCard{
				Name: "a", URL: "http://localhost:8001",
				Transport: &TransportConfig{Type: "http"},
			},
		},
		{
			name: "invalid transport",
			card: AgentCard{
				Name: "a", URL: "http://localhost:8001",
				Transport: &TransportConfig{Type: "bad"},
			},
			wantErr: true,
			errMsg:  "invalid transport type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAgentCard(tt.card)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateTransportConfig(t *testing.T) {
	tests := []struct {
		name    string
		tc      *TransportConfig
		wantErr bool
		errMsg  string
	}{
		{name: "nil", tc: nil},
		{name: "http", tc: &TransportConfig{Type: "http"}},
		{name: "ssh valid", tc: &TransportConfig{Type: "ssh", Config: map[string]string{
			"host": "h", "user": "u", "key_path": "/k",
		}}},
		{name: "docker valid", tc: &TransportConfig{Type: "docker", Config: map[string]string{
			"container": "c",
		}}},
		{name: "stdio valid", tc: &TransportConfig{Type: "stdio", Config: map[string]string{
			"command": "echo",
		}}},
		{
			name:    "unknown type",
			tc:      &TransportConfig{Type: "unknown"},
			wantErr: true,
			errMsg:  "invalid transport type",
		},
		{
			name:    "ssh missing fields",
			tc:      &TransportConfig{Type: "ssh"},
			wantErr: true,
			errMsg:  "ssh transport requires host",
		},
		{
			name:    "docker missing container",
			tc:      &TransportConfig{Type: "docker"},
			wantErr: true,
			errMsg:  "docker transport requires container",
		},
		{
			name:    "stdio missing command",
			tc:      &TransportConfig{Type: "stdio"},
			wantErr: true,
			errMsg:  "stdio transport requires command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTransportConfig(tt.tc)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateJSONRPCRequest(t *testing.T) {
	tests := []struct {
		name     string
		req      JSONRPCRequest
		wantCode int
	}{
		{
			name:     "valid",
			req:      JSONRPCRequest{JSONRPC: "2.0", ID: "1", Method: "tasks/send"},
			wantCode: 0,
		},
		{
			name:     "missing jsonrpc",
			req:      JSONRPCRequest{ID: "1", Method: "tasks/send"},
			wantCode: -32600,
		},
		{
			name:     "wrong version",
			req:      JSONRPCRequest{JSONRPC: "1.0", ID: "1", Method: "tasks/send"},
			wantCode: -32600,
		},
		{
			name:     "missing method",
			req:      JSONRPCRequest{JSONRPC: "2.0", ID: "1"},
			wantCode: -32600,
		},
		{
			name:     "missing id",
			req:      JSONRPCRequest{JSONRPC: "2.0", Method: "tasks/send"},
			wantCode: -32600,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateJSONRPCRequest(tt.req)
			if tt.wantCode == 0 {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if err.Code != tt.wantCode {
					t.Errorf("got code %d, want %d", err.Code, tt.wantCode)
				}
			}
		})
	}
}

func TestValidateSendTaskParams(t *testing.T) {
	validMsg := Message{Role: "user", Parts: []Part{{Type: "text", Text: "hi"}}}

	tests := []struct {
		name     string
		params   SendTaskParams
		wantCode int
	}{
		{
			name:     "valid",
			params:   SendTaskParams{ID: "1", Message: validMsg},
			wantCode: 0,
		},
		{
			name:     "empty task id",
			params:   SendTaskParams{Message: validMsg},
			wantCode: -32602,
		},
		{
			name:     "invalid role",
			params:   SendTaskParams{ID: "1", Message: Message{Role: "system", Parts: []Part{{Type: "text"}}}},
			wantCode: -32602,
		},
		{
			name:     "empty parts",
			params:   SendTaskParams{ID: "1", Message: Message{Role: "user"}},
			wantCode: -32602,
		},
		{
			name: "part missing type",
			params: SendTaskParams{
				ID:      "1",
				Message: Message{Role: "user", Parts: []Part{{Text: "hi"}}},
			},
			wantCode: -32602,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSendTaskParams(tt.params)
			if tt.wantCode == 0 {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if err.Code != tt.wantCode {
					t.Errorf("got code %d, want %d", err.Code, tt.wantCode)
				}
			}
		})
	}
}

func TestValidateAgentConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     AgentConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid echo",
			cfg:  AgentConfig{Name: "a", Port: 8001, Handler: "echo"},
		},
		{
			name: "valid greeter",
			cfg:  AgentConfig{Name: "a", Port: 8002, Handler: "greeter"},
		},
		{
			name:    "missing name",
			cfg:     AgentConfig{Port: 8001, Handler: "echo"},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "port zero allowed",
			cfg:  AgentConfig{Name: "a", Port: 0, Handler: "echo"},
		},
		{
			name:    "port too high",
			cfg:     AgentConfig{Name: "a", Port: 70000, Handler: "echo"},
			wantErr: true,
			errMsg:  "port must be between",
		},
		{
			name: "valid exec",
			cfg:  AgentConfig{Name: "a", Handler: "exec", Backend: &BackendConfig{Command: "claude"}},
		},
		{
			name:    "exec missing backend",
			cfg:     AgentConfig{Name: "a", Handler: "exec"},
			wantErr: true,
			errMsg:  "exec handler requires backend config",
		},
		{
			name:    "exec empty command",
			cfg:     AgentConfig{Name: "a", Handler: "exec", Backend: &BackendConfig{}},
			wantErr: true,
			errMsg:  "exec handler requires backend command",
		},
		{
			name:    "unknown handler",
			cfg:     AgentConfig{Name: "a", Port: 8001, Handler: "unknown"},
			wantErr: true,
			errMsg:  "unknown handler",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAgentConfig(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

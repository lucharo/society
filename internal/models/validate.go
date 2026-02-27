package models

import (
	"fmt"
	"net/url"
	"strings"
)

type ValidationErrors []string

func (ve ValidationErrors) Error() string {
	return strings.Join(ve, "; ")
}

func (ve *ValidationErrors) Add(msg string) {
	*ve = append(*ve, msg)
}

func (ve ValidationErrors) HasErrors() bool {
	return len(ve) > 0
}

var validTransportTypes = map[string]bool{
	"http": true, "ssh": true, "docker": true, "stdio": true,
}

func ValidateRegistry(agents []AgentCard) error {
	var errs ValidationErrors
	seen := map[string]bool{}

	for i, a := range agents {
		prefix := fmt.Sprintf("agent[%d]", i)

		if a.Name == "" {
			errs.Add(fmt.Sprintf("%s: name is required", prefix))
		} else if seen[a.Name] {
			errs.Add(fmt.Sprintf("%s: duplicate name %q", prefix, a.Name))
		} else {
			seen[a.Name] = true
		}

		if a.URL == "" {
			errs.Add(fmt.Sprintf("%s: url is required", prefix))
		} else if _, err := url.Parse(a.URL); err != nil {
			errs.Add(fmt.Sprintf("%s: invalid url: %v", prefix, err))
		}

		if a.Transport != nil {
			if !validTransportTypes[a.Transport.Type] {
				errs.Add(fmt.Sprintf("%s: invalid transport type %q", prefix, a.Transport.Type))
			} else if err := ValidateTransportConfig(a.Transport); err != nil {
				errs.Add(fmt.Sprintf("%s: %v", prefix, err))
			}
		}
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

func ValidateAgentCard(card AgentCard) error {
	var errs ValidationErrors

	if card.Name == "" {
		errs.Add("name is required")
	}
	if card.URL == "" {
		errs.Add("url is required")
	} else if _, err := url.Parse(card.URL); err != nil {
		errs.Add(fmt.Sprintf("invalid url: %v", err))
	}
	if card.Transport != nil {
		if err := ValidateTransportConfig(card.Transport); err != nil {
			errs.Add(err.Error())
		}
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

func ValidateTransportConfig(tc *TransportConfig) error {
	if tc == nil {
		return nil
	}

	if !validTransportTypes[tc.Type] {
		return fmt.Errorf("invalid transport type %q", tc.Type)
	}

	var errs ValidationErrors
	cfg := tc.Config
	if cfg == nil {
		cfg = map[string]string{}
	}

	switch tc.Type {
	case "ssh":
		if cfg["host"] == "" {
			errs.Add("ssh transport requires host")
		}
		if cfg["user"] == "" {
			errs.Add("ssh transport requires user")
		}
		if cfg["key_path"] == "" {
			errs.Add("ssh transport requires key_path")
		}
	case "docker":
		if cfg["container"] == "" {
			errs.Add("docker transport requires container")
		}
	case "stdio":
		if cfg["command"] == "" {
			errs.Add("stdio transport requires command")
		}
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

func ValidateJSONRPCRequest(req JSONRPCRequest) *JSONRPCError {
	if req.JSONRPC != "2.0" {
		return &JSONRPCError{Code: -32600, Message: "invalid request: jsonrpc must be \"2.0\""}
	}
	if req.Method == "" {
		return &JSONRPCError{Code: -32600, Message: "invalid request: method is required"}
	}
	if req.ID == nil {
		return &JSONRPCError{Code: -32600, Message: "invalid request: id is required"}
	}
	return nil
}

func ValidateSendTaskParams(p SendTaskParams) *JSONRPCError {
	if p.ID == "" {
		return &JSONRPCError{Code: -32602, Message: "invalid params: task id is required"}
	}
	if p.Message.Role != "user" {
		return &JSONRPCError{Code: -32602, Message: "invalid params: message role must be \"user\""}
	}
	if len(p.Message.Parts) == 0 {
		return &JSONRPCError{Code: -32602, Message: "invalid params: message must have at least one part"}
	}
	for i, part := range p.Message.Parts {
		if part.Type == "" {
			return &JSONRPCError{Code: -32602, Message: fmt.Sprintf("invalid params: part[%d] type is required", i)}
		}
	}
	return nil
}

func ValidateAgentConfig(cfg AgentConfig) error {
	var errs ValidationErrors

	if cfg.Name == "" {
		errs.Add("name is required")
	}
	if cfg.Port != 0 && (cfg.Port < 1 || cfg.Port > 65535) {
		errs.Add("port must be between 1 and 65535")
	}
	switch cfg.Handler {
	case "echo", "greeter":
		// valid
	case "exec":
		if cfg.Backend == nil {
			errs.Add("exec handler requires backend config")
		} else if cfg.Backend.Command == "" {
			errs.Add("exec handler requires backend command")
		}
	default:
		errs.Add(fmt.Sprintf("unknown handler %q", cfg.Handler))
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

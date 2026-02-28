// Package cliparse extracts structured results from CLI agent output.
//
// Coding agents (Claude, Codex, etc.) emit output in various formats.
// This package handles both plain JSON ({"result":"..."}) and Claude's
// verbose array format ([{type:"system",...}, ...]), filtering out
// system/init noise from verbose output.
package cliparse

import (
	"encoding/json"
	"strings"
)

// Output holds the parsed result from CLI output.
type Output struct {
	Result  string          // extracted result text
	Verbose json.RawMessage // filtered verbose events (nil if not verbose)
}

// Parse extracts text from CLI output, handling both plain
// JSON ({"result":"..."}) and verbose array format ([{type:"system",...}, ...]).
// For verbose output, system/init entries are filtered out and the remaining
// events (tool calls, usage, cost) are preserved in Verbose.
func Parse(stdout string) Output {
	stdout = strings.TrimSpace(stdout)

	// Try single-object format: {"result": "..."}
	var claudeResp struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout), &claudeResp); err == nil && claudeResp.Result != "" {
		return Output{Result: claudeResp.Result}
	}

	// Try verbose array format: [{type: "system", ...}, ...]
	var events []json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &events); err == nil && len(events) > 0 {
		var filtered []json.RawMessage
		var result string
		for _, ev := range events {
			var entry struct {
				Type   string `json:"type"`
				Result string `json:"result"`
			}
			if json.Unmarshal(ev, &entry) != nil {
				continue
			}
			if entry.Type == "system" || entry.Type == "rate_limit_event" {
				continue
			}
			filtered = append(filtered, ev)
			if entry.Type == "result" && entry.Result != "" {
				result = entry.Result
			}
		}
		if result != "" {
			verbose, _ := json.Marshal(filtered)
			return Output{Result: result, Verbose: verbose}
		}
	}

	return Output{Result: stdout}
}

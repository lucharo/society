// Package cliparse extracts structured results from CLI agent output.
//
// Coding agents (Claude, Codex, etc.) emit output in various formats.
// This package handles both plain JSON ({"result":"..."}) and Claude's
// verbose array format ([{type:"system",...}, ...]), filtering out
// system/init noise from verbose output.
package cliparse

import (
	"encoding/json"
	"fmt"
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

// TraceStyle controls trace formatting colors.
type TraceStyle struct {
	ToolCall string // prefix for tool call lines (e.g. ANSI cyan)
	Dim      string // dim/muted text
	Error    string // error prefix
	Reset    string // reset formatting
}

// PlainStyle returns a TraceStyle with no ANSI codes.
func PlainStyle() TraceStyle { return TraceStyle{} }

// FormatTrace renders verbose trace events as a readable conversation.
// The final assistant text is omitted since it duplicates the main response.
func FormatTrace(raw json.RawMessage, style TraceStyle) string {
	var events []json.RawMessage
	if json.Unmarshal(raw, &events) != nil || len(events) == 0 {
		return ""
	}

	type traceEntry struct {
		Type    string `json:"type"`
		Message struct {
			Content []struct {
				Type    string          `json:"type"`
				Text    string          `json:"text"`
				Name    string          `json:"name"`
				Input   json.RawMessage `json:"input"`
				Content string          `json:"content"`
				IsError bool            `json:"is_error"`
			} `json:"content"`
		} `json:"message"`
		Result     string  `json:"result"`
		NumTurns   int     `json:"num_turns"`
		DurationMS int     `json:"duration_ms"`
		CostUSD    float64 `json:"total_cost_usd"`
	}

	var entries []traceEntry
	var resultText string
	for _, ev := range events {
		var e traceEntry
		if json.Unmarshal(ev, &e) != nil {
			continue
		}
		entries = append(entries, e)
		if e.Type == "result" && e.Result != "" {
			resultText = e.Result
		}
	}

	var b strings.Builder
	for _, entry := range entries {
		switch entry.Type {
		case "assistant":
			for _, c := range entry.Message.Content {
				switch c.Type {
				case "text":
					if c.Text != "" && c.Text != resultText {
						fmt.Fprintf(&b, "%s\n", c.Text)
					}
				case "tool_use":
					inputStr := string(c.Input)
					if len(inputStr) > 200 {
						inputStr = inputStr[:200] + "…"
					}
					fmt.Fprintf(&b, "%s> %s%s %s%s%s\n", style.ToolCall, c.Name, style.Reset, style.Dim, inputStr, style.Reset)
				}
			}

		case "user":
			for _, c := range entry.Message.Content {
				if c.Type == "tool_result" {
					text := c.Content
					if len(text) > 300 {
						text = text[:300] + "…"
					}
					if c.IsError {
						fmt.Fprintf(&b, "%s  %s✗ %s%s%s\n", style.Dim, style.Error, style.Reset, text, style.Reset)
					} else {
						fmt.Fprintf(&b, "%s  %s%s\n", style.Dim, text, style.Reset)
					}
				}
			}

		case "result":
			if entry.DurationMS > 0 || entry.CostUSD > 0 {
				var parts []string
				if entry.NumTurns > 0 {
					parts = append(parts, fmt.Sprintf("%d turns", entry.NumTurns))
				}
				if entry.DurationMS > 0 {
					parts = append(parts, fmt.Sprintf("%.1fs", float64(entry.DurationMS)/1000))
				}
				if entry.CostUSD > 0 {
					parts = append(parts, fmt.Sprintf("$%.4f", entry.CostUSD))
				}
				fmt.Fprintf(&b, "\n%s%s%s\n", style.Dim, strings.Join(parts, " · "), style.Reset)
			}
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

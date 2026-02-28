package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/luischavesdev/society/internal/client"
	"github.com/luischavesdev/society/internal/registry"
)

func Send(registryPath, name, message string, out io.Writer, showTrace bool, threadID ...string) error {
	reg, err := registry.Load(registryPath)
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}

	if _, err := reg.Get(name); err != nil {
		return fmt.Errorf("agent %q not found — run 'society list' to see registered agents or 'society onboard' to add one", name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	c := client.New(reg)
	task, err := c.Send(ctx, name, message, threadID...)
	if err != nil {
		return err
	}

	// Print response
	for _, a := range task.Artifacts {
		for _, p := range a.Parts {
			if p.Type == "text" {
				fmt.Fprintln(out, strings.TrimSpace(p.Text))
			}
		}
	}

	// Print trace data if requested
	if showTrace {
		for _, a := range task.Artifacts {
			if a.Name == "trace" {
				for _, p := range a.Parts {
					if p.Type == "data" && p.Data != nil {
						raw, _ := json.Marshal(p.Data)
						if s := formatTrace(raw); s != "" {
							fmt.Fprintf(out, "\n%s--- trace ---%s\n%s\n", dim, reset, s)
						}
					}
				}
			}
		}
	}

	// Print thread/status info in dim
	if task.Status.State == "failed" {
		fmt.Fprintf(out, "\n%s✗ %s%s\n", red, task.Status.Message, reset)
	}
	if task.ID != "" {
		fmt.Fprintf(out, "\n%sthread: %s%s\n", dim, task.ID, reset)
	}

	return nil
}

// formatTrace renders verbose trace events as a readable conversation.
// The final assistant text is omitted since it duplicates the main response.
func formatTrace(raw json.RawMessage) string {
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

	// Parse all entries, find the result text to skip duplicate final assistant text.
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
					fmt.Fprintf(&b, "%s> %s%s %s\n", cyan, c.Name, reset, dim+inputStr+reset)
				}
			}

		case "user":
			for _, c := range entry.Message.Content {
				if c.Type == "tool_result" {
					prefix := "  "
					if c.IsError {
						prefix = "  " + red + "✗ " + reset
					}
					text := c.Content
					if len(text) > 300 {
						text = text[:300] + "…"
					}
					fmt.Fprintf(&b, "%s%s%s\n", dim, prefix+text, reset)
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
				fmt.Fprintf(&b, "\n%s%s%s\n", dim, strings.Join(parts, " · "), reset)
			}
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

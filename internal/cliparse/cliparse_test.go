package cliparse

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		stdout      string
		wantResult  string
		wantVerbose bool
	}{
		{"claude json", `{"result": "Hello!"}`, "Hello!", false},
		{"plain text", "Just text", "Just text", false},
		{"empty result", `{"result": ""}`, `{"result": ""}`, false},
		{"whitespace", "  trimmed  ", "trimmed", false},
		{
			"verbose array",
			`[{"type":"system","subtype":"init","tools":["Bash","Read"]},{"type":"assistant","message":{"content":[{"type":"text","text":"pong"}]}},{"type":"result","result":"pong","cost_usd":0.01}]`,
			"pong",
			true,
		},
		{
			"verbose filters init",
			`[{"type":"system","subtype":"init","tools":["a","b","c"]},{"type":"result","result":"done","duration_ms":500}]`,
			"done",
			true,
		},
		{
			"verbose filters rate_limit_event",
			`[{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}},{"type":"rate_limit_event","rate_limit_info":{}},{"type":"result","result":"hi","total_cost_usd":0.01}]`,
			"hi",
			true,
		},
		{
			"verbose array no result entry",
			`[{"type":"system","subtype":"init","tools":["a"]},{"type":"assistant","message":"hi"}]`,
			`[{"type":"system","subtype":"init","tools":["a"]},{"type":"assistant","message":"hi"}]`,
			false,
		},
		{
			"empty array",
			`[]`,
			`[]`,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.stdout)
			if got.Result != tt.wantResult {
				t.Errorf("Parse(%q).Result = %q, want %q", tt.stdout, got.Result, tt.wantResult)
			}
			if tt.wantVerbose && got.Verbose == nil {
				t.Errorf("Parse(%q).Verbose = nil, want non-nil", tt.stdout)
			}
			if !tt.wantVerbose && got.Verbose != nil {
				t.Errorf("Parse(%q).Verbose = %s, want nil", tt.stdout, got.Verbose)
			}
			// Verify filtered entries are removed from verbose output
			if got.Verbose != nil {
				s := string(got.Verbose)
				if strings.Contains(s, `"type":"system"`) {
					t.Errorf("verbose output should not contain system entries, got %s", s)
				}
				if strings.Contains(s, `"type":"rate_limit_event"`) {
					t.Errorf("verbose output should not contain rate_limit_event entries, got %s", s)
				}
			}
		})
	}
}

func TestFormatTrace(t *testing.T) {
	plain := PlainStyle()

	tests := []struct {
		name     string
		input    string
		style    TraceStyle
		contains []string
		excludes []string
	}{
		{
			"tool call and result",
			`[{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"ls"}}]}},{"type":"user","message":{"content":[{"type":"tool_result","content":"file.txt"}]}},{"type":"result","result":"done","num_turns":1,"duration_ms":2500,"total_cost_usd":0.003}]`,
			plain,
			[]string{"> Bash", "file.txt", "1 turns", "2.5s", "$0.0030"},
			nil,
		},
		{
			"skips final assistant text matching result",
			`[{"type":"assistant","message":{"content":[{"type":"text","text":"Hello world"}]}},{"type":"result","result":"Hello world"}]`,
			plain,
			nil,
			[]string{"Hello world"},
		},
		{
			"includes non-final assistant text",
			`[{"type":"assistant","message":{"content":[{"type":"text","text":"thinking..."}]}},{"type":"assistant","message":{"content":[{"type":"text","text":"done"}]}},{"type":"result","result":"done"}]`,
			plain,
			[]string{"thinking..."},
			[]string{},
		},
		{
			"error tool result",
			`[{"type":"user","message":{"content":[{"type":"tool_result","content":"not found","is_error":true}]}}]`,
			plain,
			[]string{"not found"},
			nil,
		},
		{
			"truncates long input",
			`[{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"path":"` + strings.Repeat("a", 250) + `"}}]}}]`,
			plain,
			[]string{"> Read", "…"},
			nil,
		},
		{
			"empty input returns empty",
			`[]`,
			plain,
			nil,
			nil,
		},
		{
			"invalid JSON returns empty",
			`not json`,
			plain,
			nil,
			nil,
		},
		{
			"ANSI style applies prefixes",
			`[{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{}}]}}]`,
			TraceStyle{ToolCall: "\033[36m", Dim: "\033[2m", Reset: "\033[0m"},
			[]string{"\033[36m", "\033[0m"},
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTrace(json.RawMessage(tt.input), tt.style)
			for _, s := range tt.contains {
				if !strings.Contains(got, s) {
					t.Errorf("FormatTrace() should contain %q, got:\n%s", s, got)
				}
			}
			for _, s := range tt.excludes {
				if strings.Contains(got, s) {
					t.Errorf("FormatTrace() should not contain %q, got:\n%s", s, got)
				}
			}
		})
	}
}

func TestFormatTrace_EmptyForNil(t *testing.T) {
	if got := FormatTrace(nil, PlainStyle()); got != "" {
		t.Errorf("FormatTrace(nil) = %q, want empty", got)
	}
}

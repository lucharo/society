package cliparse

import (
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
			// Verify system/init entries are filtered from verbose output
			if got.Verbose != nil {
				s := string(got.Verbose)
				if strings.Contains(s, `"type":"system"`) {
					t.Errorf("verbose output should not contain system entries, got %s", s)
				}
			}
		})
	}
}

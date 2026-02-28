package main

import (
	"testing"
)

func TestParseSendArgs(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantPos    []string
		wantTrace  bool
		wantThread string
	}{
		{
			"flags before positional",
			[]string{"--trace", "--thread", "abc", "echo", "hello"},
			[]string{"echo", "hello"},
			true, "abc",
		},
		{
			"flags after positional",
			[]string{"echo", "hello", "world", "--trace", "--thread", "abc"},
			[]string{"echo", "hello", "world"},
			true, "abc",
		},
		{
			"flags interleaved",
			[]string{"echo", "--trace", "hello", "--thread", "abc"},
			[]string{"echo", "hello"},
			true, "abc",
		},
		{
			"no flags",
			[]string{"echo", "hello"},
			[]string{"echo", "hello"},
			false, "",
		},
		{
			"trace only",
			[]string{"echo", "msg", "--trace"},
			[]string{"echo", "msg"},
			true, "",
		},
		{
			"thread only",
			[]string{"echo", "msg", "--thread", "t1"},
			[]string{"echo", "msg"},
			false, "t1",
		},
		{
			"thread at end without value",
			[]string{"echo", "msg", "--thread"},
			[]string{"echo", "msg"},
			false, "",
		},
		{
			"thread followed by another flag",
			[]string{"echo", "msg", "--thread", "--trace"},
			[]string{"echo", "msg"},
			true, "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSendArgs(tt.args)
			if len(got.positional) != len(tt.wantPos) {
				t.Fatalf("positional = %v, want %v", got.positional, tt.wantPos)
			}
			for i := range tt.wantPos {
				if got.positional[i] != tt.wantPos[i] {
					t.Errorf("positional[%d] = %q, want %q", i, got.positional[i], tt.wantPos[i])
				}
			}
			if got.trace != tt.wantTrace {
				t.Errorf("trace = %v, want %v", got.trace, tt.wantTrace)
			}
			if got.thread != tt.wantThread {
				t.Errorf("thread = %q, want %q", got.thread, tt.wantThread)
			}
		})
	}
}

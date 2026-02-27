package thread

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStore_SaveAndLoad(t *testing.T) {
	s := NewStore(t.TempDir())

	th := &Thread{
		ID:        "test-1",
		Agent:     "claude",
		SessionID: "sess-abc",
		Messages: []Entry{
			{Role: "user", Text: "hello"},
			{Role: "assistant", Text: "hi there"},
		},
		CreatedAt: time.Now(),
	}

	if err := s.Save(th); err != nil {
		t.Fatal(err)
	}

	loaded, err := s.Load("test-1")
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil {
		t.Fatal("expected thread, got nil")
	}
	if loaded.Agent != "claude" {
		t.Errorf("got agent %q, want claude", loaded.Agent)
	}
	if loaded.SessionID != "sess-abc" {
		t.Errorf("got session_id %q, want sess-abc", loaded.SessionID)
	}
	if len(loaded.Messages) != 2 {
		t.Errorf("got %d messages, want 2", len(loaded.Messages))
	}
	if loaded.UpdatedAt.IsZero() {
		t.Error("updated_at should be set")
	}
}

func TestStore_LoadMissing(t *testing.T) {
	s := NewStore(t.TempDir())

	th, err := s.Load("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if th != nil {
		t.Error("expected nil for missing thread")
	}
}

func TestStore_List(t *testing.T) {
	s := NewStore(t.TempDir())

	for _, th := range []*Thread{
		{ID: "t1", Agent: "claude", CreatedAt: time.Now()},
		{ID: "t2", Agent: "claude", CreatedAt: time.Now()},
		{ID: "t3", Agent: "codex", CreatedAt: time.Now()},
	} {
		if err := s.Save(th); err != nil {
			t.Fatal(err)
		}
	}

	all, err := s.List("")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Errorf("got %d threads, want 3", len(all))
	}

	claude, err := s.List("claude")
	if err != nil {
		t.Fatal(err)
	}
	if len(claude) != 2 {
		t.Errorf("got %d claude threads, want 2", len(claude))
	}
}

func TestStore_ListEmpty(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "nonexistent"))

	threads, err := s.List("")
	if err != nil {
		t.Fatal(err)
	}
	if threads != nil {
		t.Error("expected nil for empty list")
	}
}

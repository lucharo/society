package agent

import (
	"context"
	"testing"

	"github.com/luischavesdev/society/internal/models"
)

func TestEchoHandler(t *testing.T) {
	h := &EchoHandler{}

	t.Run("echoes input", func(t *testing.T) {
		params := &models.SendTaskParams{
			ID: "1",
			Message: models.Message{
				Role:  "user",
				Parts: []models.Part{{Type: "text", Text: "hello"}},
			},
		}

		task, err := h.Handle(context.Background(), params)
		if err != nil {
			t.Fatal(err)
		}
		if task.Status.State != models.TaskStateCompleted {
			t.Errorf("got state %s, want completed", task.Status.State)
		}
		if len(task.Artifacts) != 1 || len(task.Artifacts[0].Parts) != 1 {
			t.Fatal("expected 1 artifact with 1 part")
		}
		if task.Artifacts[0].Parts[0].Text != "hello" {
			t.Errorf("got %q, want hello", task.Artifacts[0].Parts[0].Text)
		}
	})

	t.Run("multiple parts", func(t *testing.T) {
		params := &models.SendTaskParams{
			ID: "2",
			Message: models.Message{
				Role: "user",
				Parts: []models.Part{
					{Type: "text", Text: "a"},
					{Type: "text", Text: "b"},
				},
			},
		}

		task, err := h.Handle(context.Background(), params)
		if err != nil {
			t.Fatal(err)
		}
		if len(task.Artifacts[0].Parts) != 2 {
			t.Errorf("expected 2 parts, got %d", len(task.Artifacts[0].Parts))
		}
	})
}

func TestGreeterHandler(t *testing.T) {
	h := &GreeterHandler{}

	t.Run("prepends greeting", func(t *testing.T) {
		params := &models.SendTaskParams{
			ID: "1",
			Message: models.Message{
				Role:  "user",
				Parts: []models.Part{{Type: "text", Text: "world"}},
			},
		}

		task, err := h.Handle(context.Background(), params)
		if err != nil {
			t.Fatal(err)
		}
		got := task.Artifacts[0].Parts[0].Text
		want := "Hello! You said: world"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("passes non-text parts through", func(t *testing.T) {
		params := &models.SendTaskParams{
			ID: "2",
			Message: models.Message{
				Role: "user",
				Parts: []models.Part{
					{Type: "data", Data: "raw"},
					{Type: "text", Text: "hi"},
				},
			},
		}

		task, err := h.Handle(context.Background(), params)
		if err != nil {
			t.Fatal(err)
		}
		parts := task.Artifacts[0].Parts
		if parts[0].Type != "data" {
			t.Errorf("first part should be data, got %s", parts[0].Type)
		}
		if parts[1].Text != "Hello! You said: hi" {
			t.Errorf("second part should be greeted, got %q", parts[1].Text)
		}
	})
}

func TestNewHandler(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"echo", false},
		{"greeter", false},
		{"unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewHandler(tt.name)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatal(err)
			}
		})
	}
}

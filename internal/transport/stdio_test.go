package transport

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var echoBinary string

func TestMain(m *testing.M) {
	// Build the echo test binary
	dir, err := os.MkdirTemp("", "society-stdio-test")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	echoBinary = filepath.Join(dir, "echo")
	cmd := exec.Command("go", "build", "-o", echoBinary, "./testdata/echo.go")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("failed to build echo binary: " + err.Error())
	}

	os.Exit(m.Run())
}

func TestSTDIOTransport_Send(t *testing.T) {
	t.Run("echo round trip", func(t *testing.T) {
		cfg := STDIOConfig{Command: echoBinary}
		tr, err := NewSTDIO(cfg)
		if err != nil {
			t.Fatal(err)
		}

		ctx := context.Background()
		if err := tr.Open(ctx); err != nil {
			t.Fatal(err)
		}
		defer tr.Close()

		resp, err := tr.Send(ctx, []byte(`{"jsonrpc":"2.0","id":"42","method":"test","params":{"hello":"world"}}`))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(resp), `"hello":"world"`) {
			t.Errorf("unexpected response: %s", resp)
		}
	})

	t.Run("command not found", func(t *testing.T) {
		_, err := NewSTDIO(STDIOConfig{Command: "nonexistent-binary-xyz"})
		if err == nil {
			t.Fatal("expected error for missing command")
		}
	})
}

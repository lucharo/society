package transport

import (
	"fmt"
	"strings"
	"testing"

	"github.com/luischavesdev/society/internal/models"
)

func TestFactory(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		tc        *models.TransportConfig
		wantType  string
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "nil config returns HTTP",
			url:      "http://localhost:8080",
			tc:       nil,
			wantType: "HTTPTransport",
		},
		{
			name:     "empty type returns HTTP",
			url:      "http://localhost:8080",
			tc:       &models.TransportConfig{Type: ""},
			wantType: "HTTPTransport",
		},
		{
			name:     "http type returns HTTP",
			url:      "http://localhost:8080",
			tc:       &models.TransportConfig{Type: "http"},
			wantType: "HTTPTransport",
		},
		{
			name: "ssh type returns SSH",
			url:  "http://localhost:8080",
			tc: &models.TransportConfig{Type: "ssh", Config: map[string]string{
				"host": "h", "user": "u", "key_path": "/k",
			}},
			wantType: "SSHTransport",
		},
		{
			name: "docker type returns Docker",
			url:  "http://localhost:8080",
			tc: &models.TransportConfig{Type: "docker", Config: map[string]string{
				"container": "c",
			}},
			wantType: "DockerTransport",
		},
		{
			name: "stdio type returns STDIO",
			url:  "http://localhost:8080",
			tc: &models.TransportConfig{Type: "stdio", Config: map[string]string{
				"command": echoBinary,
			}},
			wantType: "STDIOTransport",
		},
		{
			name:      "unknown type returns error",
			url:       "http://localhost:8080",
			tc:        &models.TransportConfig{Type: "pigeons"},
			wantErr:   true,
			errSubstr: "unknown transport type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr, err := New(tt.url, tt.tc)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q should contain %q", err, tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tr == nil {
				t.Fatal("expected non-nil transport")
			}

			got := typeName(tr)
			if got != tt.wantType {
				t.Errorf("got type %s, want %s", got, tt.wantType)
			}
		})
	}
}

func typeName(v any) string {
	switch v.(type) {
	case *HTTPTransport:
		return "HTTPTransport"
	case *SSHTransport:
		return "SSHTransport"
	case *DockerTransport:
		return "DockerTransport"
	case *STDIOTransport:
		return "STDIOTransport"
	default:
		return fmt.Sprintf("unknown(%T)", v)
	}
}

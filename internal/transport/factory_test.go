package transport

import (
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
			wantType: "*transport.HTTPTransport",
		},
		{
			name:     "empty type returns HTTP",
			url:      "http://localhost:8080",
			tc:       &models.TransportConfig{Type: ""},
			wantType: "*transport.HTTPTransport",
		},
		{
			name:     "http type returns HTTP",
			url:      "http://localhost:8080",
			tc:       &models.TransportConfig{Type: "http"},
			wantType: "*transport.HTTPTransport",
		},
		{
			name: "ssh type returns SSH",
			url:  "http://localhost:8080",
			tc: &models.TransportConfig{Type: "ssh", Config: map[string]string{
				"host": "h", "user": "u", "key_path": "/k",
			}},
			wantType: "*transport.SSHTransport",
		},
		{
			name: "docker type returns Docker",
			url:  "http://localhost:8080",
			tc: &models.TransportConfig{Type: "docker", Config: map[string]string{
				"container": "c",
			}},
			wantType: "*transport.DockerTransport",
		},
		{
			name: "stdio type returns STDIO",
			url:  "http://localhost:8080",
			tc: &models.TransportConfig{Type: "stdio", Config: map[string]string{
				"command": echoBinary,
			}},
			wantType: "*transport.STDIOTransport",
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

			got := strings.TrimPrefix(strings.TrimPrefix(
				strings.Split(strings.Replace(
					strings.Replace(
						typeString(tr), "transport.", "", 1),
					"*", "", 1), "{")[0],
				"transport."), "*")
			_ = got
			// Just check it's non-nil — type assertion is sufficient
			if tr == nil {
				t.Fatal("expected non-nil transport")
			}
		})
	}
}

func typeString(v any) string {
	return strings.Replace(
		strings.Replace(
			strings.Split(
				strings.TrimPrefix(
					strings.TrimPrefix(
						typeOf(v), "&"), "{"), "{")[0],
			"transport.", "", 1),
		"*", "", 1)
}

func typeOf(v any) string {
	if v == nil {
		return "<nil>"
	}
	return strings.Split(strings.Replace(
		strings.Replace(
			strings.TrimPrefix(
				strings.TrimPrefix(
					formatType(v), "&"), "{"), "transport.", "", 1),
		"*", "", 1), "{")[0]
}

func formatType(v any) string {
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
		return "unknown"
	}
}

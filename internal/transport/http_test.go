package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPTransport_Send(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"jsonrpc":"2.0","id":"1","result":"ok"}`))
		}))
		defer ts.Close()

		tr, err := NewHTTP(ts.URL)
		if err != nil {
			t.Fatal(err)
		}

		resp, err := tr.Send(context.Background(), []byte(`{"jsonrpc":"2.0","id":"1","method":"test"}`))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(resp), `"result":"ok"`) {
			t.Errorf("unexpected response: %s", resp)
		}
	})

	t.Run("server error 500", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "internal error", http.StatusInternalServerError)
		}))
		defer ts.Close()

		tr, _ := NewHTTP(ts.URL)
		_, err := tr.Send(context.Background(), []byte(`{}`))
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "500") {
			t.Errorf("error should mention status code: %v", err)
		}
	})

	t.Run("unreachable server", func(t *testing.T) {
		tr, _ := NewHTTP("http://127.0.0.1:1")
		_, err := tr.Send(context.Background(), []byte(`{}`))
		if err == nil {
			t.Fatal("expected error for unreachable server")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Second)
		}))
		defer ts.Close()

		tr, _ := NewHTTP(ts.URL, WithTimeout(50*time.Millisecond))
		_, err := tr.Send(context.Background(), []byte(`{}`))
		if err == nil {
			t.Fatal("expected timeout error")
		}
	})

	t.Run("invalid URL in constructor", func(t *testing.T) {
		_, err := NewHTTP("")
		if err == nil {
			t.Fatal("expected error for empty URL")
		}
	})
}

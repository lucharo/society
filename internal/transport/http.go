package transport

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type HTTPOption func(*HTTPTransport)

type HTTPTransport struct {
	url    string
	client *http.Client
}

func WithHTTPClient(c *http.Client) HTTPOption {
	return func(t *HTTPTransport) { t.client = c }
}

func WithTimeout(d time.Duration) HTTPOption {
	return func(t *HTTPTransport) { t.client.Timeout = d }
}

func NewHTTP(rawURL string, opts ...HTTPOption) (*HTTPTransport, error) {
	if _, err := url.Parse(rawURL); err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	if rawURL == "" {
		return nil, fmt.Errorf("invalid url: empty")
	}
	t := &HTTPTransport{
		url:    rawURL,
		client: &http.Client{Timeout: 30 * time.Second},
	}
	for _, o := range opts {
		o(t)
	}
	return t, nil
}

func (t *HTTPTransport) Open(ctx context.Context) error { return nil }

func (t *HTTPTransport) Send(ctx context.Context, payload []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func (t *HTTPTransport) Close() error { return nil }

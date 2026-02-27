package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

func TestProbeA2AEndpoint_SpecPath(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/agent-card.json" {
			json.NewEncoder(w).Encode(map[string]string{
				"name":        "test-agent",
				"description": "A test agent",
			})
		} else {
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	// Extract host and port from test server
	host, port := tsHostPort(t, ts)
	client := ts.Client()

	name, desc, ok := probeA2AEndpoint(client, host, port)
	if !ok {
		t.Fatal("expected probe to succeed")
	}
	if name != "test-agent" {
		t.Errorf("name = %q, want %q", name, "test-agent")
	}
	if desc != "A test agent" {
		t.Errorf("description = %q, want %q", desc, "A test agent")
	}
}

func TestProbeA2AEndpoint_LegacyPath(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/agent.json" {
			json.NewEncoder(w).Encode(map[string]string{
				"name": "legacy-agent",
			})
		} else {
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	host, port := tsHostPort(t, ts)
	name, _, ok := probeA2AEndpoint(ts.Client(), host, port)
	if !ok {
		t.Fatal("expected probe to succeed via legacy path")
	}
	if name != "legacy-agent" {
		t.Errorf("name = %q, want %q", name, "legacy-agent")
	}
}

func TestProbeA2AEndpoint_NoAgent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ts.Close()

	host, port := tsHostPort(t, ts)
	_, _, ok := probeA2AEndpoint(ts.Client(), host, port)
	if ok {
		t.Fatal("expected probe to fail for 404")
	}
}

func TestProbeA2AEndpoint_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer ts.Close()

	host, port := tsHostPort(t, ts)
	_, _, ok := probeA2AEndpoint(ts.Client(), host, port)
	if ok {
		t.Fatal("expected probe to fail for invalid JSON")
	}
}

func TestDedup_DeepReplacesShallow(t *testing.T) {
	shallow := []Candidate{
		{Name: "myapp", Transport: "docker", Config: map[string]string{"container": "myapp"}},
		{Name: "web", Transport: "docker", Config: map[string]string{"container": "web"}},
		{Name: "server1", Transport: "ssh", Config: map[string]string{"host": "10.0.0.1"}},
	}

	deep := []Candidate{
		{Name: "real-agent", Transport: "docker", Verified: true, Config: map[string]string{"container": "myapp"}},
		{Name: "ssh-agent", Transport: "ssh", Verified: true, Config: map[string]string{"host": "10.0.0.1"}},
	}

	result := dedup(shallow, deep)

	// Should have: web (shallow, not replaced) + real-agent + ssh-agent (deep)
	if len(result) != 3 {
		t.Fatalf("len(result) = %d, want 3; got %v", len(result), candidateNames(result))
	}

	names := make(map[string]bool)
	for _, c := range result {
		names[c.Name] = true
	}

	if names["myapp"] {
		t.Error("shallow 'myapp' should have been replaced by deep 'real-agent'")
	}
	if !names["web"] {
		t.Error("shallow 'web' should be kept (no deep replacement)")
	}
	if !names["real-agent"] {
		t.Error("deep 'real-agent' should be present")
	}
	if names["server1"] {
		t.Error("shallow 'server1' should have been replaced by deep 'ssh-agent'")
	}
	if !names["ssh-agent"] {
		t.Error("deep 'ssh-agent' should be present")
	}
}

func TestDedup_NoDeep(t *testing.T) {
	shallow := []Candidate{
		{Name: "a", Transport: "docker", Config: map[string]string{"container": "a"}},
	}

	result := dedup(shallow, nil)
	if len(result) != 1 || result[0].Name != "a" {
		t.Errorf("dedup with no deep should return shallow unchanged")
	}
}

func TestDedup_DifferentTransport(t *testing.T) {
	shallow := []Candidate{
		{Name: "myapp", Transport: "stdio", Config: map[string]string{"command": "/usr/bin/myapp"}},
	}

	deep := []Candidate{
		{Name: "myapp", Transport: "docker", Verified: true, Config: map[string]string{"container": "myapp"}},
	}

	result := dedup(shallow, deep)
	// Different transport — both should be kept
	if len(result) != 2 {
		t.Fatalf("len(result) = %d, want 2", len(result))
	}
}

// tsHostPort extracts host and port from a httptest.Server URL.
func tsHostPort(t *testing.T, ts *httptest.Server) (string, int) {
	t.Helper()
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("parsing test server URL %q: %v", ts.URL, err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parsing port from %q: %v", ts.URL, err)
	}
	return u.Hostname(), port
}

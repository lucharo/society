package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/luischavesdev/society/internal/models"
	"github.com/luischavesdev/society/internal/registry"
)

func TestDiscover_Valid(t *testing.T) {
	card := models.AgentCard{
		Name:        "remote-agent",
		Description: "A remote agent",
		URL:         "http://example.com/a2a",
		Skills:      []models.Skill{{ID: "qa", Name: "QA"}},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/agent.json" {
			json.NewEncoder(w).Encode(card)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	regPath := strings.Replace(t.TempDir()+"/reg.json", "//", "/", -1)
	// User says yes, then picks http transport
	input := "y\nhttp\n"
	out := &bytes.Buffer{}

	err := Discover(regPath, ts.URL, strings.NewReader(input), out)
	if err != nil {
		t.Fatal(err)
	}

	output := out.String()
	if !strings.Contains(output, "remote-agent") {
		t.Errorf("output should contain agent name: %s", output)
	}
	if !strings.Contains(output, "Added") {
		t.Errorf("output should confirm add: %s", output)
	}

	reg, _ := registry.Load(regPath)
	if !reg.Has("remote-agent") {
		t.Error("agent should be in registry")
	}
}

func TestDiscover_DirectJSON(t *testing.T) {
	card := models.AgentCard{
		Name: "direct",
		URL:  "http://example.com",
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(card)
	}))
	defer ts.Close()

	regPath := t.TempDir() + "/reg.json"
	input := "y\nhttp\n"
	out := &bytes.Buffer{}

	// Use .json URL to hit direct path
	err := Discover(regPath, ts.URL+"/card.json", strings.NewReader(input), out)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDiscover_404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ts.Close()

	regPath := t.TempDir() + "/reg.json"
	err := Discover(regPath, ts.URL, strings.NewReader(""), &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestDiscover_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json`))
	}))
	defer ts.Close()

	regPath := t.TempDir() + "/reg.json"
	err := Discover(regPath, ts.URL, strings.NewReader(""), &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDiscover_UserDeclines(t *testing.T) {
	card := models.AgentCard{Name: "declined", URL: "http://example.com"}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(card)
	}))
	defer ts.Close()

	regPath := t.TempDir() + "/reg.json"
	input := "n\n"
	out := &bytes.Buffer{}

	err := Discover(regPath, ts.URL+"/card.json", strings.NewReader(input), out)
	if err != nil {
		t.Fatal(err)
	}

	reg, _ := registry.Load(regPath)
	if reg.Has("declined") {
		t.Error("agent should not be in registry")
	}
}

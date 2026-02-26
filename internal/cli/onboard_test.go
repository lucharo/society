package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luischavesdev/society/internal/registry"
)

func TestOnboard_HTTP(t *testing.T) {
	regPath := filepath.Join(t.TempDir(), "reg.json")
	input := "myagent\nA test agent\nhttp\nhttp://localhost:9000\n\n"
	out := &bytes.Buffer{}

	err := Onboard(regPath, strings.NewReader(input), out)
	if err != nil {
		t.Fatal(err)
	}

	reg, _ := registry.Load(regPath)
	card, err := reg.Get("myagent")
	if err != nil {
		t.Fatal(err)
	}
	if card.URL != "http://localhost:9000" {
		t.Errorf("got URL %q", card.URL)
	}
}

func TestOnboard_SSH(t *testing.T) {
	regPath := filepath.Join(t.TempDir(), "reg.json")
	input := "sshagent\n\nssh\ndev-server\nclaude\n22\n~/.ssh/id_rsa\n8002\n\n"
	out := &bytes.Buffer{}

	err := Onboard(regPath, strings.NewReader(input), out)
	if err != nil {
		t.Fatal(err)
	}

	reg, _ := registry.Load(regPath)
	card, _ := reg.Get("sshagent")
	if card.Transport == nil || card.Transport.Type != "ssh" {
		t.Fatal("expected SSH transport")
	}
	if card.Transport.Config["host"] != "dev-server" {
		t.Errorf("got host %q", card.Transport.Config["host"])
	}
}

func TestOnboard_Docker(t *testing.T) {
	regPath := filepath.Join(t.TempDir(), "reg.json")
	input := "dockeragent\n\ndocker\nmy-container\nbridge\n8080\n/var/run/docker.sock\n\n"
	out := &bytes.Buffer{}

	err := Onboard(regPath, strings.NewReader(input), out)
	if err != nil {
		t.Fatal(err)
	}

	reg, _ := registry.Load(regPath)
	card, _ := reg.Get("dockeragent")
	if card.Transport == nil || card.Transport.Type != "docker" {
		t.Fatal("expected Docker transport")
	}
}

func TestOnboard_STDIO(t *testing.T) {
	regPath := filepath.Join(t.TempDir(), "reg.json")
	input := "stdioagent\n\nstdio\necho\n\n\n"
	out := &bytes.Buffer{}

	err := Onboard(regPath, strings.NewReader(input), out)
	if err != nil {
		t.Fatal(err)
	}

	reg, _ := registry.Load(regPath)
	card, _ := reg.Get("stdioagent")
	if card.Transport == nil || card.Transport.Type != "stdio" {
		t.Fatal("expected STDIO transport")
	}
}

func TestOnboard_EmptyName(t *testing.T) {
	regPath := filepath.Join(t.TempDir(), "reg.json")
	input := "\n"
	out := &bytes.Buffer{}

	err := Onboard(regPath, strings.NewReader(input), out)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestOnboard_Duplicate(t *testing.T) {
	regPath := filepath.Join(t.TempDir(), "reg.json")

	// First onboard
	input := "dup\n\nhttp\nhttp://localhost:9000\n\n"
	Onboard(regPath, strings.NewReader(input), &bytes.Buffer{})

	// Second onboard with same name
	input = "dup\n\nhttp\nhttp://localhost:9001\n\n"
	err := Onboard(regPath, strings.NewReader(input), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected duplicate error, got: %v", err)
	}
}

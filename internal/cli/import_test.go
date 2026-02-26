package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luischavesdev/society/internal/models"
)

func writeRegFile(t *testing.T, path string, agents []models.AgentCard) {
	t.Helper()
	data, _ := json.MarshalIndent(models.RegistryFile{Agents: agents}, "", "  ")
	os.WriteFile(path, data, 0644)
}

func TestImport_FromFile(t *testing.T) {
	regPath := filepath.Join(t.TempDir(), "reg.json")
	importPath := filepath.Join(t.TempDir(), "import.json")

	writeRegFile(t, importPath, []models.AgentCard{
		{Name: "a", URL: "http://a"},
		{Name: "b", URL: "http://b"},
	})

	out := &bytes.Buffer{}
	err := Import(regPath, importPath, strings.NewReader(""), out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "imported 2") {
		t.Errorf("output: %s", out.String())
	}
}

func TestImport_FromURL(t *testing.T) {
	rf := models.RegistryFile{Agents: []models.AgentCard{
		{Name: "remote", URL: "http://remote"},
	}}
	data, _ := json.Marshal(rf)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	}))
	defer ts.Close()

	regPath := filepath.Join(t.TempDir(), "reg.json")
	out := &bytes.Buffer{}
	err := Import(regPath, ts.URL, strings.NewReader(""), out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "imported 1") {
		t.Errorf("output: %s", out.String())
	}
}

func TestImport_ConflictOverwrite(t *testing.T) {
	regPath := filepath.Join(t.TempDir(), "reg.json")
	writeRegFile(t, regPath, []models.AgentCard{{Name: "a", URL: "http://old"}})

	importPath := filepath.Join(t.TempDir(), "import.json")
	writeRegFile(t, importPath, []models.AgentCard{{Name: "a", URL: "http://new"}})

	out := &bytes.Buffer{}
	err := Import(regPath, importPath, strings.NewReader("y\n"), out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "replaced 1") {
		t.Errorf("output: %s", out.String())
	}
}

func TestImport_ConflictSkip(t *testing.T) {
	regPath := filepath.Join(t.TempDir(), "reg.json")
	writeRegFile(t, regPath, []models.AgentCard{{Name: "a", URL: "http://old"}})

	importPath := filepath.Join(t.TempDir(), "import.json")
	writeRegFile(t, importPath, []models.AgentCard{{Name: "a", URL: "http://new"}})

	out := &bytes.Buffer{}
	err := Import(regPath, importPath, strings.NewReader("n\n"), out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "skipped 1") {
		t.Errorf("output: %s", out.String())
	}
}

func TestImport_ConflictRename(t *testing.T) {
	regPath := filepath.Join(t.TempDir(), "reg.json")
	writeRegFile(t, regPath, []models.AgentCard{{Name: "a", URL: "http://old"}})

	importPath := filepath.Join(t.TempDir(), "import.json")
	writeRegFile(t, importPath, []models.AgentCard{{Name: "a", URL: "http://new"}})

	out := &bytes.Buffer{}
	err := Import(regPath, importPath, strings.NewReader("rename\na2\n"), out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "renamed 1") {
		t.Errorf("output: %s", out.String())
	}
}

func TestImport_InvalidData(t *testing.T) {
	regPath := filepath.Join(t.TempDir(), "reg.json")
	importPath := filepath.Join(t.TempDir(), "import.json")
	// Agent missing name
	writeRegFile(t, importPath, []models.AgentCard{{URL: "http://a"}})

	out := &bytes.Buffer{}
	err := Import(regPath, importPath, strings.NewReader(""), out)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestImport_NonexistentFile(t *testing.T) {
	regPath := filepath.Join(t.TempDir(), "reg.json")
	err := Import(regPath, "/nonexistent/file.json", strings.NewReader(""), &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestImport_URL404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", 404)
	}))
	defer ts.Close()

	regPath := filepath.Join(t.TempDir(), "reg.json")
	err := Import(regPath, ts.URL, strings.NewReader(""), &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

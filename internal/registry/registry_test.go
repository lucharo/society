package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luischavesdev/society/internal/models"
)

func makeCard(name, url string) models.AgentCard {
	return models.AgentCard{Name: name, URL: url}
}

func writeRegistry(t *testing.T, path string, agents []models.AgentCard) {
	t.Helper()
	rf := models.RegistryFile{Agents: agents}
	data, _ := json.MarshalIndent(rf, "", "  ")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestLoad(t *testing.T) {
	t.Run("valid file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "reg.json")
		writeRegistry(t, path, []models.AgentCard{makeCard("a", "http://a"), makeCard("b", "http://b")})

		r, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(r.List()) != 2 {
			t.Errorf("got %d agents, want 2", len(r.List()))
		}
	})

	t.Run("nonexistent file returns empty", func(t *testing.T) {
		r, err := Load(filepath.Join(t.TempDir(), "nope.json"))
		if err != nil {
			t.Fatal(err)
		}
		if len(r.List()) != 0 {
			t.Errorf("got %d agents, want 0", len(r.List()))
		}
	})

	t.Run("malformed JSON returns error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "bad.json")
		os.WriteFile(path, []byte(`{invalid`), 0644)

		_, err := Load(path)
		if err == nil {
			t.Fatal("expected error for malformed JSON")
		}
	})
}

func TestAddAndGet(t *testing.T) {
	r, _ := Load(filepath.Join(t.TempDir(), "reg.json"))

	card := makeCard("test", "http://test")
	if err := r.Add(card); err != nil {
		t.Fatal(err)
	}

	got, err := r.Get("test")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "test" {
		t.Errorf("got name %q, want test", got.Name)
	}

	t.Run("duplicate returns error", func(t *testing.T) {
		err := r.Add(card)
		if err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("expected duplicate error, got: %v", err)
		}
	})
}

func TestList(t *testing.T) {
	r, _ := Load(filepath.Join(t.TempDir(), "reg.json"))
	r.Add(makeCard("b", "http://b"))
	r.Add(makeCard("a", "http://a"))

	list := r.List()
	if len(list) != 2 {
		t.Fatalf("got %d agents, want 2", len(list))
	}
	// Insertion order preserved
	if list[0].Name != "b" || list[1].Name != "a" {
		t.Errorf("got order [%s, %s], want [b, a]", list[0].Name, list[1].Name)
	}
}

func TestRemove(t *testing.T) {
	r, _ := Load(filepath.Join(t.TempDir(), "reg.json"))
	r.Add(makeCard("a", "http://a"))

	if err := r.Remove("a"); err != nil {
		t.Fatal(err)
	}
	if r.Has("a") {
		t.Error("agent should be removed")
	}

	t.Run("nonexistent returns error", func(t *testing.T) {
		err := r.Remove("nope")
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("expected not found error, got: %v", err)
		}
	})
}

func TestSaveAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reg.json")
	r, _ := Load(path)
	r.Add(makeCard("x", "http://x"))
	r.Add(makeCard("y", "http://y"))

	if err := r.Save(); err != nil {
		t.Fatal(err)
	}

	r2, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(r2.List()) != 2 {
		t.Errorf("got %d agents after reload, want 2", len(r2.List()))
	}
	if r2.List()[0].Name != "x" {
		t.Errorf("first agent should be x, got %s", r2.List()[0].Name)
	}
}

func TestHas(t *testing.T) {
	r, _ := Load(filepath.Join(t.TempDir(), "reg.json"))
	r.Add(makeCard("a", "http://a"))

	if !r.Has("a") {
		t.Error("Has should return true for existing agent")
	}
	if r.Has("z") {
		t.Error("Has should return false for missing agent")
	}
}

func TestExport(t *testing.T) {
	r, _ := Load(filepath.Join(t.TempDir(), "reg.json"))
	r.Add(makeCard("a", "http://a"))
	r.Add(makeCard("b", "http://b"))

	rf := r.Export()
	if len(rf.Agents) != 2 {
		t.Errorf("got %d agents in export, want 2", len(rf.Agents))
	}
}

func TestMerge_NewAgent(t *testing.T) {
	r, _ := Load(filepath.Join(t.TempDir(), "reg.json"))
	result, err := r.Merge([]models.AgentCard{makeCard("a", "http://a")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Added) != 1 || result.Added[0] != "a" {
		t.Errorf("expected 1 added, got %v", result.Added)
	}
	if !r.Has("a") {
		t.Error("agent should be in registry")
	}
}

func TestMerge_IdenticalAgent(t *testing.T) {
	r, _ := Load(filepath.Join(t.TempDir(), "reg.json"))
	card := makeCard("a", "http://a")
	r.Add(card)

	result, err := r.Merge([]models.AgentCard{card}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Skipped) != 1 {
		t.Errorf("expected 1 skipped, got %v", result.Skipped)
	}
}

func TestMerge_ConflictOverwrite(t *testing.T) {
	r, _ := Load(filepath.Join(t.TempDir(), "reg.json"))
	r.Add(makeCard("a", "http://old"))

	resolver := func(local, imported models.AgentCard) MergeAction {
		return MergeAction{Action: "overwrite"}
	}

	result, err := r.Merge([]models.AgentCard{makeCard("a", "http://new")}, resolver)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Replaced) != 1 {
		t.Errorf("expected 1 replaced, got %v", result.Replaced)
	}
	got, _ := r.Get("a")
	if got.URL != "http://new" {
		t.Errorf("URL should be updated to http://new, got %s", got.URL)
	}
}

func TestMerge_ConflictSkip(t *testing.T) {
	r, _ := Load(filepath.Join(t.TempDir(), "reg.json"))
	r.Add(makeCard("a", "http://old"))

	resolver := func(local, imported models.AgentCard) MergeAction {
		return MergeAction{Action: "skip"}
	}

	result, err := r.Merge([]models.AgentCard{makeCard("a", "http://new")}, resolver)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Skipped) != 1 {
		t.Errorf("expected 1 skipped, got %v", result.Skipped)
	}
	got, _ := r.Get("a")
	if got.URL != "http://old" {
		t.Errorf("URL should remain http://old, got %s", got.URL)
	}
}

func TestMerge_ConflictRename(t *testing.T) {
	r, _ := Load(filepath.Join(t.TempDir(), "reg.json"))
	r.Add(makeCard("a", "http://old"))

	resolver := func(local, imported models.AgentCard) MergeAction {
		return MergeAction{Action: "rename", Name: "a2"}
	}

	result, err := r.Merge([]models.AgentCard{makeCard("a", "http://new")}, resolver)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Renamed) != 1 || result.Renamed["a"] != "a2" {
		t.Errorf("expected renamed a->a2, got %v", result.Renamed)
	}
	if !r.Has("a") && !r.Has("a2") {
		t.Error("both a and a2 should exist")
	}
}

func TestMerge_InvalidCard(t *testing.T) {
	r, _ := Load(filepath.Join(t.TempDir(), "reg.json"))
	_, err := r.Merge([]models.AgentCard{{Name: ""}}, nil)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestMerge_Multiple(t *testing.T) {
	r, _ := Load(filepath.Join(t.TempDir(), "reg.json"))
	r.Add(makeCard("existing", "http://existing"))
	r.Add(makeCard("conflict", "http://old"))

	cards := []models.AgentCard{
		makeCard("new", "http://new"),
		makeCard("existing", "http://existing"),         // identical → skip
		makeCard("conflict", "http://different"),        // conflict → overwrite
	}

	resolver := func(local, imported models.AgentCard) MergeAction {
		return MergeAction{Action: "overwrite"}
	}

	result, err := r.Merge(cards, resolver)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Added) != 1 {
		t.Errorf("expected 1 added, got %d", len(result.Added))
	}
	if len(result.Skipped) != 1 {
		t.Errorf("expected 1 skipped, got %d", len(result.Skipped))
	}
	if len(result.Replaced) != 1 {
		t.Errorf("expected 1 replaced, got %d", len(result.Replaced))
	}
}

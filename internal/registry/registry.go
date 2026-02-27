package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/luischavesdev/society/internal/models"
)

type Registry struct {
	path   string
	agents map[string]models.AgentCard
	order  []string
}

type ConflictResolver func(local, imported models.AgentCard) MergeAction

type MergeAction struct {
	Action string // "overwrite", "skip", "rename"
	Name   string // new name if Action == "rename"
}

type MergeResult struct {
	Added    []string
	Skipped  []string
	Replaced []string
	Renamed  map[string]string
}

func Load(path string) (*Registry, error) {
	r := &Registry{
		path:   path,
		agents: make(map[string]models.AgentCard),
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return r, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading registry: %w", err)
	}

	var rf models.RegistryFile
	if err := json.Unmarshal(data, &rf); err != nil {
		return nil, fmt.Errorf("parsing registry: %w", err)
	}

	for _, a := range rf.Agents {
		r.agents[a.Name] = a
		r.order = append(r.order, a.Name)
	}
	return r, nil
}

func (r *Registry) Get(name string) (models.AgentCard, error) {
	a, ok := r.agents[name]
	if !ok {
		return models.AgentCard{}, fmt.Errorf("agent %q not found", name)
	}
	return a, nil
}

func (r *Registry) List() []models.AgentCard {
	result := make([]models.AgentCard, 0, len(r.order))
	for _, name := range r.order {
		if a, ok := r.agents[name]; ok {
			result = append(result, a)
		}
	}
	return result
}

func (r *Registry) Add(card models.AgentCard) error {
	if err := models.ValidateAgentCard(card); err != nil {
		return err
	}
	if _, exists := r.agents[card.Name]; exists {
		return fmt.Errorf("agent %q already exists", card.Name)
	}
	r.agents[card.Name] = card
	r.order = append(r.order, card.Name)
	return nil
}

func (r *Registry) Remove(name string) error {
	if _, exists := r.agents[name]; !exists {
		return fmt.Errorf("agent %q not found", name)
	}
	delete(r.agents, name)
	for i, n := range r.order {
		if n == name {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
	return nil
}

func (r *Registry) Save() error {
	rf := models.RegistryFile{Agents: r.List()}
	data, err := json.MarshalIndent(rf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling registry: %w", err)
	}
	data = append(data, '\n')
	if dir := filepath.Dir(r.path); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating registry directory: %w", err)
		}
	}
	if err := os.WriteFile(r.path, data, 0644); err != nil {
		return fmt.Errorf("writing registry: %w", err)
	}
	return nil
}

func (r *Registry) Has(name string) bool {
	_, ok := r.agents[name]
	return ok
}

func (r *Registry) Export() models.RegistryFile {
	return models.RegistryFile{Agents: r.List()}
}

func (r *Registry) Merge(cards []models.AgentCard, resolve ConflictResolver) (*MergeResult, error) {
	// Validate all first
	for i, c := range cards {
		if err := models.ValidateAgentCard(c); err != nil {
			return nil, fmt.Errorf("imported agent[%d] (%s): %w", i, c.Name, err)
		}
	}

	result := &MergeResult{Renamed: make(map[string]string)}

	for _, c := range cards {
		local, exists := r.agents[c.Name]
		if !exists {
			r.agents[c.Name] = c
			r.order = append(r.order, c.Name)
			result.Added = append(result.Added, c.Name)
			continue
		}

		if cardsEqualIgnoringTransport(local, c) {
			result.Skipped = append(result.Skipped, c.Name)
			continue
		}

		action := resolve(local, c)
		switch action.Action {
		case "overwrite":
			r.agents[c.Name] = c
			result.Replaced = append(result.Replaced, c.Name)
		case "skip":
			result.Skipped = append(result.Skipped, c.Name)
		case "rename":
			renamed := c
			renamed.Name = action.Name
			r.agents[action.Name] = renamed
			r.order = append(r.order, action.Name)
			result.Renamed[c.Name] = action.Name
		}
	}

	return result, nil
}

func cardsEqualIgnoringTransport(a, b models.AgentCard) bool {
	aCopy := a
	bCopy := b
	aCopy.Transport = nil
	bCopy.Transport = nil
	return reflect.DeepEqual(aCopy, bCopy)
}

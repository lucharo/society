package thread

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Thread struct {
	ID        string    `json:"id"`
	Agent     string    `json:"agent"`
	SessionID string    `json:"session_id,omitempty"`
	Messages  []Entry   `json:"messages"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Entry struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

type Store struct {
	dir string
}

func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

func DefaultStore() *Store {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return NewStore(filepath.Join(home, ".society", "threads"))
}

func (s *Store) Load(id string) (*Thread, error) {
	path := s.path(id)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var t Thread
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) Save(t *Thread) error {
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return err
	}
	t.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(t.ID), data, 0644)
}

func (s *Store) List(agent string) ([]*Thread, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var threads []*Thread
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		t, err := s.Load(id)
		if err != nil || t == nil {
			continue
		}
		if agent == "" || t.Agent == agent {
			threads = append(threads, t)
		}
	}
	return threads, nil
}

func (s *Store) path(id string) string {
	return filepath.Join(s.dir, id+".json")
}

package agent

import (
	"fmt"
	"os"

	"github.com/luischavesdev/society/internal/models"
	"gopkg.in/yaml.v3"
)

func LoadConfig(path string) (*models.AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg models.AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if err := models.ValidateAgentConfig(cfg); err != nil {
		return nil, err
	}
	if cfg.Backend != nil {
		cfg.Backend.ApplyDefaults()
	}
	return &cfg, nil
}

package agent

import (
	"fmt"
	"github.com/tk103331/eino-cli/config"
)

// Factory is used to create Agent instances
type Factory struct {
	cfg *config.Config
}

// NewFactory creates a new Factory
func NewFactory(cfg *config.Config) *Factory {
	return &Factory{cfg: cfg}
}

// CreateAgent creates Agent based on name
func (f *Factory) CreateAgent(name string) (Agent, error) {
	// Get Agent configuration
	agentCfg, ok := f.cfg.Agents[name]
	if !ok {
		return nil, fmt.Errorf("Agent configuration does not exist: %s", name)
	}

	// Create ReactAgent
	agent := NewReactAgent(name, &agentCfg)
	return agent, nil
}

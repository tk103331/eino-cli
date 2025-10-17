package agent

import (
	"fmt"
	"github.com/tk103331/eino-cli/config"
)

// Factory 用于创建Agent实例
type Factory struct {
	cfg *config.Config
}

// NewFactory 创建一个新的Factory
func NewFactory(cfg *config.Config) *Factory {
	return &Factory{cfg: cfg}
}

// CreateAgent 根据名称创建Agent
func (f *Factory) CreateAgent(name string) (Agent, error) {
	// 获取Agent配置
	agentCfg, ok := f.cfg.Agents[name]
	if !ok {
		return nil, fmt.Errorf("Agent配置不存在: %s", name)
	}

	// 创建ReactAgent
	agent := NewReactAgent(name, &agentCfg)
	return agent, nil
}

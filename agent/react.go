package agent

import (
	"context"
	"fmt"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/tk103331/eino-cli/config"
	modelFactory "github.com/tk103331/eino-cli/model"
)

// ReactAgent 实现了使用cloudwego/eino库的React模式的Agent
type ReactAgent struct {
	config *config.Agent
	agent  *react.Agent
	ctx    context.Context
}

// NewReactAgent 创建一个新的ReactAgent
func NewReactAgent(cfg *config.Agent) *ReactAgent {
	return &ReactAgent{
		config: cfg,
		ctx:    context.Background(),
	}
}

// Init 初始化Agent
func (r *ReactAgent) Init() error {
	// 创建模型
	model, err := r.createModel()
	if err != nil {
		return fmt.Errorf("创建模型失败: %w", err)
	}
	
	// 创建工具配置
	toolsConfig, err := r.createToolsConfig()
	if err != nil {
		return fmt.Errorf("创建工具配置失败: %w", err)
	}
	
	// 创建Agent配置
	agentConfig := &react.AgentConfig{
		ToolCallingModel: model,
		ToolsConfig:      toolsConfig,
	}
	
	// 创建Agent
	agent, err := react.NewAgent(r.ctx, agentConfig)
	if err != nil {
		return fmt.Errorf("创建Agent失败: %w", err)
	}
	
	// 保存agent实例
	r.agent = agent
	return nil
}

// Run 运行Agent
func (r *ReactAgent) Run(prompt string) error {
	if r.agent == nil {
		if err := r.Init(); err != nil {
			return err
		}
	}
	
	// 创建用户消息
	messages := []*schema.Message{
		schema.SystemMessage(r.config.System),
		schema.UserMessage(prompt),
	}
	
	// 使用Agent生成响应
	response, err := r.agent.Generate(r.ctx, messages)
	if err != nil {
		return fmt.Errorf("运行Agent失败: %w", err)
	}
	
	// 打印响应
	fmt.Println(response.Content)
	return nil
}

// createModel 创建模型
func (r *ReactAgent) createModel() (model.ToolCallingChatModel, error) {
	// 从Factory获取全局配置
	globalCfg := config.GetConfig()
	if globalCfg == nil {
		return nil, fmt.Errorf("全局配置未初始化")
	}

	// 创建模型工厂
	factory := modelFactory.NewFactory(globalCfg)

	// 使用工厂创建模型
	return factory.CreateChatModel(r.ctx, r.config.Model)
}

// createToolsConfig 创建工具配置
func (r *ReactAgent) createToolsConfig() (compose.ToolsNodeConfig, error) {
	// 创建工具配置
	toolsConfig := compose.ToolsNodeConfig{
		Tools: []tool.BaseTool{},
	}
	
	// 获取全局配置
	globalCfg := config.GetConfig()
	if globalCfg == nil {
		return toolsConfig, fmt.Errorf("全局配置未初始化")
	}
	
	// 添加工具
	for _, toolName := range r.config.Tools {
		// 获取工具配置
		toolCfg, ok := globalCfg.Tools[toolName]
		if !ok {
			return toolsConfig, fmt.Errorf("工具配置不存在: %s", toolName)
		}
		
		// 创建工具实例
		toolInstance, err := createTool(toolName, toolCfg)
		if err != nil {
			return toolsConfig, err
		}
		
		toolsConfig.Tools = append(toolsConfig.Tools, toolInstance)
	}
	
	return toolsConfig, nil
}
package model

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/tk103331/eino-cli/config"
)

// Factory 用于创建不同 provider 的 ChatModel
type Factory struct {
	cfg *config.Config
}

// NewFactory 创建一个新的 Factory
func NewFactory(cfg *config.Config) *Factory {
	return &Factory{cfg: cfg}
}

// CreateChatModel 根据模型名称创建对应的 ChatModel
func (f *Factory) CreateChatModel(ctx context.Context, modelName string) (model.ToolCallingChatModel, error) {
	// 获取模型配置
	modelCfg, ok := f.cfg.Models[modelName]
	if !ok {
		return nil, fmt.Errorf("模型配置不存在: %s", modelName)
	}

	// 获取提供商配置
	providerCfg, ok := f.cfg.Providers[modelCfg.Provider]
	if !ok {
		return nil, fmt.Errorf("提供商配置不存在: %s", modelCfg.Provider)
	}

	// 根据提供商类型创建对应的模型
	switch providerCfg.Type {
	case "openai":
		return f.createOpenAIModel(ctx, &modelCfg, &providerCfg)
	case "claude":
		return f.createClaudeModel(ctx, &modelCfg, &providerCfg)
	case "gemini":
		return f.createGeminiModel(ctx, &modelCfg, &providerCfg)
	case "qwen":
		return f.createQwenModel(ctx, &modelCfg, &providerCfg)
	case "qianfan":
		return f.createQianfanModel(ctx, &modelCfg, &providerCfg)
	case "ark":
		return f.createArkModel(ctx, &modelCfg, &providerCfg)
	case "deepseek":
		return f.createDeepSeekModel(ctx, &modelCfg, &providerCfg)
	case "ollama":
		return f.createOllamaModel(ctx, &modelCfg, &providerCfg)
	default:
		return nil, fmt.Errorf("不支持的提供商类型: %s", providerCfg.Type)
	}
}
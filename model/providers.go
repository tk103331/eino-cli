package model

import (
	"context"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/gemini"
	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino-ext/components/model/qianfan"
	"github.com/cloudwego/eino-ext/components/model/qwen"
	"github.com/cloudwego/eino/components/model"
	"github.com/tk103331/eino-cli/config"
)

// createOpenAIModel 创建 OpenAI 模型
func (f *Factory) createOpenAIModel(ctx context.Context, modelCfg *config.Model, providerCfg *config.Provider) (model.ToolCallingChatModel, error) {
	cfg := &openai.ChatModelConfig{
		Model:   modelCfg.Model,
		BaseURL: providerCfg.BaseURL,
		APIKey:  providerCfg.APIKey,
	}

	if modelCfg.MaxTokens > 0 {
		cfg.MaxTokens = &modelCfg.MaxTokens
	}
	if modelCfg.Temperature > 0 {
		temp := float32(modelCfg.Temperature)
		cfg.Temperature = &temp
	}
	if modelCfg.TopP > 0 {
		topP := float32(modelCfg.TopP)
		cfg.TopP = &topP
	}

	return openai.NewChatModel(ctx, cfg)
}

// createClaudeModel 创建 Claude 模型
func (f *Factory) createClaudeModel(ctx context.Context, modelCfg *config.Model, providerCfg *config.Provider) (model.ToolCallingChatModel, error) {
	cfg := &claude.Config{
		Model:   modelCfg.Model,
		BaseURL: &(providerCfg.BaseURL),
		APIKey:  providerCfg.APIKey,
	}
	if modelCfg.MaxTokens > 0 {
		cfg.MaxTokens = modelCfg.MaxTokens
	}
	if modelCfg.Temperature > 0 {
		temp := float32(modelCfg.Temperature)
		cfg.Temperature = &temp
	}
	if modelCfg.TopP > 0 {
		topP := float32(modelCfg.TopP)
		cfg.TopP = &topP
	}

	return claude.NewChatModel(ctx, cfg)
}

// createGeminiModel 创建 Gemini 模型
func (f *Factory) createGeminiModel(ctx context.Context, modelCfg *config.Model, providerCfg *config.Provider) (model.ToolCallingChatModel, error) {
	cfg := &gemini.Config{
		Model: modelCfg.Model,
	}

	if modelCfg.MaxTokens > 0 {
		cfg.MaxTokens = &modelCfg.MaxTokens
	}
	if modelCfg.Temperature > 0 {
		temp := float32(modelCfg.Temperature)
		cfg.Temperature = &temp
	}
	if modelCfg.TopP > 0 {
		topP := float32(modelCfg.TopP)
		cfg.TopP = &topP
	}

	return gemini.NewChatModel(ctx, cfg)
}

// createQwenModel 创建 Qwen 模型
func (f *Factory) createQwenModel(ctx context.Context, modelCfg *config.Model, providerCfg *config.Provider) (model.ToolCallingChatModel, error) {
	cfg := &qwen.ChatModelConfig{
		Model:   modelCfg.Model,
		BaseURL: providerCfg.BaseURL,
		APIKey:  providerCfg.APIKey,
	}

	if modelCfg.MaxTokens > 0 {
		cfg.MaxTokens = &modelCfg.MaxTokens
	}
	if modelCfg.Temperature > 0 {
		temp := float32(modelCfg.Temperature)
		cfg.Temperature = &temp
	}
	if modelCfg.TopP > 0 {
		topP := float32(modelCfg.TopP)
		cfg.TopP = &topP
	}

	return qwen.NewChatModel(ctx, cfg)
}

// createQianfanModel 创建 Qianfan 模型
func (f *Factory) createQianfanModel(ctx context.Context, modelCfg *config.Model, providerCfg *config.Provider) (model.ToolCallingChatModel, error) {
	cfg := &qianfan.ChatModelConfig{
		Model: modelCfg.Model,
	}

	// Qianfan 配置可能需要根据实际 API 调整
	// 这里提供基本配置，实际使用时可能需要根据具体需求配置

	return qianfan.NewChatModel(ctx, cfg)
}

// createArkModel 创建 Ark 模型
func (f *Factory) createArkModel(ctx context.Context, modelCfg *config.Model, providerCfg *config.Provider) (model.ToolCallingChatModel, error) {
	cfg := &ark.ChatModelConfig{
		Model:   modelCfg.Model,
		BaseURL: providerCfg.BaseURL,
		APIKey:  providerCfg.APIKey,
	}

	if modelCfg.MaxTokens > 0 {
		cfg.MaxTokens = &modelCfg.MaxTokens
	}
	if modelCfg.Temperature > 0 {
		temp := float32(modelCfg.Temperature)
		cfg.Temperature = &temp
	}
	if modelCfg.TopP > 0 {
		topP := float32(modelCfg.TopP)
		cfg.TopP = &topP
	}

	return ark.NewChatModel(ctx, cfg)
}

// createDeepSeekModel 创建 DeepSeek 模型
func (f *Factory) createDeepSeekModel(ctx context.Context, modelCfg *config.Model, providerCfg *config.Provider) (model.ToolCallingChatModel, error) {
	cfg := &deepseek.ChatModelConfig{
		Model:   modelCfg.Model,
		BaseURL: providerCfg.BaseURL,
		APIKey:  providerCfg.APIKey,
	}

	if modelCfg.MaxTokens > 0 {
		cfg.MaxTokens = modelCfg.MaxTokens
	}
	if modelCfg.Temperature > 0 {
		temp := float32(modelCfg.Temperature)
		cfg.Temperature = temp
	}
	if modelCfg.TopP > 0 {
		topP := float32(modelCfg.TopP)
		cfg.TopP = topP
	}

	return deepseek.NewChatModel(ctx, cfg)
}

// createOllamaModel 创建 Ollama 模型
func (f *Factory) createOllamaModel(ctx context.Context, modelCfg *config.Model, providerCfg *config.Provider) (model.ToolCallingChatModel, error) {
	cfg := &ollama.ChatModelConfig{
		Model:   modelCfg.Model,
		BaseURL: providerCfg.BaseURL,
	}

	// Ollama 的配置通过 Options 字段设置
	// 这里暂时简化处理，实际使用时可能需要根据具体需求配置 Options

	return ollama.NewChatModel(ctx, cfg)
}

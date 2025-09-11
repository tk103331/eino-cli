package tools

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino-ext/components/tool/googlesearch"
	"github.com/tk103331/eino-cli/config"
)

// NewGoogleSearchTool 创建Google搜索工具
func NewGoogleSearchTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	ctx := context.Background()

	// 创建Google搜索配置
	googleConfig := &googlesearch.Config{
		ToolName: name,
		ToolDesc: cfg.Description,
		Num:      5,       // 默认返回5个结果
		Lang:     "zh-CN", // 默认中文
	}

	// 从配置中读取必需和可选参数
	if apiKey, exists := cfg.Config["api_key"]; exists {
		googleConfig.APIKey = apiKey.String()
	}
	if searchEngineID, exists := cfg.Config["search_engine_id"]; exists {
		googleConfig.SearchEngineID = searchEngineID.String()
	}
	if baseURL, exists := cfg.Config["base_url"]; exists {
		googleConfig.BaseURL = baseURL.String()
	}
	if num, exists := cfg.Config["num"]; exists {
		if val := num.Int(); val > 0 {
			googleConfig.Num = val
		}
	}
	if lang, exists := cfg.Config["lang"]; exists {
		googleConfig.Lang = lang.String()
	}

	// 创建工具实例
	return googlesearch.NewTool(ctx, googleConfig)
}
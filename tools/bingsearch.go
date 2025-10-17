package tools

import (
	"context"

	"github.com/cloudwego/eino-ext/components/tool/bingsearch"
	"github.com/cloudwego/eino/components/tool"
	"github.com/tk103331/eino-cli/config"
)

// NewBingSearchTool 创建Bing搜索工具
func NewBingSearchTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	ctx := context.Background()

	// 创建Bing搜索配置
	bingConfig := &bingsearch.Config{
		ToolName:   name,
		ToolDesc:   cfg.Description,
		MaxResults: 5, // 默认返回5个结果
	}

	// 从配置中读取必需和可选参数
	if apiKey, exists := cfg.Config["api_key"]; exists {
		bingConfig.APIKey = apiKey.String()
	}
	if maxResults, exists := cfg.Config["max_results"]; exists {
		if val := maxResults.Int(); val > 0 {
			bingConfig.MaxResults = val
		}
	}

	// 创建工具实例
	return bingsearch.NewTool(ctx, bingConfig)
}

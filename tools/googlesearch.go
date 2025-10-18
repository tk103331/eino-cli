package tools

import (
	"context"

	"github.com/cloudwego/eino-ext/components/tool/googlesearch"
	"github.com/cloudwego/eino/components/tool"
	"github.com/tk103331/eino-cli/config"
)

// NewGoogleSearchTool creates Google search tool
func NewGoogleSearchTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	ctx := context.Background()

	// Create Google search configuration
	googleConfig := &googlesearch.Config{
		ToolName: name,
		ToolDesc: cfg.Description,
		Num:      5,       // Default return 5 results
		Lang:     "zh-CN", // Default Chinese
	}

	// Read required and optional parameters from configuration
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

	// Create tool instance
	return googlesearch.NewTool(ctx, googleConfig)
}

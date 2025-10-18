package tools

import (
	"context"

	"github.com/cloudwego/eino-ext/components/tool/bingsearch"
	"github.com/cloudwego/eino/components/tool"
	"github.com/tk103331/eino-cli/config"
)

// NewBingSearchTool creates Bing search tool
func NewBingSearchTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	ctx := context.Background()

	// Create Bing search configuration
	bingConfig := &bingsearch.Config{
		ToolName:   name,
		ToolDesc:   cfg.Description,
		MaxResults: 5, // Default return 5 results
	}

	// Read required and optional parameters from configuration
	if apiKey, exists := cfg.Config["api_key"]; exists {
		bingConfig.APIKey = apiKey.String()
	}
	if maxResults, exists := cfg.Config["max_results"]; exists {
		if val := maxResults.Int(); val > 0 {
			bingConfig.MaxResults = val
		}
	}

	// Create tool instance
	return bingsearch.NewTool(ctx, bingConfig)
}

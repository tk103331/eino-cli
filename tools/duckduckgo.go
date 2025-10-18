package tools

import (
	"context"
	"time"

	"github.com/cloudwego/eino-ext/components/tool/duckduckgo"
	"github.com/cloudwego/eino-ext/components/tool/duckduckgo/ddgsearch"
	"github.com/cloudwego/eino/components/tool"
	"github.com/tk103331/eino-cli/config"
)

// NewDuckDuckGoTool creates DuckDuckGo search tool
func NewDuckDuckGoTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	ctx := context.Background()

	// Create DuckDuckGo configuration
	ddgConfig := &duckduckgo.Config{
		ToolName:   name,
		ToolDesc:   cfg.Description,
		Region:     ddgsearch.RegionWT, // Default global
		MaxResults: 10,                 // Default return 10 results
		SafeSearch: ddgsearch.SafeSearchOff,
		TimeRange:  ddgsearch.TimeRangeAll,
		DDGConfig: &ddgsearch.Config{
			Timeout:    10 * time.Second,
			Cache:      true,
			MaxRetries: 5,
		},
	}

	// Read custom parameters from configuration
	if maxResults, exists := cfg.Config["max_results"]; exists {
		if val := maxResults.Int(); val > 0 {
			ddgConfig.MaxResults = val
		}
	}
	if region, exists := cfg.Config["region"]; exists {
		switch region.String() {
		case "cn":
			ddgConfig.Region = ddgsearch.RegionCN
		case "us":
			ddgConfig.Region = ddgsearch.RegionUS
		case "uk":
			ddgConfig.Region = ddgsearch.RegionUK
		default:
			ddgConfig.Region = ddgsearch.RegionWT
		}
	}
	if safeSearch, exists := cfg.Config["safe_search"]; exists {
		switch safeSearch.String() {
		case "strict":
			ddgConfig.SafeSearch = ddgsearch.SafeSearchStrict
		case "moderate":
			ddgConfig.SafeSearch = ddgsearch.SafeSearchModerate
		default:
			ddgConfig.SafeSearch = ddgsearch.SafeSearchOff
		}
	}
	if timeout, exists := cfg.Config["timeout"]; exists {
		if val := timeout.Int(); val > 0 {
			ddgConfig.DDGConfig.Timeout = time.Duration(val) * time.Second
		}
	}

	// Create tool instance
	return duckduckgo.NewTool(ctx, ddgConfig)
}

package tools

import (
	"context"
	"time"

	"github.com/cloudwego/eino-ext/components/tool/wikipedia"
	"github.com/cloudwego/eino/components/tool"
	"github.com/tk103331/eino-cli/config"
)

// NewWikipediaTool creates Wikipedia tool
func NewWikipediaTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	ctx := context.Background()

	// Create default configuration
	wikiConfig := &wikipedia.Config{
		Language:    "zh",             // Default Chinese
		TopK:        5,                // Default return 5 results
		DocMaxChars: 500,              // Default summary max length
		Timeout:     15 * time.Second, // Default timeout
		MaxRedirect: 3,                // Default max redirect count
	}

	// Read parameters from configuration
	if language, exists := cfg.Config["language"]; exists {
		wikiConfig.Language = language.String()
	}
	if topK, exists := cfg.Config["top_k"]; exists {
		if val := topK.Int(); val > 0 {
			wikiConfig.TopK = val
		}
	}
	if docMaxChars, exists := cfg.Config["doc_max_chars"]; exists {
		if val := docMaxChars.Int(); val > 0 {
			wikiConfig.DocMaxChars = val
		}
	}
	if timeoutConfig, exists := cfg.Config["timeout"]; exists {
		if val := timeoutConfig.Int(); val > 0 {
			wikiConfig.Timeout = time.Duration(val) * time.Second
		}
	}
	if baseURL, exists := cfg.Config["base_url"]; exists {
		wikiConfig.BaseURL = baseURL.String()
	}
	if userAgent, exists := cfg.Config["user_agent"]; exists {
		wikiConfig.UserAgent = userAgent.String()
	}
	if maxRedirect, exists := cfg.Config["max_redirect"]; exists {
		if val := maxRedirect.Int(); val > 0 {
			wikiConfig.MaxRedirect = val
		}
	}

	// Create tool instance
	return wikipedia.NewTool(ctx, wikiConfig)
}

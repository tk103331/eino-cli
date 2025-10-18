package tools

import (
	"context"

	"github.com/cloudwego/eino-ext/components/tool/browseruse"
	"github.com/cloudwego/eino/components/tool"
	"github.com/tk103331/eino-cli/config"
)

// NewBrowserUseTool creates browser usage tool
func NewBrowserUseTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	ctx := context.Background()

	// Create BrowserUse configuration
	browserConfig := &browseruse.Config{}

	// Read optional parameters from configuration
	for _, param := range cfg.Params {
		switch param.Name {
		// More configuration items can be added here based on actual Config structure
		// Currently browseruse.Config may contain browser-related configurations
		default:
			// Temporarily don't handle unknown parameters
		}
	}

	// Create tool instance
	return browseruse.NewBrowserUseTool(ctx, browserConfig)
}

package tools

import (
	"github.com/cloudwego/eino-ext/components/tool/sequentialthinking"
	"github.com/cloudwego/eino/components/tool"
	"github.com/tk103331/eino-cli/config"
)

// NewSequentialThinkingTool creates sequential thinking tool
func NewSequentialThinkingTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	// Sequential Thinking tool doesn't need special configuration, create directly
	return sequentialthinking.NewTool()
}

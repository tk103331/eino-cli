package tools

import (
	"github.com/cloudwego/eino-ext/components/tool/sequentialthinking"
	"github.com/cloudwego/eino/components/tool"
	"github.com/tk103331/eino-cli/config"
)

// NewSequentialThinkingTool 创建顺序思考工具
func NewSequentialThinkingTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	// Sequential Thinking工具不需要特殊配置，直接创建
	return sequentialthinking.NewTool()
}

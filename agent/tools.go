package agent

import (
	"github.com/cloudwego/eino/components/tool"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/tools"
)

// createTool 创建工具实例
func createTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	return tools.CreateTool(name, cfg)
}

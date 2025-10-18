package agent

import (
	"github.com/cloudwego/eino/components/tool"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/tools"
)

// createTool creates tool instances
func createTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	return tools.CreateTool(name, cfg)
}

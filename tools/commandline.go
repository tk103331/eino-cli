package tools

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino-ext/components/tool/commandline"
	"github.com/tk103331/eino-cli/config"
)

// NewCommandLineTool 创建命令行编辑器工具
func NewCommandLineTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	ctx := context.Background()

	// 创建编辑器配置
	editorConfig := &commandline.EditorConfig{
		// 使用默认的操作器
		Operator: nil, // 将使用默认实现
	}

	// 创建工具实例
	return commandline.NewStrReplaceEditor(ctx, editorConfig)
}
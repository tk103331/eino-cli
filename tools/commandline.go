package tools

import (
	"context"

	"github.com/cloudwego/eino-ext/components/tool/commandline"
	"github.com/cloudwego/eino/components/tool"
	"github.com/tk103331/eino-cli/config"
)

// NewCommandLineTool creates command line editor tool
func NewCommandLineTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	ctx := context.Background()

	// Create editor configuration
	editorConfig := &commandline.EditorConfig{
		// Use default operator
		Operator: nil, // Will use default implementation
	}

	// Create tool instance
	return commandline.NewStrReplaceEditor(ctx, editorConfig)
}

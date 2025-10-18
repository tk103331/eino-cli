package tools

import (
	"fmt"
	"github.com/tk103331/eino-cli/tools/custom"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/tk103331/eino-cli/config"
)

// CreateTool creates tool instance based on configuration
func CreateTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	switch strings.ToLower(cfg.Type) {
	case "customhttp":
		return custom.NewHTTPTool(name, cfg)
	case "customexec":
		return custom.NewExecTool(name, cfg)
	case "bingsearch":
		return NewBingSearchTool(name, cfg)
	case "browseruse":
		return NewBrowserUseTool(name, cfg)
	case "commandline":
		return NewCommandLineTool(name, cfg)
	case "duckduckgo":
		return NewDuckDuckGoTool(name, cfg)
	case "googlesearch":
		return NewGoogleSearchTool(name, cfg)
	case "httprequest":
		return NewHTTPRequestTool(name, cfg)
	case "sequentialthinking":
		return NewSequentialThinkingTool(name, cfg)
	case "wikipedia":
		return NewWikipediaTool(name, cfg)
	default:
		return nil, fmt.Errorf("unsupported tool type: %s", cfg.Type)
	}
}

// CreateToolsFromConfig creates all tools from configuration
func CreateToolsFromConfig(cfg *config.Config) (map[string]tool.InvokableTool, error) {
	tools := make(map[string]tool.InvokableTool)

	for name, toolCfg := range cfg.Tools {
		toolInstance, err := CreateTool(name, toolCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create tool %s: %v", name, err)
		}
		tools[name] = toolInstance
	}

	return tools, nil
}

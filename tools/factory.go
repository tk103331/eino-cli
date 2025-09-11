package tools

import (
	"fmt"
	"github.com/tk103331/eino-cli/tools/custom"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/tk103331/eino-cli/config"
)

// CreateTool 根据配置创建工具实例
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
		return nil, fmt.Errorf("不支持的工具类型: %s", cfg.Type)
	}
}

// CreateToolsFromConfig 从配置中创建所有工具
func CreateToolsFromConfig(cfg *config.Config) (map[string]tool.InvokableTool, error) {
	tools := make(map[string]tool.InvokableTool)

	for name, toolCfg := range cfg.Tools {
		toolInstance, err := CreateTool(name, toolCfg)
		if err != nil {
			return nil, fmt.Errorf("创建工具 %s 失败: %v", name, err)
		}
		tools[name] = toolInstance
	}

	return tools, nil
}

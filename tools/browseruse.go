package tools

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino-ext/components/tool/browseruse"
	"github.com/tk103331/eino-cli/config"
)

// NewBrowserUseTool 创建浏览器使用工具
func NewBrowserUseTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	ctx := context.Background()

	// 创建BrowserUse配置
	browserConfig := &browseruse.Config{}

	// 从配置中读取可选参数
	for _, param := range cfg.Params {
		switch param.Name {
		// 这里可以根据实际的Config结构添加更多配置项
		// 目前browseruse.Config可能包含浏览器相关的配置
		default:
			// 暂时不处理未知参数
		}
	}

	// 创建工具实例
	return browseruse.NewBrowserUseTool(ctx, browserConfig)
}
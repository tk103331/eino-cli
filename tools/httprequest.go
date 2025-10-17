package tools

import (
	"context"
	"net/http"
	"time"

	getTool "github.com/cloudwego/eino-ext/components/tool/httprequest/get"
	postTool "github.com/cloudwego/eino-ext/components/tool/httprequest/post"
	"github.com/cloudwego/eino/components/tool"
	"github.com/tk103331/eino-cli/config"
)

// NewHTTPRequestTool 创建HTTP请求工具
func NewHTTPRequestTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	ctx := context.Background()

	// 默认配置
	headers := make(map[string]string)
	timeout := 30 * time.Second
	method := "GET" // 默认GET方法

	// 从配置中读取参数
	if timeoutConfig, exists := cfg.Config["timeout"]; exists {
		if val := timeoutConfig.Int(); val > 0 {
			timeout = time.Duration(val) * time.Second
		}
	}
	if userAgent, exists := cfg.Config["user_agent"]; exists {
		headers["User-Agent"] = userAgent.String()
	}
	if methodConfig, exists := cfg.Config["method"]; exists {
		method = methodConfig.String()
	}
	if headersConfig, exists := cfg.Config["headers"]; exists {
		if headersMap := headersConfig.Map(); headersMap != nil {
			for k, v := range headersMap {
				headers[k] = v.String()
			}
		}
	}

	// 创建HTTP客户端
	httpClient := &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{},
	}

	// 根据方法类型创建相应的工具
	switch method {
	case "POST":
		postConfig := &postTool.Config{
			Headers:    headers,
			HttpClient: httpClient,
		}
		return postTool.NewTool(ctx, postConfig)
	default: // GET或其他方法默认使用GET
		getConfig := &getTool.Config{
			Headers:    headers,
			HttpClient: httpClient,
		}
		return getTool.NewTool(ctx, getConfig)
	}
}

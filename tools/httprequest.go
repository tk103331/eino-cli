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

// NewHTTPRequestTool creates HTTP request tool
func NewHTTPRequestTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	ctx := context.Background()

	// Default configuration
	headers := make(map[string]string)
	timeout := 30 * time.Second
	method := "GET" // Default GET method

	// Read parameters from configuration
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

	// Create HTTP client
	httpClient := &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{},
	}

	// Create corresponding tool based on method type
	switch method {
	case "POST":
		postConfig := &postTool.Config{
			Headers:    headers,
			HttpClient: httpClient,
		}
		return postTool.NewTool(ctx, postConfig)
	default: // GET or other methods default to GET
		getConfig := &getTool.Config{
			Headers:    headers,
			HttpClient: httpClient,
		}
		return getTool.NewTool(ctx, getConfig)
	}
}

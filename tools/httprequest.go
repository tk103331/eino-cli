package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	getTool "github.com/cloudwego/eino-ext/components/tool/httprequest/get"
	postTool "github.com/cloudwego/eino-ext/components/tool/httprequest/post"
	"github.com/cloudwego/eino/components/tool"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/tools/approval"
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

	// Create corresponding tool based on method type.
	method = strings.ToUpper(strings.TrimSpace(method))
	var inner tool.InvokableTool
	var err error
	switch method {
	case "POST":
		inner, err = postTool.NewTool(ctx, &postTool.Config{
			ToolName:   name,
			ToolDesc:   cfg.Description,
			Headers:    headers,
			HttpClient: httpClient,
		})
	default: // GET or other methods default to GET
		method = "GET"
		inner, err = getTool.NewTool(ctx, &getTool.Config{
			ToolName:   name,
			ToolDesc:   cfg.Description,
			Headers:    headers,
			HttpClient: httpClient,
		})
	}
	if err != nil {
		return nil, err
	}
	return &httpRequestTool{InvokableTool: inner, method: method}, nil
}

// httpRequestTool keeps the configured tool name and tells the approval layer
// which requests can change remote state.
type httpRequestTool struct {
	tool.InvokableTool
	method string
}

// ApprovalRequest flags non-read-only methods.
func (t *httpRequestTool) ApprovalRequest(arguments string) (approval.Request, bool) {
	switch t.method {
	case "GET", "HEAD", "OPTIONS":
		return approval.Request{}, false
	}
	summary := "发送 " + t.method + " 请求"
	var args struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err == nil && args.URL != "" {
		summary += "：" + args.URL
	}
	return approval.Request{Summary: summary, Detail: arguments}, true
}

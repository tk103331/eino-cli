package custom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/template"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/tk103331/eino-cli/config"
)

// HTTPConfig HTTP tool configuration structure
type HTTPConfig struct {
	URL     string            `yaml:"url"`     // Request URL template
	Method  string            `yaml:"method"`  // HTTP method
	Headers map[string]string `yaml:"headers"` // Request headers
	Body    string            `yaml:"body"`    // Request body template
	Timeout int               `yaml:"timeout"` // Timeout in seconds
}

// HTTPTool HTTP tool implementation
type HTTPTool struct {
	info       *schema.ToolInfo
	config     config.Tool
	httpConfig *HTTPConfig
}

// NewHTTPTool creates HTTP tool
func NewHTTPTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	// Initialize HTTPConfig
	httpConfig := &HTTPConfig{}
	if cfg.Config != nil {
		if urlValue, exists := cfg.Config["url"]; exists {
			httpConfig.URL = urlValue.String()
		}
		if methodValue, exists := cfg.Config["method"]; exists {
			httpConfig.Method = methodValue.String()
		}
		if bodyValue, exists := cfg.Config["body"]; exists {
			httpConfig.Body = bodyValue.String()
		}
		if timeoutValue, exists := cfg.Config["timeout"]; exists {
			httpConfig.Timeout = timeoutValue.Int()
		}
		if headersValue, exists := cfg.Config["headers"]; exists && headersValue.IsMap() {
			httpConfig.Headers = make(map[string]string)
			for k, v := range headersValue.Map() {
				httpConfig.Headers[k] = v.String()
			}
		}
	}

	// Check required attributes
	if httpConfig.URL == "" {
		return nil, fmt.Errorf("http tool must configure url attribute")
	}

	// Set default values
	if httpConfig.Method == "" {
		httpConfig.Method = "GET"
	}
	if httpConfig.Timeout == 0 {
		httpConfig.Timeout = 30 // Default 30 seconds timeout
	}

	// Get description information
	desc := cfg.Description
	if desc == "" {
		desc = "HTTP tool"
	}

	// Create tool information
	toolInfo := &schema.ToolInfo{
		Name: name,
		Desc: desc,
	}

	// Add parameter information
	params := make(map[string]*schema.ParameterInfo)
	for _, param := range cfg.Params {
		// Convert string type to schema.DataType
		var dataType schema.DataType
		switch param.Type {
		case "string":
			dataType = schema.String
		case "number":
			dataType = schema.Number
		case "integer":
			dataType = schema.Integer
		case "boolean":
			dataType = schema.Boolean
		case "array":
			dataType = schema.Array
		case "object":
			dataType = schema.Object
		default:
			dataType = schema.String
		}

		params[param.Name] = &schema.ParameterInfo{
			Type: dataType,
			Desc: param.Description,
		}
	}
	toolInfo.ParamsOneOf = schema.NewParamsOneOfByParams(params)

	return &HTTPTool{
		info:       toolInfo,
		config:     cfg,
		httpConfig: httpConfig,
	}, nil
}

// Info gets tool information
func (h *HTTPTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return h.info, nil
}

// InvokableRun implements InvokableTool interface
func (h *HTTPTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// Parse parameters
	var args map[string]interface{}
	if argumentsInJSON != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
			return "", fmt.Errorf("failed to parse parameters: %v", err)
		}
	}

	// Template replacement for URL
	url, err := h.renderTemplate(h.httpConfig.URL, args)
	if err != nil {
		return "", fmt.Errorf("failed to render URL template: %v", err)
	}

	// Prepare request body
	var body io.Reader
	if h.httpConfig.Body != "" {
		bodyStr, err := h.renderTemplate(h.httpConfig.Body, args)
		if err != nil {
			return "", fmt.Errorf("failed to render request body template: %v", err)
		}
		body = strings.NewReader(bodyStr)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, h.httpConfig.Method, url, body)
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %v", err)
	}

	// Set request headers
	if h.httpConfig.Headers != nil {
		for key, value := range h.httpConfig.Headers {
			headerValue, err := h.renderTemplate(value, args)
			if err != nil {
				return "", fmt.Errorf("failed to render request header template: %v", err)
			}
			req.Header.Set(key, headerValue)
		}
	}

	// Send HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send HTTP request: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	// Check HTTP status code
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP request failed, status code: %d, response: %s", resp.StatusCode, string(respBody))
	}

	return string(respBody), nil
}

// renderTemplate renders template
func (h *HTTPTool) renderTemplate(templateStr string, args map[string]interface{}) (string, error) {
	tmpl, err := template.New("http").Parse(templateStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, args); err != nil {
		return "", err
	}

	return buf.String(), nil
}

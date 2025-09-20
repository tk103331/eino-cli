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

// HTTPConfig HTTP工具配置结构体
type HTTPConfig struct {
	URL     string            `yaml:"url"`     // 请求URL模板
	Method  string            `yaml:"method"`  // HTTP方法
	Headers map[string]string `yaml:"headers"` // 请求头
	Body    string            `yaml:"body"`    // 请求体模板
	Timeout int               `yaml:"timeout"` // 超时时间(秒)
}

// HTTPTool HTTP工具实现
type HTTPTool struct {
	info       *schema.ToolInfo
	config     config.Tool
	httpConfig *HTTPConfig
}

// NewHTTPTool 创建HTTP工具
func NewHTTPTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	// 初始化HTTPConfig
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

	// 检查必要属性
	if httpConfig.URL == "" {
		return nil, fmt.Errorf("http工具必须配置url属性")
	}

	// 设置默认值
	if httpConfig.Method == "" {
		httpConfig.Method = "GET"
	}
	if httpConfig.Timeout == 0 {
		httpConfig.Timeout = 30 // 默认30秒超时
	}

	// 获取描述信息
	desc := cfg.Description
	if desc == "" {
		desc = "HTTP工具"
	}

	// 创建工具信息
	toolInfo := &schema.ToolInfo{
		Name: name,
		Desc: desc,
	}

	// 添加参数信息
	params := make(map[string]*schema.ParameterInfo)
	for _, param := range cfg.Params {
		// 将字符串类型转换为schema.DataType
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

// Info 获取工具信息
func (h *HTTPTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return h.info, nil
}

// InvokableRun 实现InvokableTool接口
func (h *HTTPTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// 解析参数
	var args map[string]interface{}
	if argumentsInJSON != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
			return "", fmt.Errorf("解析参数失败: %v", err)
		}
	}

	// 模板替换URL
	url, err := h.renderTemplate(h.httpConfig.URL, args)
	if err != nil {
		return "", fmt.Errorf("渲染URL模板失败: %v", err)
	}

	// 准备请求体
	var body io.Reader
	if h.httpConfig.Body != "" {
		bodyStr, err := h.renderTemplate(h.httpConfig.Body, args)
		if err != nil {
			return "", fmt.Errorf("渲染请求体模板失败: %v", err)
		}
		body = strings.NewReader(bodyStr)
	}

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, h.httpConfig.Method, url, body)
	if err != nil {
		return "", fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	// 设置请求头
	if h.httpConfig.Headers != nil {
		for key, value := range h.httpConfig.Headers {
			headerValue, err := h.renderTemplate(value, args)
			if err != nil {
				return "", fmt.Errorf("渲染请求头模板失败: %v", err)
			}
			req.Header.Set(key, headerValue)
		}
	}

	// 发送HTTP请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(respBody))
	}

	return string(respBody), nil
}

// renderTemplate 渲染模板
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

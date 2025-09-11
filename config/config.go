package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// 私有全局变量保存配置
var globalConfig *Config

// Config 表示Eino CLI的配置
type Config struct {
	Agents       map[string]Agent     `yaml:"agents,omitempty"`
	Providers    map[string]Provider  `yaml:"providers,omitempty"`
	Models       map[string]Model     `yaml:"models,omitempty"`
	DefaultModel string               `yaml:"default_model,omitempty"`
	MCPServers   map[string]MCPServer `yaml:"mcp_servers,omitempty"`
	Tools        map[string]Tool      `yaml:"tools,omitempty"`
}

// Agent 表示AI代理配置
type Agent struct {
	System     string   `yaml:"system"`
	Model      string   `yaml:"model"`
	Tools      []string `yaml:"tools,omitempty"`
	MCPServers []string `yaml:"mcp_servers,omitempty"`
}

// Provider 表示AI提供商配置
type Provider struct {
	Type    string `yaml:"type"`
	BaseURL string `yaml:"base_url,omitempty"`
	APIKey  string `yaml:"api_key,omitempty"`
}

// Model 表示AI模型配置
type Model struct {
	Provider    string  `yaml:"provider"`
	Model       string  `yaml:"model"`
	MaxTokens   int     `yaml:"max_tokens,omitempty"`
	Temperature float64 `yaml:"temperature,omitempty"`
	TopP        float64 `yaml:"top_p,omitempty"`
	TopK        int     `yaml:"top_k,omitempty"`
}

// MCPServer 表示MCP服务器配置
type MCPServer struct {
	Type string `yaml:"type"`
	// for stdio
	Cmd  string            `yaml:"cmd,omitempty"`
	Args []string          `yaml:"args,omitempty"`
	Env  map[string]string `yaml:"env,omitempty"`
	// for sse & streamable-http
	URL     string            `yaml:"url,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
}

// Tool 表示工具配置
type Tool struct {
	Type        string           `yaml:"type"`
	Description string           `yaml:"description,omitempty"`
	Config      map[string]Value `yaml:"config,omitempty"`
	Params      []ToolParam      `yaml:"params,omitempty"`
}

// ToolParam 表示工具参数配置
type ToolParam struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Description string `yaml:"description,omitempty"`
	Required    bool   `yaml:"required,omitempty"`
}

// LoadConfig 从配置文件加载配置并保存到全局变量
func LoadConfig(configPath string) (*Config, error) {
	// 检查配置文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("配置文件不存在: %s", configPath)
	}

	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析YAML
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 保存到全局变量
	globalConfig = &cfg

	return &cfg, nil
}

// GetConfig 获取全局配置
func GetConfig() *Config {
	return globalConfig
}

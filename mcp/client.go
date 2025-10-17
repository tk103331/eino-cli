package mcp

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/components/tool"
	"github.com/mark3labs/mcp-go/client"
	"github.com/tk103331/eino-cli/config"
)

// Client MCP客户端结构体
type Client struct {
	mu      sync.RWMutex
	clients map[string]*client.Client
	tools   map[string]tool.InvokableTool
	config  *config.Config
}

// NewClient 创建新的MCP客户端
func NewClient(cfg *config.Config) *Client {
	return &Client{
		clients: make(map[string]*client.Client),
		tools:   make(map[string]tool.InvokableTool),
		config:  cfg,
	}
}

// Initialize 初始化MCP客户端
func (c *Client) Initialize(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 验证配置
	if err := ValidateConfig(c.config); err != nil {
		return NewMCPError("initialize", "", "", fmt.Errorf("配置验证失败: %w", err))
	}

	// 为每个配置的MCP服务器创建客户端
	for serverName, serverConfig := range c.config.MCPServers {
		client, err := c.createMCPClient(ctx, serverName, serverConfig)
		if err != nil {
			return NewMCPError("initialize", serverName, "", fmt.Errorf("创建MCP客户端失败: %w", err))
		}

		c.clients[serverName] = client
	}

	// 发现并注册所有工具
	if err := c.discoverTools(ctx); err != nil {
		return NewMCPError("initialize", "", "", fmt.Errorf("发现MCP工具失败: %w", err))
	}

	return nil
}

// GetTools 获取所有可用的MCP工具
func (c *Client) GetTools() map[string]tool.InvokableTool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tools := make(map[string]tool.InvokableTool)
	for name, tool := range c.tools {
		tools[name] = tool
	}
	return tools
}

// GetToolsForServers 获取指定MCP服务器的工具
func (c *Client) GetToolsForServers(serverNames []string) map[string]tool.InvokableTool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tools := make(map[string]tool.InvokableTool)
	for _, serverName := range serverNames {
		for toolName, tool := range c.tools {
			// 工具名格式: serverName_toolName
			if len(toolName) > len(serverName)+1 && toolName[:len(serverName)+1] == serverName+"_" {
				tools[toolName] = tool
			}
		}
	}
	return tools
}

// Close 关闭所有MCP客户端连接
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var errs []error
	for name, client := range c.clients {
		if err := client.Close(); err != nil {
			errs = append(errs, fmt.Errorf("关闭MCP客户端 %s 失败: %w", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("关闭MCP客户端时发生错误: %v", errs)
	}

	return nil
}

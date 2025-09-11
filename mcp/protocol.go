package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/tk103331/eino-cli/config"
)

// createMCPClient 根据配置创建MCP客户端
func (c *Client) createMCPClient(ctx context.Context, serverName string, serverConfig config.MCPServer) (*client.Client, error) {
	switch serverConfig.Type {
	case "stdio":
		return c.createStdioClient(ctx, serverConfig)
	case "mcp", "sse":
		return c.createSSEClient(ctx, serverConfig)
	default:
		return nil, fmt.Errorf("不支持的MCP服务器类型: %s", serverConfig.Type)
	}
}

// createStdioClient 创建STDIO类型的MCP客户端
func (c *Client) createStdioClient(ctx context.Context, serverConfig config.MCPServer) (*client.Client, error) {
	if serverConfig.Cmd == "" {
		return nil, fmt.Errorf("STDIO类型的MCP服务器必须指定cmd")
	}

	// 准备环境变量
	var env []string
	if len(serverConfig.Env) > 0 {
		for key, value := range serverConfig.Env {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	// 创建STDIO客户端
	client, err := client.NewStdioMCPClient(serverConfig.Cmd, env, serverConfig.Args...)
	if err != nil {
		return nil, fmt.Errorf("创建STDIO MCP客户端失败: %w", err)
	}

	return client, nil
}

// createSSEClient 创建SSE类型的MCP客户端
func (c *Client) createSSEClient(ctx context.Context, serverConfig config.MCPServer) (*client.Client, error) {
	if serverConfig.URL == "" {
		return nil, fmt.Errorf("SSE类型的MCP服务器必须指定URL")
	}

	// 准备客户端选项
	var options []transport.ClientOption
	if len(serverConfig.Headers) > 0 {
		options = append(options, transport.WithHeaders(serverConfig.Headers))
	}

	// 创建SSE客户端
	client, err := client.NewSSEMCPClient(serverConfig.URL, options...)
	if err != nil {
		return nil, fmt.Errorf("创建SSE MCP客户端失败: %w", err)
	}

	return client, nil
}

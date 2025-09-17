package mcp

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	mcpProtocol "github.com/mark3labs/mcp-go/mcp"
)

// discoverTools 从MCP服务器发现工具
func (c *Client) discoverTools(ctx context.Context) error {
	for serverName, mcpClient := range c.clients {
		// 检查客户端是否为空
		if mcpClient == nil {
			return fmt.Errorf("服务器 %s 的MCP客户端未初始化", serverName)
		}

		// 初始化MCP客户端连接
		initRequest := mcpProtocol.InitializeRequest{
			Params: mcpProtocol.InitializeParams{
				ProtocolVersion: "2024-11-05",
				ClientInfo: mcpProtocol.Implementation{
					Name:    "eino-cli",
					Version: "1.0.0",
				},
			},
		}

		_, err := mcpClient.Initialize(ctx, initRequest)
		if err != nil {
			return fmt.Errorf("初始化服务器 %s 的MCP客户端失败: %w", serverName, err)
		}

		// 使用eino-ext的mcp包获取工具
		mcpTools, err := mcp.GetTools(ctx, &mcp.Config{Cli: mcpClient})
		if err != nil {
			return fmt.Errorf("从服务器 %s 获取工具失败: %w", serverName, err)
		}

		// 将工具添加到工具映射中
		for _, mcpTool := range mcpTools {
			// 尝试将BaseTool转换为InvokableTool
			if invokableTool, ok := mcpTool.(tool.InvokableTool); ok {
				// 获取工具信息以获取工具名称
				info, err := mcpTool.Info(ctx)
				if err != nil {
					return fmt.Errorf("获取工具信息失败: %w", err)
				}
				// 使用 serverName_toolName 作为工具名称以避免冲突
				toolName := fmt.Sprintf("%s_%s", serverName, info.Name)
				c.tools[toolName] = invokableTool
			}
		}
	}
	return nil
}

package mcp

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/components/tool"
	"github.com/tk103331/eino-cli/config"
)

// Manager MCP管理器，负责管理所有MCP客户端和工具
type Manager struct {
	mu     sync.RWMutex
	client *Client
	config *config.Config
}

// NewManager 创建新的MCP管理器
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		client: NewClient(cfg),
		config: cfg,
	}
}

// Initialize 初始化MCP管理器
func (m *Manager) Initialize(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证配置
	if err := ValidateConfig(m.config); err != nil {
		return NewMCPError("manager_init", "", "", fmt.Errorf("MCP管理器配置验证失败: %w", err))
	}

	// 初始化客户端
	if err := m.client.Initialize(ctx); err != nil {
		return NewMCPError("manager_init", "", "", fmt.Errorf("MCP客户端初始化失败: %w", err))
	}

	return nil
}

// GetToolsForAgent 获取指定Agent的MCP工具
func (m *Manager) GetToolsForAgent(agentName string) ([]tool.InvokableTool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 检查管理器是否已初始化
	if m.client == nil {
		return nil, NewMCPError("get_tools", "", "", ErrMCPNotInitialized)
	}

	// 获取Agent的MCP服务器列表
	serverNames, err := GetAgentMCPServers(m.config, agentName)
	if err != nil {
		return nil, NewMCPError("get_tools", "", "", err)
	}

	// 如果Agent没有配置MCP服务器，返回空列表
	if len(serverNames) == 0 {
		return []tool.InvokableTool{}, nil
	}

	// 获取指定服务器的工具
	mcpTools := m.client.GetToolsForServers(serverNames)
	
	// 转换为工具列表
	tools := make([]tool.InvokableTool, 0, len(mcpTools))
	for _, mcpTool := range mcpTools {
		tools = append(tools, mcpTool)
	}

	return tools, nil
}

// GetAllTools 获取所有MCP工具
func (m *Manager) GetAllTools() map[string]tool.InvokableTool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.client.GetTools()
}

// Close 关闭MCP管理器
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.client.Close()
}

// 全局MCP管理器实例
var (
	globalManager *Manager
	managerMu     sync.RWMutex
)

// InitializeGlobalManager 初始化全局MCP管理器
func InitializeGlobalManager(ctx context.Context, cfg *config.Config) error {
	managerMu.Lock()
	defer managerMu.Unlock()

	if globalManager != nil {
		// 如果已经存在，先关闭
		globalManager.Close()
	}

	globalManager = NewManager(cfg)
	return globalManager.Initialize(ctx)
}

// GetGlobalManager 获取全局MCP管理器
func GetGlobalManager() *Manager {
	managerMu.RLock()
	defer managerMu.RUnlock()
	return globalManager
}

// CloseGlobalManager 关闭全局MCP管理器
func CloseGlobalManager() error {
	managerMu.Lock()
	defer managerMu.Unlock()

	if globalManager != nil {
		err := globalManager.Close()
		globalManager = nil
		return err
	}
	return nil
}
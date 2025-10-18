package mcp

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/components/tool"
	"github.com/tk103331/eino-cli/config"
)

// Manager MCP manager, responsible for managing all MCP clients and tools
type Manager struct {
	mu     sync.RWMutex
	client *Client
	config *config.Config
}

// NewManager creates a new MCP manager
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		client: NewClient(cfg),
		config: cfg,
	}
}

// Initialize initializes the MCP manager
func (m *Manager) Initialize(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate configuration
	if err := ValidateConfig(m.config); err != nil {
		return NewMCPError("manager_init", "", "", fmt.Errorf("MCP manager configuration validation failed: %w", err))
	}

	// Initialize client
	if err := m.client.Initialize(ctx); err != nil {
		return NewMCPError("manager_init", "", "", fmt.Errorf("MCP client initialization failed: %w", err))
	}

	return nil
}

// GetToolsForAgent gets MCP tools for specified agent
func (m *Manager) GetToolsForAgent(agentName string) ([]tool.InvokableTool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if manager is initialized
	if m.client == nil {
		return nil, NewMCPError("get_tools", "", "", ErrMCPNotInitialized)
	}

	// Get agent's MCP server list
	serverNames, err := GetAgentMCPServers(m.config, agentName)
	if err != nil {
		return nil, NewMCPError("get_tools", "", "", err)
	}

	// If agent has no MCP servers configured, return empty list
	if len(serverNames) == 0 {
		return []tool.InvokableTool{}, nil
	}

	// Get tools from specified servers
	mcpTools := m.client.GetToolsForServers(serverNames)

	// Convert to tool list
	tools := make([]tool.InvokableTool, 0, len(mcpTools))
	for _, mcpTool := range mcpTools {
		tools = append(tools, mcpTool)
	}

	return tools, nil
}

// GetAllTools gets all MCP tools
func (m *Manager) GetAllTools() map[string]tool.InvokableTool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.client.GetTools()
}

// Close closes the MCP manager
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.client.Close()
}

// Global MCP manager instance
var (
	globalManager *Manager
	managerMu     sync.RWMutex
)

// InitializeGlobalManager initializes the global MCP manager
func InitializeGlobalManager(ctx context.Context, cfg *config.Config) error {
	managerMu.Lock()
	defer managerMu.Unlock()

	if globalManager != nil {
		// If already exists, close it first
		globalManager.Close()
	}

	globalManager = NewManager(cfg)
	return globalManager.Initialize(ctx)
}

// GetGlobalManager gets the global MCP manager
func GetGlobalManager() *Manager {
	managerMu.RLock()
	defer managerMu.RUnlock()
	return globalManager
}

// CloseGlobalManager closes the global MCP manager
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

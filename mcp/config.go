package mcp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tk103331/eino-cli/config"
)

// ValidateConfig 验证MCP配置
func ValidateConfig(cfg *config.Config) error {
	if cfg == nil {
		return NewMCPError("validate", "", "", ErrInvalidConfig)
	}

	// 验证MCP服务器配置
	for serverName, serverConfig := range cfg.MCPServers {
		if err := validateServerConfig(serverName, serverConfig); err != nil {
			return err
		}
	}

	// 验证Agent的MCP服务器引用
	for agentName, agentConfig := range cfg.Agents {
		for _, serverName := range agentConfig.MCPServers {
			if _, exists := cfg.MCPServers[serverName]; !exists {
				return NewMCPError("validate", serverName, "",
					fmt.Errorf("Agent %s 引用了不存在的MCP服务器: %s", agentName, serverName))
			}
		}
	}

	return nil
}

// validateServerConfig 验证单个MCP服务器配置
func validateServerConfig(serverName string, serverConfig config.MCPServer) error {
	// 验证服务器名称
	if strings.TrimSpace(serverName) == "" {
		return NewMCPError("validate", serverName, "",
			fmt.Errorf("MCP服务器名称不能为空"))
	}

	// 验证命令或URL
	if serverConfig.Cmd == "" && serverConfig.URL == "" {
		return NewMCPError("validate", serverName, "",
			fmt.Errorf("MCP服务器必须指定cmd或url"))
	}

	// 如果同时指定了cmd和url，返回错误
	if serverConfig.Cmd != "" && serverConfig.URL != "" {
		return NewMCPError("validate", serverName, "",
			fmt.Errorf("MCP服务器不能同时指定cmd和url"))
	}

	// 验证命令路径
	if serverConfig.Cmd != "" {
		if err := validateCommand(serverName, serverConfig.Cmd); err != nil {
			return err
		}
	}

	// 验证URL格式
	if serverConfig.URL != "" {
		if err := validateURL(serverName, serverConfig.URL); err != nil {
			return err
		}
	}

	return nil
}

// validateCommand 验证命令路径
func validateCommand(serverName, command string) error {
	// 解析命令（可能包含参数）
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return NewMCPError("validate", serverName, "",
			fmt.Errorf("命令不能为空"))
	}

	cmdPath := parts[0]

	// 如果是相对路径，检查是否存在
	if !filepath.IsAbs(cmdPath) {
		// 检查PATH中是否存在该命令
		if _, err := os.Stat(cmdPath); err != nil {
			// 如果当前目录不存在，尝试在PATH中查找
			if _, pathErr := exec.LookPath(cmdPath); pathErr != nil {
				return NewMCPError("validate", serverName, "",
					fmt.Errorf("命令不存在: %s", cmdPath))
			}
		}
	} else {
		// 绝对路径，直接检查文件是否存在
		if _, err := os.Stat(cmdPath); err != nil {
			return NewMCPError("validate", serverName, "",
				fmt.Errorf("命令文件不存在: %s", cmdPath))
		}
	}

	return nil
}

// validateURL 验证URL格式
func validateURL(serverName, url string) error {
	// 简单的URL格式验证
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "ws://") && !strings.HasPrefix(url, "wss://") {
		return NewMCPError("validate", serverName, "",
			fmt.Errorf("无效的URL格式: %s，必须以http://、https://、ws://或wss://开头", url))
	}

	return nil
}

// GetServerConfig 获取指定服务器的配置
func GetServerConfig(cfg *config.Config, serverName string) (*config.MCPServer, error) {
	if cfg == nil {
		return nil, NewMCPError("get_config", serverName, "", ErrInvalidConfig)
	}

	serverConfig, exists := cfg.MCPServers[serverName]
	if !exists {
		return nil, NewMCPError("get_config", serverName, "", ErrServerNotFound)
	}

	return &serverConfig, nil
}

// GetAgentMCPServers 获取指定Agent的MCP服务器列表
func GetAgentMCPServers(cfg *config.Config, agentName string) ([]string, error) {
	if cfg == nil {
		return nil, NewMCPError("get_agent_servers", "", "", ErrInvalidConfig)
	}

	agentConfig, exists := cfg.Agents[agentName]
	if !exists {
		return nil, NewMCPError("get_agent_servers", "", "",
			fmt.Errorf("Agent配置不存在: %s", agentName))
	}

	return agentConfig.MCPServers, nil
}

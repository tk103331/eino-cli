package mcp

import (
	"errors"
	"fmt"
)

// MCP相关错误定义
var (
	// ErrMCPNotInitialized MCP管理器未初始化
	ErrMCPNotInitialized = errors.New("MCP管理器未初始化")
	
	// ErrServerNotFound MCP服务器未找到
	ErrServerNotFound = errors.New("MCP服务器未找到")
	
	// ErrToolNotFound MCP工具未找到
	ErrToolNotFound = errors.New("MCP工具未找到")
	
	// ErrInvalidConfig 无效的MCP配置
	ErrInvalidConfig = errors.New("无效的MCP配置")
	
	// ErrConnectionFailed MCP连接失败
	ErrConnectionFailed = errors.New("MCP连接失败")
)

// MCPError MCP错误包装器
type MCPError struct {
	Op     string // 操作名称
	Server string // 服务器名称
	Tool   string // 工具名称
	Err    error  // 原始错误
}

// Error 实现error接口
func (e *MCPError) Error() string {
	if e.Server != "" && e.Tool != "" {
		return fmt.Sprintf("MCP错误 [%s] 服务器:%s 工具:%s - %v", e.Op, e.Server, e.Tool, e.Err)
	} else if e.Server != "" {
		return fmt.Sprintf("MCP错误 [%s] 服务器:%s - %v", e.Op, e.Server, e.Err)
	} else {
		return fmt.Sprintf("MCP错误 [%s] - %v", e.Op, e.Err)
	}
}

// Unwrap 返回原始错误
func (e *MCPError) Unwrap() error {
	return e.Err
}

// NewMCPError 创建新的MCP错误
func NewMCPError(op, server, tool string, err error) *MCPError {
	return &MCPError{
		Op:     op,
		Server: server,
		Tool:   tool,
		Err:    err,
	}
}

// IsConnectionError 检查是否为连接错误
func IsConnectionError(err error) bool {
	if mcpErr, ok := err.(*MCPError); ok {
		return errors.Is(mcpErr.Err, ErrConnectionFailed)
	}
	return errors.Is(err, ErrConnectionFailed)
}

// IsConfigError 检查是否为配置错误
func IsConfigError(err error) bool {
	if mcpErr, ok := err.(*MCPError); ok {
		return errors.Is(mcpErr.Err, ErrInvalidConfig)
	}
	return errors.Is(err, ErrInvalidConfig)
}
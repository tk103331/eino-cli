package agent

import (
	"context"
)

// StreamChunk 表示流式输出的数据块
type StreamChunk struct {
	Content string
	Type    string // "content", "tool_start", "tool_end", "error"
	Tool    string // 工具名称（仅用于工具相关消息）
}

// Agent 定义了CLI中使用的代理接口
type Agent interface {
	// Run 运行代理
	Run(prompt string) error
	// Chat 进行对话，返回响应内容
	Chat(ctx context.Context, prompt string) (string, error)
	// ChatWithCallback 进行对话，支持工具调用回调
	ChatWithCallback(ctx context.Context, prompt string, callback func(interface{})) (string, error)
	// ChatStream 进行流式对话，通过chunk回调处理流式输出
	ChatStream(ctx context.Context, prompt string, chunkCallback func(*StreamChunk), toolCallback func(interface{})) error
}

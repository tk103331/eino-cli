package agent

import (
	"context"
)

// StreamChunk represents a data chunk for streaming output
type StreamChunk struct {
	Content string
	Type    string // "content", "tool_start", "tool_end", "error"
	Tool    string // Tool name (only used for tool-related messages)
}

// Agent defines the agent interface used in the CLI
type Agent interface {
	// Run runs the agent
	Run(prompt string) error
	// Chat performs conversation, returns response content
	Chat(ctx context.Context, prompt string) (string, error)
	// ChatWithCallback performs conversation, supporting tool call callbacks
	ChatWithCallback(ctx context.Context, prompt string, callback func(interface{})) (string, error)
	// ChatStream performs streaming conversation, handles streaming output through chunk callback
	ChatStream(ctx context.Context, prompt string, chunkCallback func(*StreamChunk), toolCallback func(interface{})) error
}

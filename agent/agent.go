package agent

import (
	"context"
	"github.com/cloudwego/eino/flow/agent"
	"github.com/cloudwego/eino/schema"
)

// Agent 定义了CLI中使用的代理接口
type Agent interface {
	// Run 运行代理
	Run(prompt string) error
	// Chat 进行对话，返回响应内容
	Chat(ctx context.Context, prompt string) (string, error)
	// Generate 使用Agent生成响应，支持传入选项
	Generate(ctx context.Context, messages []*schema.Message, opts ...agent.AgentOption) (*schema.Message, error)
}

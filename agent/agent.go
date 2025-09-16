package agent

import "context"

// Agent 定义了CLI中使用的代理接口
type Agent interface {
	// Run 运行代理
	Run(prompt string) error
	// Chat 进行对话，返回响应内容
	Chat(ctx context.Context, prompt string) (string, error)
}

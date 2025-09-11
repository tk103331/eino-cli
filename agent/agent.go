package agent

// Agent 定义了CLI中使用的代理接口
type Agent interface {
	// Run 运行代理
	Run(prompt string) error
}
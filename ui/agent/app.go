package agent

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cloudwego/eino/schema"
	"github.com/tk103331/eino-cli/agent"
	"github.com/tk103331/eino-cli/config"
)

// AgentApp Agent应用结构
type AgentApp struct {
	agentName string
	program   *tea.Program
	model     *ViewModel
	agent     agent.Agent
	ctx       context.Context
}

// NewAgentApp 创建新的Agent应用
func NewAgentApp(agentName string) (*AgentApp, error) {
	cfg := config.GetConfig()
	if cfg == nil {
		return nil, fmt.Errorf("全局配置未初始化")
	}

	// 检查Agent配置是否存在
	_, ok := cfg.Agents[agentName]
	if !ok {
		return nil, fmt.Errorf("Agent配置不存在: %s", agentName)
	}

	// 创建Agent工厂
	factory := agent.NewFactory(cfg)

	// 创建Agent实例
	agentInstance, err := factory.CreateAgent(agentName)
	if err != nil {
		return nil, fmt.Errorf("创建Agent失败: %v", err)
	}

	app := &AgentApp{
		agentName: agentName,
		agent:     agentInstance,
		ctx:       context.Background(),
	}

	// 创建Agent模型，传入发送消息的回调函数
	agentModel := NewViewModel(app.sendMessage)
	app.model = agentModel

	// 创建Bubble Tea程序
	app.program = tea.NewProgram(*agentModel, tea.WithAltScreen())

	return app, nil
}

// Run 运行Agent应用
func (app *AgentApp) Run() error {
	_, err := app.program.Run()
	return err
}

// sendMessage 发送消息给AI
func (app *AgentApp) sendMessage(message string) error {
	// 获取Agent配置
	cfg := config.GetConfig()
	agentConfig := cfg.Agents[app.agentName]

	// 构建消息列表
	var messages []*schema.Message

	// 添加系统消息（如果有）
	if agentConfig.System != "" {
		messages = append(messages, schema.SystemMessage(agentConfig.System))
	}

	// 添加用户消息
	messages = append(messages, schema.UserMessage(message))

	// 在goroutine中处理对话，避免阻塞UI
	go app.processConversation(messages)

	return nil
}

// processConversation 处理对话（使用流式输出）
func (app *AgentApp) processConversation(messages []*schema.Message) {
	// 获取最后一条用户消息作为prompt
	var prompt string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == schema.User {
			prompt = messages[i].Content
			break
		}
	}

	if prompt == "" {
		app.program.Send(ErrorMsg("未找到用户消息"))
		return
	}

	// 创建工具调用回调函数
	toolCallback := func(msg interface{}) {
		switch v := msg.(type) {
		case struct {
			Name      string
			Arguments string
		}:
			// 只发送真正的工具调用，过滤内部组件
			if !isInternalComponent(v.Name) {
				app.program.Send(ToolStartMsg{
					Name:      v.Name,
					Arguments: v.Arguments,
				})
			}
		case struct {
			Name   string
			Result string
		}:
			// 只发送真正的工具调用结果，过滤内部组件
			if !isInternalComponent(v.Name) {
				app.program.Send(ToolEndMsg{
					Name:   v.Name,
					Result: v.Result,
				})
			}
		default:
			if errMsg, ok := msg.(string); ok {
				app.program.Send(ErrorMsg(errMsg))
			}
		}
	}

	// 创建流式内容回调函数
	chunkCallback := func(chunk *agent.StreamChunk) {
		switch chunk.Type {
		case "content":
			if chunk.Content != "" {
				app.program.Send(StreamChunkMsg(chunk.Content))
			} else {
				app.program.Send(StreamEndMsg{})
			}
		case "error":
			app.program.Send(ErrorMsg(chunk.Content))
		}
	}

	// 使用Agent的ChatStream方法进行流式对话
	err := app.agent.ChatStream(app.ctx, prompt, chunkCallback, toolCallback)
	if err != nil {
		app.program.Send(ErrorMsg(fmt.Sprintf("AI响应错误: %v", err)))
	}
}

// isInternalComponent 判断是否为内部组件，不应显示给用户
func isInternalComponent(name string) bool {
	internalComponents := []string{
		"ChatModel",
		"Tools",
		"ReActAgent",
	}

	for _, component := range internalComponents {
		if name == component {
			return true
		}
	}
	return false
}

// Stop 停止Agent应用
func (app *AgentApp) Stop() {
	if app.program != nil {
		app.program.Quit()
	}
}

package agent

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent"
	"github.com/cloudwego/eino/schema"
	agentpkg "github.com/tk103331/eino-cli/agent"
	"github.com/tk103331/eino-cli/config"
)

// AgentApp represents an interactive Agent application structure
type AgentApp struct {
	agentFactory *agentpkg.Factory
	agentName    string
	program      *tea.Program
	model        *ViewModel
	agent        agentpkg.Agent
	handler      callbacks.Handler
}

// NewAgentApp 创建新的Agent交互应用
func NewAgentApp(agentName string) *AgentApp {
	cfg := config.GetConfig()
	factory := agentpkg.NewFactory(cfg)

	app := &AgentApp{
		agentFactory: factory,
		agentName:    agentName,
	}

	// 创建Agent交互模型，传入发送消息的回调函数
	agentModel := NewViewModel(app.sendMessage)
	app.model = &agentModel

	// 创建callback handler来处理agent输出
	app.handler = callbacks.NewHandlerBuilder().
		OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
			// 发送开始消息到UI
			app.program.Send(StepStartMsg(info.Name))
			app.program.Send(StreamChunkMsg(fmt.Sprintf("🚀 开始执行: %s\n", info.Name)))
			return ctx
		}).
		OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
			// 发送完成消息到UI
			app.program.Send(StepEndMsg(info.Name))
			app.program.Send(StreamChunkMsg(fmt.Sprintf("✅ 完成执行: %s\n", info.Name)))
			return ctx
		}).
		OnErrorFn(func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			// 发送错误消息到UI
			app.program.Send(ErrorMsg(fmt.Sprintf("❌ 执行错误 %s: %v", info.Name, err)))
			return ctx
		}).
		Build()

	// 创建Bubble Tea程序
	app.program = tea.NewProgram(agentModel, tea.WithAltScreen())

	return app
}

// Run 运行Agent交互应用
func (app *AgentApp) Run() error {
	_, err := app.program.Run()
	return err
}

// sendMessage 发送消息给Agent
func (app *AgentApp) sendMessage(message string) error {
	// 创建Agent实例（如果还没有创建）
	if app.agent == nil {
		agentInstance, err := app.agentFactory.CreateAgent(app.agentName)
		if err != nil {
			// 发送错误消息到UI
			app.program.Send(ErrorMsg(fmt.Sprintf("创建Agent失败: %v", err)))
			return err
		}
		app.agent = agentInstance
	}

	// 在后台运行Agent并获取响应
	go func() {
		ctx := context.Background()

		// 创建消息
		messages := []*schema.Message{
			schema.UserMessage(message),
		}

		// 调用Agent的Generate方法获取响应，传入callbacks
		response, err := app.agent.Generate(ctx, messages, agent.WithComposeOptions(compose.WithCallbacks(app.handler)))
		if err != nil {
			app.program.Send(ErrorMsg(fmt.Sprintf("Agent响应错误: %v", err)))
			return
		}

		// 发送响应到UI
		app.program.Send(ResponseMsg(response.Content))
	}()

	return nil
}

// Stop 停止Agent交互应用
func (app *AgentApp) Stop() {
	if app.program != nil {
		app.program.Quit()
	}
}

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/tk103331/eino-cli/agent"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/models"
	"github.com/tk103331/eino-cli/tools"
)

// AgentApp Agent应用结构
type AgentApp struct {
	agentName string
	program   *tea.Program
	model     *ViewModel
	agent     agent.Agent
	ctx       context.Context
}

// ChatApp 聊天应用结构（从chat功能合并而来）
type ChatApp struct {
	modelFactory *models.Factory
	agentFactory *agent.Factory
	modelName    string
	tools        []string
	system       string
	program      *tea.Program
	model        *ViewModel
	chatModel    model.ToolCallingChatModel
	reactAgent   agent.Agent
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

// NewChatApp 创建新的聊天应用（从chat功能合并而来）
func NewChatApp(modelName string, tools []string, system string) *ChatApp {
	cfg := config.GetConfig()
	factory := models.NewFactory(cfg)
	agentFactory := agent.NewFactory(cfg)

	app := &ChatApp{
		modelFactory: factory,
		agentFactory: agentFactory,
		modelName:    modelName,
		tools:        tools,
		system:       system,
	}

	// 创建聊天模型，传入发送消息的回调函数
	chatModel := NewViewModel(app.sendMessage)
	app.model = chatModel

	// 创建Bubble Tea程序
	app.program = tea.NewProgram(*chatModel, tea.WithAltScreen())

	return app
}

// Run 运行Agent应用
func (app *AgentApp) Run() error {
	_, err := app.program.Run()
	return err
}

// Run 运行Chat应用（从chat功能合并而来）
func (app *ChatApp) Run() error {
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

// sendMessage 发送消息给AI模型（ChatApp用）
func (app *ChatApp) sendMessage(message string) error {
	// 如果有工具配置，使用ReactAgent的ChatWithCallback方法
	if len(app.tools) > 0 {
		return app.sendMessageWithAgent(message)
	}

	// 否则使用原有的模型直接调用方式
	return app.sendMessageWithModel(message)
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

// sendMessageWithAgent 使用ReactAgent发送消息，支持工具调用回调（ChatApp用）
func (app *ChatApp) sendMessageWithAgent(message string) error {
	// 创建临时Agent配置
	if app.reactAgent == nil {
		agentConfig := config.Agent{
			System: app.system,
			Model:  app.modelName,
			Tools:  app.tools,
		}

		// 创建ReactAgent实例
		reactAgent := agent.NewReactAgent("temp_chat_agent", &agentConfig)
		if err := reactAgent.Init(); err != nil {
			app.program.Send(ErrorMsg(fmt.Sprintf("初始化Agent失败: %v", err)))
			return err
		}
		app.reactAgent = reactAgent
	}

	// 在后台运行Agent并获取响应
	go func() {
		ctx := context.Background()

		// 创建工具调用回调函数
		callback := func(data interface{}) {
			switch v := data.(type) {
			case string:
				// 流式内容
				app.program.Send(StreamChunkMsg(v))
			case map[string]interface{}:
				// 工具调用信息
				if toolType, ok := v["type"].(string); ok {
					switch toolType {
					case "tool_start":
						if toolName, ok := v["tool_name"].(string); ok {
							var inputStr string
							if input := v["input"]; input != nil {
								if inputBytes, err := json.Marshal(input); err == nil {
									inputStr = string(inputBytes)
								}
							}
							app.program.Send(ToolStartMsg{
								Name:      toolName,
								Arguments: inputStr,
							})
						}
					case "tool_end":
						if toolName, ok := v["tool_name"].(string); ok {
							var outputStr string
							if output := v["output"]; output != nil {
								if outputBytes, err := json.Marshal(output); err == nil {
									outputStr = string(outputBytes)
								}
							}
							app.program.Send(ToolEndMsg{
								Name:   toolName,
								Result: outputStr,
							})
						}
					case "tool_error":
						if toolName, ok := v["tool_name"].(string); ok {
							if errorMsg, ok := v["error"].(string); ok {
								app.program.Send(ErrorMsg(fmt.Sprintf("工具 %s 执行错误: %s", toolName, errorMsg)))
							}
						}
					}
				}
			}
		}

		// 使用Agent的ChatWithCallback方法生成响应
		response, err := app.reactAgent.ChatWithCallback(ctx, message, callback)
		if err != nil {
			app.program.Send(ErrorMsg(fmt.Sprintf("AI响应错误: %v", err)))
			return
		}

		// 发送完整响应
		if response != "" {
			app.program.Send(ResponseMsg(response))
		}
	}()

	return nil
}

// sendMessageWithModel 使用原有的模型直接调用方式发送消息（ChatApp用）
func (app *ChatApp) sendMessageWithModel(message string) error {
	// 创建模型实例（如果还没有创建）
	if app.chatModel == nil {
		ctx := context.Background()
		chatModel, err := app.modelFactory.CreateChatModel(ctx, app.modelName)
		if err != nil {
			// 发送错误消息到UI
			app.program.Send(ErrorMsg(fmt.Sprintf("创建Model失败: %v", err)))
			return err
		}

		// 如果指定了工具，则使用WithTools方法加载工具
		if len(app.tools) > 0 {
			toolInstances, err := app.createTools()
			if err != nil {
				app.program.Send(ErrorMsg(fmt.Sprintf("创建工具失败: %v", err)))
				return err
			}

			// 将 InvokableTool 转换为 ToolInfo
			var toolInfos []*schema.ToolInfo
			for _, tool := range toolInstances {
				info, err := tool.Info(context.Background())
				if err != nil {
					app.program.Send(ErrorMsg(fmt.Sprintf("获取工具信息失败: %v", err)))
					return err
				}
				toolInfos = append(toolInfos, info)
			}

			chatModel, err = chatModel.WithTools(toolInfos)
			if err != nil {
				app.program.Send(ErrorMsg(fmt.Sprintf("设置工具失败: %v", err)))
				return err
			}
		}

		app.chatModel = chatModel
	}

	// 在后台运行模型并获取流式响应
	go func() {
		ctx := context.Background()

		// 创建消息列表，包含可选的system提示
		var messages []*schema.Message
		if app.system != "" {
			messages = append(messages, schema.SystemMessage(app.system))
		}
		messages = append(messages, schema.UserMessage(message))

		// 开始对话循环，处理工具调用
		app.processConversation(ctx, messages)
	}()

	return nil
}

// processConversation 处理对话循环，包括工具调用（ChatApp用）
func (app *ChatApp) processConversation(ctx context.Context, messages []*schema.Message) {
	maxIterations := 10 // 防止无限循环
	iteration := 0

	for iteration < maxIterations {
		iteration++

		// 调用Model的Stream方法获取流式响应
		streamReader, err := app.chatModel.Stream(ctx, messages)
		if err != nil {
			app.program.Send(ErrorMsg(fmt.Sprintf("AI响应错误: %v", err)))
			return
		}

		// 处理流式响应
		var fullContent string
		var assistantMessage *schema.Message
		var allToolCalls []schema.ToolCall

		for {
			chunk, err := streamReader.Recv()
			if err != nil {
				// 流结束或出错
				if err.Error() != "EOF" && err.Error() != "io: read/write on closed pipe" {
					app.program.Send(ErrorMsg(fmt.Sprintf("流式响应错误: %v", err)))
					streamReader.Close()
					return
				}
				break
			}

			// 累积内容并发送增量更新
			fullContent += chunk.Content
			app.program.Send(StreamChunkMsg(chunk.Content))

			// 检查当前chunk是否包含工具调用
			if len(chunk.ToolCalls) > 0 {
				// 累积所有工具调用
				allToolCalls = append(allToolCalls, chunk.ToolCalls...)
			}

			// 保存完整的助手消息（包含可能的工具调用）
			assistantMessage = chunk
		}
		streamReader.Close()

		// 如果累积了工具调用，将它们合并到最终的助手消息中
		if len(allToolCalls) > 0 && assistantMessage != nil {
			assistantMessage.ToolCalls = allToolCalls
		}

		// 检查是否有工具调用
		if assistantMessage != nil && len(assistantMessage.ToolCalls) > 0 {
			// 发送完整响应到UI（如果有内容）
			if fullContent != "" {
				app.program.Send(ResponseMsg(fullContent))
			}

			// 将助手消息添加到消息历史
			messages = append(messages, assistantMessage)

			// 执行工具调用
			toolResults, err := app.executeToolCalls(ctx, assistantMessage.ToolCalls)
			if err != nil {
				app.program.Send(ErrorMsg(fmt.Sprintf("工具执行错误: %v", err)))
				return
			}

			// 将工具结果添加到消息历史
			messages = append(messages, toolResults...)

			// 继续下一轮对话
			continue
		} else {
			// 没有工具调用，发送最终响应并结束
			if fullContent != "" {
				app.program.Send(ResponseMsg(fullContent))
			}
			break
		}
	}

	if iteration >= maxIterations {
		app.program.Send(ErrorMsg("达到最大迭代次数，停止对话"))
	}
}

// executeToolCalls 执行工具调用（ChatApp用）
func (app *ChatApp) executeToolCalls(ctx context.Context, toolCalls []schema.ToolCall) ([]*schema.Message, error) {
	// 创建工具实例映射
	toolInstances, err := app.createTools()
	if err != nil {
		return nil, fmt.Errorf("创建工具实例失败: %v", err)
	}

	// 创建工具名称到实例的映射
	toolMap := make(map[string]tool.InvokableTool)
	for _, toolInstance := range toolInstances {
		info, err := toolInstance.Info(ctx)
		if err != nil {
			return nil, fmt.Errorf("获取工具信息失败: %v", err)
		}
		toolMap[info.Name] = toolInstance
	}

	var toolMessages []*schema.Message

	// 执行每个工具调用
	for _, toolCall := range toolCalls {
		toolName := toolCall.Function.Name
		arguments := toolCall.Function.Arguments
		if toolName == "" {
			continue
		}
		// 查找工具实例
		toolInstance, exists := toolMap[toolName]
		if !exists {
			// 工具不存在，返回错误消息
			errorMsg := fmt.Sprintf("工具 '%s' 不存在", toolName)
			toolMessage := schema.ToolMessage(errorMsg, toolCall.ID, schema.WithToolName(toolName))
			toolMessages = append(toolMessages, toolMessage)

			// 显示工具不存在错误
			app.program.Send(StreamChunkMsg(fmt.Sprintf("❌ 工具 '%s' 不存在\n", toolName)))
			continue
		}

		// 显示工具调用信息
		app.program.Send(StreamChunkMsg(fmt.Sprintf("\n🔧 调用工具: %s\n参数: %s\n", toolName, arguments)))

		// 执行工具
		result, err := toolInstance.InvokableRun(ctx, arguments)
		if err != nil {
			// 工具执行失败，返回错误消息
			errorMsg := fmt.Sprintf("工具执行失败: %v", err)
			toolMessage := schema.ToolMessage(errorMsg, toolCall.ID, schema.WithToolName(toolName))
			toolMessages = append(toolMessages, toolMessage)

			// 显示工具执行错误
			app.program.Send(StreamChunkMsg(fmt.Sprintf("❌ 工具执行失败: %v\n", err)))
		} else {
			// 工具执行成功，返回结果
			toolMessage := schema.ToolMessage(result, toolCall.ID, schema.WithToolName(toolName))
			toolMessages = append(toolMessages, toolMessage)

			// 显示工具执行结果（限制长度以避免UI过于冗长）
			displayResult := result
			if len(result) > 500 {
				displayResult = result[:500] + "...(结果已截断)"
			}
			app.program.Send(StreamChunkMsg(fmt.Sprintf("✅ 工具执行结果: %s\n", displayResult)))
		}
	}

	return toolMessages, nil
}

// createTools 创建工具实例（ChatApp用）
func (app *ChatApp) createTools() ([]tool.InvokableTool, error) {
	cfg := config.GetConfig()
	if cfg == nil {
		return nil, fmt.Errorf("全局配置未初始化")
	}

	var toolInstances []tool.InvokableTool
	for _, toolName := range app.tools {
		// 获取工具配置
		toolCfg, ok := cfg.Tools[toolName]
		if !ok {
			return nil, fmt.Errorf("工具配置不存在: %s", toolName)
		}

		// 创建工具实例
		toolInstance, err := tools.CreateTool(toolName, toolCfg)
		if err != nil {
			return nil, fmt.Errorf("创建工具 %s 失败: %v", toolName, err)
		}

		toolInstances = append(toolInstances, toolInstance)
	}

	return toolInstances, nil
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

// Stop 停止Chat应用（从chat功能合并而来）
func (app *ChatApp) Stop() {
	if app.program != nil {
		app.program.Quit()
	}
}

package chat

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/models"
	"github.com/tk103331/eino-cli/tools"
)

// ChatApp 聊天应用结构
type ChatApp struct {
	modelFactory *models.Factory
	modelName    string
	tools        []string
	program      *tea.Program
	model        *ViewModel
	chatModel    model.ToolCallingChatModel
}

// NewChatApp 创建新的聊天应用
func NewChatApp(modelName string, tools []string) *ChatApp {
	cfg := config.GetConfig()
	factory := models.NewFactory(cfg)

	app := &ChatApp{
		modelFactory: factory,
		modelName:    modelName,
		tools:        tools,
	}

	// 创建聊天模型，传入发送消息的回调函数
	chatModel := NewViewModel(app.sendMessage)
	app.model = &chatModel

	// 创建Bubble Tea程序
	app.program = tea.NewProgram(chatModel, tea.WithAltScreen())

	return app
}

// Run 运行聊天应用
func (app *ChatApp) Run() error {
	_, err := app.program.Run()
	return err
}

// sendMessage 发送消息给AI模型
func (app *ChatApp) sendMessage(message string) error {
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

		// 创建用户消息
		messages := []*schema.Message{
			schema.UserMessage(message),
		}

		// 开始对话循环，处理工具调用
		app.processConversation(ctx, messages)
	}()

	return nil
}

// processConversation 处理对话循环，包括工具调用
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
			
			// 保存完整的助手消息（包含可能的工具调用）
			assistantMessage = chunk
		}
		streamReader.Close()

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

// executeToolCalls 执行工具调用
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

		// 查找工具实例
		toolInstance, exists := toolMap[toolName]
		if !exists {
			// 工具不存在，返回错误消息
			errorMsg := fmt.Sprintf("工具 '%s' 不存在", toolName)
			toolMessage := schema.ToolMessage(errorMsg, toolCall.ID)
			toolMessage.ToolName = toolName
			toolMessages = append(toolMessages, toolMessage)
			continue
		}

		// 显示工具调用信息
		app.program.Send(StreamChunkMsg(fmt.Sprintf("\n🔧 调用工具: %s\n参数: %s\n", toolName, arguments)))

		// 执行工具
		result, err := toolInstance.InvokableRun(ctx, arguments)
		if err != nil {
			// 工具执行失败，返回错误消息
			errorMsg := fmt.Sprintf("工具执行失败: %v", err)
			toolMessage := schema.ToolMessage(errorMsg, toolCall.ID)
			toolMessage.ToolName = toolName
			toolMessages = append(toolMessages, toolMessage)
			
			// 显示工具执行错误
			app.program.Send(StreamChunkMsg(fmt.Sprintf("❌ 工具执行失败: %v\n", err)))
		} else {
			// 工具执行成功，返回结果
			toolMessage := schema.ToolMessage(result, toolCall.ID)
			toolMessage.ToolName = toolName
			toolMessages = append(toolMessages, toolMessage)
			
			// 显示工具执行结果
			app.program.Send(StreamChunkMsg(fmt.Sprintf("✅ 工具执行结果: %s\n", result)))
		}
	}

	return toolMessages, nil
}

// createTools 创建工具实例
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

// Stop 停止聊天应用
func (app *ChatApp) Stop() {
	if app.program != nil {
		app.program.Quit()
	}
}

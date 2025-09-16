package ui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/models"
)

// ChatApp 聊天应用结构
type ChatApp struct {
	modelFactory *models.Factory
	modelName    string
	program      *tea.Program
	model        *ChatModel
	chatModel    model.ToolCallingChatModel
}

// NewChatApp 创建新的聊天应用
func NewChatApp(modelName string) *ChatApp {
	cfg := config.GetConfig()
	factory := models.NewFactory(cfg)

	app := &ChatApp{
		modelFactory: factory,
		modelName:    modelName,
	}

	// 创建聊天模型，传入发送消息的回调函数
	chatModel := NewChatModel(app.sendMessage)
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
		app.chatModel = chatModel
	}

	// 在后台运行模型并获取流式响应
	go func() {
		ctx := context.Background()

		// 创建用户消息
		messages := []*schema.Message{
			schema.UserMessage(message),
		}

		// 调用Model的Stream方法获取流式响应
		streamReader, err := app.chatModel.Stream(ctx, messages)
		if err != nil {
			app.program.Send(ErrorMsg(fmt.Sprintf("AI响应错误: %v", err)))
			return
		}
		defer streamReader.Close()

		// 处理流式响应
		var fullContent string
		for {
			chunk, err := streamReader.Recv()
			if err != nil {
				// 流结束或出错
				if fullContent != "" {
					// 发送完整响应到UI
					app.program.Send(ResponseMsg(fullContent))
				}
				if err.Error() != "EOF" && err.Error() != "io: read/write on closed pipe" {
					app.program.Send(ErrorMsg(fmt.Sprintf("流式响应错误: %v", err)))
				}
				break
			}
			
			// 累积内容并发送增量更新
			fullContent += chunk.Content
			app.program.Send(StreamChunkMsg(chunk.Content))
		}
	}()

	return nil
}

// Stop 停止聊天应用
func (app *ChatApp) Stop() {
	if app.program != nil {
		app.program.Quit()
	}
}

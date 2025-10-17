package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/mcp"
	"github.com/tk103331/eino-cli/models"
)

// ReactAgent 实现了使用cloudwego/eino库的React模式的Agent
type ReactAgent struct {
	config    *config.Agent
	agent     *react.Agent
	ctx       context.Context
	agentName string
}

// ToolCallCallback 自定义回调处理器，用于捕获工具调用信息
type ToolCallCallback struct {
	callback func(interface{})
}

// OnStart 节点开始时的回调
func (t *ToolCallCallback) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	if t.callback != nil && info.Name != "" {
		// 发送工具开始调用信息
		t.callback(struct {
			Name      string
			Arguments string
		}{
			Name:      info.Name,
			Arguments: fmt.Sprintf("%v", input),
		})
	}
	return ctx
}

// OnEnd 节点结束时的回调
func (t *ToolCallCallback) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	if t.callback != nil && info.Name != "" {
		// 发送工具执行完成信息
		t.callback(struct {
			Name   string
			Result string
		}{
			Name:   info.Name,
			Result: fmt.Sprintf("%v", output),
		})
	}
	return ctx
}

// OnError 节点出错时的回调
func (t *ToolCallCallback) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	if t.callback != nil && info.Name != "" {
		// 任何有名称的节点错误都发送回调
		t.callback(fmt.Sprintf("工具 %s 执行错误: %v", info.Name, err))
	}
	return ctx
}

// OnStartWithStreamInput 流式输入开始时的回调
func (t *ToolCallCallback) OnStartWithStreamInput(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
	return ctx
}

// OnEndWithStreamOutput 流式输出结束时的回调
func (t *ToolCallCallback) OnEndWithStreamOutput(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
	return ctx
}

// NewReactAgent 创建一个新的ReactAgent
func NewReactAgent(agentName string, cfg *config.Agent) *ReactAgent {
	return &ReactAgent{
		config:    cfg,
		ctx:       context.Background(),
		agentName: agentName,
	}
}

// Init 初始化Agent
func (r *ReactAgent) Init() error {
	// 创建模型
	model, err := r.createModel()
	if err != nil {
		return fmt.Errorf("创建模型失败: %w", err)
	}

	// 创建工具配置
	toolsConfig, err := r.createToolsConfig()
	if err != nil {
		return fmt.Errorf("创建工具配置失败: %w", err)
	}

	// 创建Agent配置
	agentConfig := &react.AgentConfig{
		ToolCallingModel: model,
		ToolsConfig:      toolsConfig,
	}

	// 创建Agent
	agent, err := react.NewAgent(r.ctx, agentConfig)
	if err != nil {
		return fmt.Errorf("创建Agent失败: %w", err)
	}

	// 保存agent实例
	r.agent = agent
	return nil
}

// Run 运行Agent
func (r *ReactAgent) Run(prompt string) error {
	if r.agent == nil {
		if err := r.Init(); err != nil {
			return err
		}
	}

	// 创建用户消息
	messages := []*schema.Message{
		schema.SystemMessage(r.config.System),
		schema.UserMessage(prompt),
	}

	// 使用react.Generate方法生成响应
	response, err := r.agent.Generate(r.ctx, messages)
	if err != nil {
		return fmt.Errorf("运行Agent失败: %w", err)
	}

	if response.Content != "" {
		fmt.Print(response.Content)
	}
	fmt.Println()
	return nil
}

// Chat 进行对话，返回响应内容
func (r *ReactAgent) Chat(ctx context.Context, prompt string) (string, error) {
	if r.agent == nil {
		if err := r.Init(); err != nil {
			return "", err
		}
	}

	// 创建消息
	messages := []*schema.Message{
		schema.SystemMessage(r.config.System),
		schema.UserMessage(prompt),
	}

	// 使用Generate方法进行同步调用
	response, err := r.agent.Generate(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("Chat失败: %w", err)
	}

	return response.Content, nil
}

// ChatWithCallback 进行对话，支持流式输出和回调
func (r *ReactAgent) ChatWithCallback(ctx context.Context, prompt string, callback func(interface{})) (string, error) {
	if r.agent == nil {
		if err := r.Init(); err != nil {
			return "", err
		}
	}

	// 创建消息
	messages := []*schema.Message{
		schema.SystemMessage(r.config.System),
		schema.UserMessage(prompt),
	}

	// 如果没有回调函数，直接使用Generate方法
	if callback == nil {
		response, err := r.agent.Generate(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("Chat失败: %w", err)
		}
		return response.Content, nil
	}

	// 创建工具调用回调处理器
	toolCallback := &ToolCallCallback{callback: callback}

	// 使用Stream方法进行流式调用，并通过agent.WithComposeOptions添加回调处理器
	sr, err := r.agent.Stream(ctx, messages, agent.WithComposeOptions(compose.WithCallbacks(toolCallback)))
	if err != nil {
		return "", fmt.Errorf("Stream失败: %w", err)
	}
	defer sr.Close()

	var result strings.Builder
	for {
		msg, err := sr.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				// 流结束
				break
			}
			return "", fmt.Errorf("接收流消息失败: %w", err)
		}

		// 发送回调（用于显示消息内容）
		if callback != nil && msg.Content != "" {
			callback(msg.Content)
		}

		// 累积结果
		result.WriteString(msg.Content)
	}

	return result.String(), nil
}

// ChatStream 进行流式对话，通过chunk回调处理流式输出
func (r *ReactAgent) ChatStream(ctx context.Context, prompt string, chunkCallback func(*StreamChunk), toolCallback func(interface{})) error {
	if r.agent == nil {
		if err := r.Init(); err != nil {
			return err
		}
	}

	// 创建消息
	messages := []*schema.Message{
		schema.SystemMessage(r.config.System),
		schema.UserMessage(prompt),
	}

	// 创建工具调用回调处理器
	var toolCallCallback *ToolCallCallback
	if toolCallback != nil {
		toolCallCallback = &ToolCallCallback{callback: toolCallback}
	}

	// 使用Stream方法进行流式调用
	var sr *schema.StreamReader[*schema.Message]
	var err error

	if toolCallCallback != nil {
		sr, err = r.agent.Stream(ctx, messages, agent.WithComposeOptions(compose.WithCallbacks(toolCallCallback)))
	} else {
		sr, err = r.agent.Stream(ctx, messages)
	}
	if err != nil {
		if chunkCallback != nil {
			chunkCallback(&StreamChunk{
				Type:    "error",
				Content: fmt.Sprintf("Stream失败: %v", err),
			})
		}
		return fmt.Errorf("Stream失败: %w", err)
	}
	defer sr.Close()

	// 读取流式响应
	for {
		msg, err := sr.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				// 流结束，发送结束标记
				if chunkCallback != nil {
					chunkCallback(&StreamChunk{
						Type:    "content",
						Content: "",
					})
				}
				break
			}
			if chunkCallback != nil {
				chunkCallback(&StreamChunk{
					Type:    "error",
					Content: fmt.Sprintf("接收流消息失败: %v", err),
				})
			}
			return fmt.Errorf("接收流消息失败: %w", err)
		}

		// 发送内容块
		if chunkCallback != nil && msg.Content != "" {
			chunkCallback(&StreamChunk{
				Type:    "content",
				Content: msg.Content,
			})
		}
	}

	return nil
}

// createModel 创建模型
func (r *ReactAgent) createModel() (model.ToolCallingChatModel, error) {
	// 从Factory获取全局配置
	globalCfg := config.GetConfig()
	if globalCfg == nil {
		return nil, fmt.Errorf("全局配置未初始化")
	}

	// 创建模型工厂
	factory := models.NewFactory(globalCfg)

	// 使用工厂创建模型
	return factory.CreateChatModel(r.ctx, r.config.Model)
}

// createToolsConfig 创建工具配置
func (r *ReactAgent) createToolsConfig() (compose.ToolsNodeConfig, error) {
	// 创建工具配置
	toolsConfig := compose.ToolsNodeConfig{
		Tools: []tool.BaseTool{},
	}

	// 获取全局配置
	globalCfg := config.GetConfig()
	if globalCfg == nil {
		return toolsConfig, fmt.Errorf("全局配置未初始化")
	}

	// 添加常规工具
	for _, toolName := range r.config.Tools {
		// 获取工具配置
		toolCfg, ok := globalCfg.Tools[toolName]
		if !ok {
			return toolsConfig, fmt.Errorf("工具配置不存在: %s", toolName)
		}

		// 创建工具实例
		toolInstance, err := createTool(toolName, toolCfg)
		if err != nil {
			return toolsConfig, err
		}

		toolsConfig.Tools = append(toolsConfig.Tools, toolInstance)
	}

	// 添加MCP工具
	if len(r.config.MCPServers) > 0 {
		mcpManager := mcp.GetGlobalManager()
		if mcpManager != nil {
			// 获取当前Agent的MCP工具
			mcpTools, err := mcpManager.GetToolsForAgent(r.agentName)
			if err != nil {
				return toolsConfig, fmt.Errorf("获取MCP工具失败: %w", err)
			}

			// 添加MCP工具到工具配置
			for _, mcpTool := range mcpTools {
				toolsConfig.Tools = append(toolsConfig.Tools, mcpTool)
			}
		}
	}

	return toolsConfig, nil
}

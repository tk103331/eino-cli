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

// ReactAgent implements Agent using React pattern from cloudwego/eino library
type ReactAgent struct {
	config    *config.Agent
	agent     *react.Agent
	ctx       context.Context
	agentName string
}

// ToolCallCallback custom callback handler for capturing tool call information
type ToolCallCallback struct {
	callback func(interface{})
}

// OnStart callback when node starts
func (t *ToolCallCallback) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	if t.callback != nil && info.Name != "" {
		// Send tool start call information
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

// OnEnd callback when node ends
func (t *ToolCallCallback) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	if t.callback != nil && info.Name != "" {
		// Send tool execution completion information
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

// OnError callback when node encounters error
func (t *ToolCallCallback) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	if t.callback != nil && info.Name != "" {
		// Send callback for any named node error
		t.callback(fmt.Sprintf("tool %s execution error: %v", info.Name, err))
	}
	return ctx
}

// OnStartWithStreamInput callback when stream input starts
func (t *ToolCallCallback) OnStartWithStreamInput(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
	return ctx
}

// OnEndWithStreamOutput callback when stream output ends
func (t *ToolCallCallback) OnEndWithStreamOutput(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
	return ctx
}

// NewReactAgent creates a new ReactAgent
func NewReactAgent(agentName string, cfg *config.Agent) *ReactAgent {
	return &ReactAgent{
		config:    cfg,
		ctx:       context.Background(),
		agentName: agentName,
	}
}

// Init initializes Agent
func (r *ReactAgent) Init() error {
	// Create model
	model, err := r.createModel()
	if err != nil {
		return fmt.Errorf("failed to create model: %w", err)
	}

	// Create tools configuration
	toolsConfig, err := r.createToolsConfig()
	if err != nil {
		return fmt.Errorf("failed to create tools configuration: %w", err)
	}

	// Create Agent configuration
	agentConfig := &react.AgentConfig{
		ToolCallingModel: model,
		ToolsConfig:      toolsConfig,
	}

	// Create Agent
	agent, err := react.NewAgent(r.ctx, agentConfig)
	if err != nil {
		return fmt.Errorf("failed to create Agent: %w", err)
	}

	// Save agent instance
	r.agent = agent
	return nil
}

// Run runs Agent
func (r *ReactAgent) Run(prompt string) error {
	if r.agent == nil {
		if err := r.Init(); err != nil {
			return err
		}
	}

	// Create user message
	messages := []*schema.Message{
		schema.SystemMessage(r.config.System),
		schema.UserMessage(prompt),
	}

	// Use react.Generate method to generate response
	response, err := r.agent.Generate(r.ctx, messages)
	if err != nil {
		return fmt.Errorf("failed to run Agent: %w", err)
	}

	if response.Content != "" {
		fmt.Print(response.Content)
	}
	fmt.Println()
	return nil
}

// Chat performs conversation, returns response content
func (r *ReactAgent) Chat(ctx context.Context, prompt string) (string, error) {
	if r.agent == nil {
		if err := r.Init(); err != nil {
			return "", err
		}
	}

	// Create messages
	messages := []*schema.Message{
		schema.SystemMessage(r.config.System),
		schema.UserMessage(prompt),
	}

	// Use Generate method for synchronous call
	response, err := r.agent.Generate(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("Chat failed: %w", err)
	}

	return response.Content, nil
}

// ChatWithCallback performs conversation with streaming output and callback support
func (r *ReactAgent) ChatWithCallback(ctx context.Context, prompt string, callback func(interface{})) (string, error) {
	if r.agent == nil {
		if err := r.Init(); err != nil {
			return "", err
		}
	}

	// Create messages
	messages := []*schema.Message{
		schema.SystemMessage(r.config.System),
		schema.UserMessage(prompt),
	}

	// If no callback function, use Generate method directly
	if callback == nil {
		response, err := r.agent.Generate(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("Chat failed: %w", err)
		}
		return response.Content, nil
	}

	// Create tool call callback handler
	toolCallback := &ToolCallCallback{callback: callback}

	// Use Stream method for streaming call and add callback handler via agent.WithComposeOptions
	sr, err := r.agent.Stream(ctx, messages, agent.WithComposeOptions(compose.WithCallbacks(toolCallback)))
	if err != nil {
		return "", fmt.Errorf("Stream failed: %w", err)
	}
	defer sr.Close()

	var result strings.Builder
	for {
		msg, err := sr.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				// Stream ends
				break
			}
			return "", fmt.Errorf("failed to receive stream message: %w", err)
		}

		// Send callback (for displaying message content)
		if callback != nil && msg.Content != "" {
			callback(msg.Content)
		}

		// Accumulate results
		result.WriteString(msg.Content)
	}

	return result.String(), nil
}

// ChatStream performs streaming conversation, handles streaming output via chunk callback
func (r *ReactAgent) ChatStream(ctx context.Context, prompt string, chunkCallback func(*StreamChunk), toolCallback func(interface{})) error {
	if r.agent == nil {
		if err := r.Init(); err != nil {
			return err
		}
	}

	// Create messages
	messages := []*schema.Message{
		schema.SystemMessage(r.config.System),
		schema.UserMessage(prompt),
	}

	// Create tool call callback handler
	var toolCallCallback *ToolCallCallback
	if toolCallback != nil {
		toolCallCallback = &ToolCallCallback{callback: toolCallback}
	}

	// Use Stream method for streaming call
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
				Content: fmt.Sprintf("Stream failed: %v", err),
			})
		}
		return fmt.Errorf("Stream failed: %w", err)
	}
	defer sr.Close()

	// Read streaming response
	for {
		msg, err := sr.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				// Stream ends, send end marker
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
					Content: fmt.Sprintf("failed to receive stream message: %v", err),
				})
			}
			return fmt.Errorf("failed to receive stream message: %w", err)
		}

		// Send content chunk
		if chunkCallback != nil && msg.Content != "" {
			chunkCallback(&StreamChunk{
				Type:    "content",
				Content: msg.Content,
			})
		}
	}

	return nil
}

// createModel creates model
func (r *ReactAgent) createModel() (model.ToolCallingChatModel, error) {
	// Get global configuration from Factory
	globalCfg := config.GetConfig()
	if globalCfg == nil {
		return nil, fmt.Errorf("global configuration not initialized")
	}

	// Create model factory
	factory := models.NewFactory(globalCfg)

	// Use factory to create model
	return factory.CreateChatModel(r.ctx, r.config.Model)
}

// createToolsConfig creates tools configuration
func (r *ReactAgent) createToolsConfig() (compose.ToolsNodeConfig, error) {
	// Create tools configuration
	toolsConfig := compose.ToolsNodeConfig{
		Tools: []tool.BaseTool{},
	}

	// Get global configuration
	globalCfg := config.GetConfig()
	if globalCfg == nil {
		return toolsConfig, fmt.Errorf("global configuration not initialized")
	}

	// Add regular tools
	for _, toolName := range r.config.Tools {
		// Get tool configuration
		toolCfg, ok := globalCfg.Tools[toolName]
		if !ok {
			return toolsConfig, fmt.Errorf("tool configuration does not exist: %s", toolName)
		}

		// Create tool instance
		toolInstance, err := createTool(toolName, toolCfg)
		if err != nil {
			return toolsConfig, err
		}

		toolsConfig.Tools = append(toolsConfig.Tools, toolInstance)
	}

	// Add MCP tools
	if len(r.config.MCPServers) > 0 {
		mcpManager := mcp.GetGlobalManager()
		if mcpManager != nil {
			// Get current Agent's MCP tools
			mcpTools, err := mcpManager.GetToolsForAgent(r.agentName)
			if err != nil {
				return toolsConfig, fmt.Errorf("failed to get MCP tools: %w", err)
			}

			// Add MCP tools to tools configuration
			for _, mcpTool := range mcpTools {
				toolsConfig.Tools = append(toolsConfig.Tools, mcpTool)
			}
		}
	}

	return toolsConfig, nil
}

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

// AgentApp represents the Agent application structure
type AgentApp struct {
	agentName string
	program   *tea.Program
	model     *ViewModel
	agent     agent.Agent
	ctx       context.Context
}

// ChatApp represents the chat application structure (merged from chat functionality)
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

// NewAgentApp creates a new Agent application
func NewAgentApp(agentName string) (*AgentApp, error) {
	cfg := config.GetConfig()
	if cfg == nil {
		return nil, fmt.Errorf("global configuration not initialized")
	}

	// Check if Agent configuration exists
	_, ok := cfg.Agents[agentName]
	if !ok {
		return nil, fmt.Errorf("Agent configuration does not exist: %s", agentName)
	}

	// Create Agent factory
	factory := agent.NewFactory(cfg)

	// Create Agent instance
	agentInstance, err := factory.CreateAgent(agentName)
	if err != nil {
		return nil, fmt.Errorf("failed to create Agent: %v", err)
	}

	app := &AgentApp{
		agentName: agentName,
		agent:     agentInstance,
		ctx:       context.Background(),
	}

	// Create Agent model, passing in the callback function for sending messages
	agentModel := NewViewModel(app.sendMessage)
	app.model = agentModel

	// Create Bubble Tea program
	app.program = tea.NewProgram(*agentModel, tea.WithAltScreen())

	return app, nil
}

// NewChatApp creates a new chat application (merged from chat functionality)
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

	// Create chat model, passing in the callback function for sending messages
	chatModel := NewViewModel(app.sendMessage)
	app.model = chatModel

	// Create Bubble Tea program
	app.program = tea.NewProgram(*chatModel, tea.WithAltScreen())

	return app
}

// Run runs the Agent application
func (app *AgentApp) Run() error {
	_, err := app.program.Run()
	return err
}

// Run runs the Chat application (merged from chat functionality)
func (app *ChatApp) Run() error {
	_, err := app.program.Run()
	return err
}

// sendMessage sends a message to AI
func (app *AgentApp) sendMessage(message string) error {
	// Get Agent configuration
	cfg := config.GetConfig()
	agentConfig := cfg.Agents[app.agentName]

	// Build message list
	var messages []*schema.Message

	// Add system message (if any)
	if agentConfig.System != "" {
		messages = append(messages, schema.SystemMessage(agentConfig.System))
	}

	// Add user message
	messages = append(messages, schema.UserMessage(message))

	// Handle conversation in goroutine to avoid blocking UI
	go app.processConversation(messages)

	return nil
}

// sendMessage sends a message to AI model (for ChatApp use)
func (app *ChatApp) sendMessage(message string) error {
	// If there are tool configurations, use ReactAgent's ChatWithCallback method
	if len(app.tools) > 0 {
		return app.sendMessageWithAgent(message)
	}

	// Otherwise use the original model direct call method
	return app.sendMessageWithModel(message)
}

// processConversation handles conversation (using streaming output)
func (app *AgentApp) processConversation(messages []*schema.Message) {
	// Get the last user message as prompt
	var prompt string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == schema.User {
			prompt = messages[i].Content
			break
		}
	}

	if prompt == "" {
		app.program.Send(ErrorMsg("User message not found"))
		return
	}

	// Create tool call callback function
	toolCallback := func(msg interface{}) {
		switch v := msg.(type) {
		case struct {
			Name      string
			Arguments string
		}:
			// Only send real tool calls, filter internal components
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
			// Only send real tool call results, filter internal components
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

	// Create streaming content callback function
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

	// Use Agent's ChatStream method for streaming conversation
	err := app.agent.ChatStream(app.ctx, prompt, chunkCallback, toolCallback)
	if err != nil {
		app.program.Send(ErrorMsg(fmt.Sprintf("AI response error: %v", err)))
	}
}

// sendMessageWithAgent sends messages using ReactAgent, supporting tool call callbacks (for ChatApp use)
func (app *ChatApp) sendMessageWithAgent(message string) error {
	// Create temporary Agent configuration
	if app.reactAgent == nil {
		agentConfig := config.Agent{
			System: app.system,
			Model:  app.modelName,
			Tools:  app.tools,
		}

		// Create ReactAgent instance
		reactAgent := agent.NewReactAgent("temp_chat_agent", &agentConfig)
		if err := reactAgent.Init(); err != nil {
			app.program.Send(ErrorMsg(fmt.Sprintf("Failed to initialize Agent: %v", err)))
			return err
		}
		app.reactAgent = reactAgent
	}

	// Run Agent in background and get response
	go func() {
		ctx := context.Background()

		// Create tool call callback function
		callback := func(data interface{}) {
			switch v := data.(type) {
			case string:
				// Streaming content
				app.program.Send(StreamChunkMsg(v))
			case map[string]interface{}:
				// Tool call information
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
								app.program.Send(ErrorMsg(fmt.Sprintf("Tool %s execution error: %s", toolName, errorMsg)))
							}
						}
					}
				}
			}
		}

		// Use Agent's ChatWithCallback method to generate response
		response, err := app.reactAgent.ChatWithCallback(ctx, message, callback)
		if err != nil {
			app.program.Send(ErrorMsg(fmt.Sprintf("AI response error: %v", err)))
			return
		}

		// Send complete response
		if response != "" {
			app.program.Send(ResponseMsg(response))
		}
	}()

	return nil
}

// sendMessageWithModel sends messages using the original model direct call method (for ChatApp use)
func (app *ChatApp) sendMessageWithModel(message string) error {
	// Create model instance (if not created yet)
	if app.chatModel == nil {
		ctx := context.Background()
		chatModel, err := app.modelFactory.CreateChatModel(ctx, app.modelName)
		if err != nil {
			// Send error message to UI
			app.program.Send(ErrorMsg(fmt.Sprintf("Failed to create Model: %v", err)))
			return err
		}

		// If tools are specified, use WithTools method to load tools
		if len(app.tools) > 0 {
			toolInstances, err := app.createTools()
			if err != nil {
				app.program.Send(ErrorMsg(fmt.Sprintf("Failed to create tools: %v", err)))
				return err
			}

			// Convert InvokableTool to ToolInfo
			var toolInfos []*schema.ToolInfo
			for _, tool := range toolInstances {
				info, err := tool.Info(context.Background())
				if err != nil {
					app.program.Send(ErrorMsg(fmt.Sprintf("Failed to get tool information: %v", err)))
					return err
				}
				toolInfos = append(toolInfos, info)
			}

			chatModel, err = chatModel.WithTools(toolInfos)
			if err != nil {
				app.program.Send(ErrorMsg(fmt.Sprintf("Failed to set up tools: %v", err)))
				return err
			}
		}

		app.chatModel = chatModel
	}

	// Run model in background and get streaming response
	go func() {
		ctx := context.Background()

		// Create message list, including optional system prompt
		var messages []*schema.Message
		if app.system != "" {
			messages = append(messages, schema.SystemMessage(app.system))
		}
		messages = append(messages, schema.UserMessage(message))

		// Start conversation loop, handling tool calls
		app.processConversation(ctx, messages)
	}()

	return nil
}

// processConversation handles conversation loop, including tool calls (for ChatApp use)
func (app *ChatApp) processConversation(ctx context.Context, messages []*schema.Message) {
	maxIterations := 10 // Prevent infinite loops
	iteration := 0

	for iteration < maxIterations {
		iteration++

		// Call Model's Stream method to get streaming response
		streamReader, err := app.chatModel.Stream(ctx, messages)
		if err != nil {
			app.program.Send(ErrorMsg(fmt.Sprintf("AI response error: %v", err)))
			return
		}

		// Handle streaming response
		var fullContent string
		var assistantMessage *schema.Message
		var allToolCalls []schema.ToolCall

		for {
			chunk, err := streamReader.Recv()
			if err != nil {
				// Stream ended or error occurred
				if err.Error() != "EOF" && err.Error() != "io: read/write on closed pipe" {
					app.program.Send(ErrorMsg(fmt.Sprintf("Streaming response error: %v", err)))
					streamReader.Close()
					return
				}
				break
			}

			// Accumulate content and send incremental updates
			fullContent += chunk.Content
			app.program.Send(StreamChunkMsg(chunk.Content))

			// Check if current chunk contains tool calls
			if len(chunk.ToolCalls) > 0 {
				// Accumulate all tool calls
				allToolCalls = append(allToolCalls, chunk.ToolCalls...)
			}

			// Save complete assistant message (including possible tool calls)
			assistantMessage = chunk
		}
		streamReader.Close()

		// If tool calls were accumulated, merge them into the final assistant message
		if len(allToolCalls) > 0 && assistantMessage != nil {
			assistantMessage.ToolCalls = allToolCalls
		}

		// Check if there are tool calls
		if assistantMessage != nil && len(assistantMessage.ToolCalls) > 0 {
			// Send complete response to UI (if there is content)
			if fullContent != "" {
				app.program.Send(ResponseMsg(fullContent))
			}

			// Add assistant message to message history
			messages = append(messages, assistantMessage)

			// Execute tool calls
			toolResults, err := app.executeToolCalls(ctx, assistantMessage.ToolCalls)
			if err != nil {
				app.program.Send(ErrorMsg(fmt.Sprintf("Tool execution error: %v", err)))
				return
			}

			// Add tool results to message history
			messages = append(messages, toolResults...)

			// Continue to next round of conversation
			continue
		} else {
			// No tool calls, send final response and end
			if fullContent != "" {
				app.program.Send(ResponseMsg(fullContent))
			}
			break
		}
	}

	if iteration >= maxIterations {
		app.program.Send(ErrorMsg("Maximum iterations reached, stopping conversation"))
	}
}

// executeToolCalls executes tool calls (for ChatApp use)
func (app *ChatApp) executeToolCalls(ctx context.Context, toolCalls []schema.ToolCall) ([]*schema.Message, error) {
	// Create tool instance mapping
	toolInstances, err := app.createTools()
	if err != nil {
		return nil, fmt.Errorf("failed to create tool instances: %v", err)
	}

	// Create tool name to instance mapping
	toolMap := make(map[string]tool.InvokableTool)
	for _, toolInstance := range toolInstances {
		info, err := toolInstance.Info(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get tool information: %v", err)
		}
		toolMap[info.Name] = toolInstance
	}

	var toolMessages []*schema.Message

	// Execute each tool call
	for _, toolCall := range toolCalls {
		toolName := toolCall.Function.Name
		arguments := toolCall.Function.Arguments
		if toolName == "" {
			continue
		}
		// Find tool instance
		toolInstance, exists := toolMap[toolName]
		if !exists {
			// Tool does not exist, return error message
			errorMsg := fmt.Sprintf("Tool '%s' does not exist", toolName)
			toolMessage := schema.ToolMessage(errorMsg, toolCall.ID, schema.WithToolName(toolName))
			toolMessages = append(toolMessages, toolMessage)

			// Display tool not found error
			app.program.Send(StreamChunkMsg(fmt.Sprintf("❌ Tool '%s' does not exist\n", toolName)))
			continue
		}

		// Display tool call information
		app.program.Send(StreamChunkMsg(fmt.Sprintf("\n🔧 Calling tool: %s\nArguments: %s\n", toolName, arguments)))

		// Execute tool
		result, err := toolInstance.InvokableRun(ctx, arguments)
		if err != nil {
			// Tool execution failed, return error message
			errorMsg := fmt.Sprintf("Tool execution failed: %v", err)
			toolMessage := schema.ToolMessage(errorMsg, toolCall.ID, schema.WithToolName(toolName))
			toolMessages = append(toolMessages, toolMessage)

			// Display tool execution error
			app.program.Send(StreamChunkMsg(fmt.Sprintf("❌ Tool execution failed: %v\n", err)))
		} else {
			// Tool execution succeeded, return result
			toolMessage := schema.ToolMessage(result, toolCall.ID, schema.WithToolName(toolName))
			toolMessages = append(toolMessages, toolMessage)

			// Display tool execution result (limit length to avoid UI being too verbose)
			displayResult := result
			if len(result) > 500 {
				displayResult = result[:500] + "...(result truncated)"
			}
			app.program.Send(StreamChunkMsg(fmt.Sprintf("✅ Tool execution result: %s\n", displayResult)))
		}
	}

	return toolMessages, nil
}

// createTools creates tool instances (for ChatApp use)
func (app *ChatApp) createTools() ([]tool.InvokableTool, error) {
	cfg := config.GetConfig()
	if cfg == nil {
		return nil, fmt.Errorf("global configuration not initialized")
	}

	var toolInstances []tool.InvokableTool
	for _, toolName := range app.tools {
		// Get tool configuration
		toolCfg, ok := cfg.Tools[toolName]
		if !ok {
			return nil, fmt.Errorf("tool configuration does not exist: %s", toolName)
		}

		// Create tool instance
		toolInstance, err := tools.CreateTool(toolName, toolCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create tool %s: %v", toolName, err)
		}

		toolInstances = append(toolInstances, toolInstance)
	}

	return toolInstances, nil
}

// isInternalComponent determines if it is an internal component that should not be displayed to users
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

// Stop stops the Agent application
func (app *AgentApp) Stop() {
	if app.program != nil {
		app.program.Quit()
	}
}

// Stop stops the Chat application (merged from chat functionality)
func (app *ChatApp) Stop() {
	if app.program != nil {
		app.program.Quit()
	}
}

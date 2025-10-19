package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
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

// formatArguments formats tool arguments for better readability
func formatArguments(args string) string {
	// Try to parse as JSON first
	var jsonArgs interface{}
	if err := json.Unmarshal([]byte(args), &jsonArgs); err == nil {
		if formatted, err := json.MarshalIndent(jsonArgs, "   ", "  "); err == nil {
			return string(formatted)
		}
	}

	// Handle Go struct format (e.g., &{key value key2 value2})
	if strings.HasPrefix(args, "&{") {
		return formatGoStruct(args)
	}

	// Handle map-like format
	if strings.Contains(args, ":") && strings.Contains(args, "{") {
		return formatMapLike(args)
	}

	// Clean up common formatting issues
	cleaned := strings.ReplaceAll(args, "\n", " ")
	cleaned = regexp.MustCompile(`\s+`).ReplaceAllString(cleaned, " ")
	cleaned = strings.TrimSpace(cleaned)

	// Truncate if still too long
	if len(cleaned) > 300 {
		return cleaned[:297] + "..."
	}

	return cleaned
}

// formatResult formats tool results for better readability
func formatResult(result string) string {
	// Clean up result first
	cleaned := strings.TrimSpace(result)

	// Handle JSON results
	var jsonResult interface{}
	if err := json.Unmarshal([]byte(cleaned), &jsonResult); err == nil {
		if formatted, err := json.MarshalIndent(jsonResult, "   ", "  "); err == nil {
			formattedStr := string(formatted)
			if len(formattedStr) > 500 {
				return formattedStr[:497] + "..."
			}
			return formattedStr
		}
	}

	// Handle multiline results
	lines := strings.Split(cleaned, "\n")
	if len(lines) > 10 {
		return strings.Join(lines[:10], "\n") + "\n... (truncated)"
	}

	// Truncate single line if too long
	if len(cleaned) > 500 {
		return cleaned[:497] + "..."
	}

	return cleaned
}

// formatGoStruct formats Go struct-like strings into readable format
func formatGoStruct(structStr string) string {
	// Remove &{ and }
	content := strings.TrimPrefix(structStr, "&{")
	content = strings.TrimSuffix(content, "}")

	// Split by spaces and try to parse key-value pairs
	parts := strings.Fields(content)
	var result []string

	for i := 0; i < len(parts); i++ {
		part := parts[i]

		// Skip memory addresses and pointers
		if strings.HasPrefix(part, "0x") || len(part) == 14 && part[0] == '0' && part[1] == 'x' {
			continue
		}

		// Skip empty brackets and special chars
		if part == "[]" || part == "<nil>" || part == "map[]" {
			continue
		}

		// Clean up the part
		if strings.Contains(part, ":") {
			result = append(result, part)
		} else if i+1 < len(parts) && !strings.HasPrefix(parts[i+1], "0x") {
			// Assume it's a key-value pair
			result = append(result, part+": "+parts[i+1])
			i++ // Skip next part as it's the value
		} else {
			result = append(result, part)
		}
	}

	formatted := strings.Join(result, ", ")
	if len(formatted) > 300 {
		return formatted[:297] + "..."
	}
	return formatted
}

// formatMapLike formats map-like strings into readable format
func formatMapLike(mapStr string) string {
	// Try to extract key-value pairs
	re := regexp.MustCompile(`(\w+):\s*([^{,}\[\]]+)|(\w+):\s*\{([^}]*)\}`)
	matches := re.FindAllStringSubmatch(mapStr, -1)

	var result []string
	for _, match := range matches {
		if match[1] != "" { // Simple key: value
			key := match[1]
			value := strings.TrimSpace(match[2])
			result = append(result, key+": "+value)
		} else if match[3] != "" { // key: {complex}
			key := match[3]
			value := strings.TrimSpace(match[4])
			if value != "" {
				result = append(result, key+": {"+value+"}")
			} else {
				result = append(result, key+": {}")
			}
		}
	}

	if len(result) > 0 {
		formatted := "{ " + strings.Join(result, ", ") + " }"
		if len(formatted) > 300 {
			return formatted[:297] + "..."
		}
		return formatted
	}

	// Fallback: clean up the original string
	cleaned := regexp.MustCompile(`\s+`).ReplaceAllString(mapStr, " ")
	cleaned = strings.TrimSpace(cleaned)
	if len(cleaned) > 300 {
		return cleaned[:297] + "..."
	}
	return cleaned
}

// formatGeneralInfo formats general callback information
func formatGeneralInfo(info string) string {
	// Skip empty or memory address info
	if info == "" || regexp.MustCompile(`^0x[a-fA-F0-9]+$`).MatchString(info) {
		return ""
	}

	// Handle ChatModel messages
	if strings.Contains(info, "system:") && strings.Contains(info, "user:") {
		return formatChatMessages(info)
	}

	// Handle tool call information
	if strings.Contains(info, "tool_calls:") {
		return formatToolCallInfo(info)
	}

	// Clean up and truncate
	cleaned := strings.ReplaceAll(info, "\n", " ")
	cleaned = regexp.MustCompile(`\s+`).ReplaceAllString(cleaned, " ")
	cleaned = strings.TrimSpace(cleaned)

	if len(cleaned) > 200 {
		return cleaned[:197] + "..."
	}

	return cleaned
}

// formatChatMessages formats chat message information for ChatModel node
func formatChatMessages(info string) string {
	// For ChatModel node, we only want to show a simple indicator
	// instead of the complex message content
	return "🤖 Processing messages with model"
}

// formatToolCallInfo formats tool call information for Tools node
func formatToolCallInfo(info string) string {
	// Parse and format tool call information
	if strings.Contains(info, "tool_calls:") {
		return formatToolCalls(info)
	}

	return truncateString(info, 150)
}

// formatToolCalls extracts and formats individual tool calls
func formatToolCalls(info string) string {
	// Look for tool call patterns in the response
	lines := strings.Split(info, "\n")
	var result []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "tool_calls:") {
			result = append(result, "🔧 Processing tool calls...")
		} else if strings.Contains(line, "Function:{Name:") {
			// Extract tool name
			if start := strings.Index(line, "Name:"); start != -1 {
				nameStart := start + 5
				if end := strings.Index(line[nameStart:], " "); end != -1 {
					toolName := line[nameStart : nameStart+end]
					result = append(result, fmt.Sprintf("   📋 Tool: %s", toolName))
				}
			}
		} else if strings.Contains(line, "Arguments:") {
			// Extract tool arguments
			if start := strings.Index(line, "Arguments:"); start != -1 {
				args := line[start+11:]
				args = strings.TrimSpace(args)
				if args == "{}" {
					result = append(result, "   📝 Arguments: (none)")
				} else {
					result = append(result, fmt.Sprintf("   📝 Arguments: %s", args))
				}
			}
		} else if strings.Contains(line, "finish_reason:") {
			// Extract finish reason
			if start := strings.Index(line, "finish_reason:"); start != -1 {
				reason := strings.TrimSpace(line[start+14:])
				if reason == "tool_calls" {
					result = append(result, "   ✅ Reason: Tool calls completed")
				} else if reason == "stop" {
					result = append(result, "   ✅ Reason: Response completed")
				} else {
					result = append(result, fmt.Sprintf("   ✅ Reason: %s", reason))
				}
			}
		}
	}

	if len(result) == 0 {
		return "🔧 Tool calls detected in response"
	}

	return strings.Join(result, "\n")
}

// truncateString truncates a string to the specified length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ToolCallInfo represents structured tool call information
type ToolCallInfo struct {
	Type      string // "start", "end", "error"
	Name      string
	Arguments string
	Result    string
	Error     string
}

// ToolCallCallback custom callback handler for capturing tool call information
type ToolCallCallback struct {
	callback func(interface{})
}

// OnStart callback when node starts
func (t *ToolCallCallback) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	if t.callback != nil && info.Name != "" {
		// Format arguments for better readability
		args := fmt.Sprintf("%v", input)
		if len(args) > 200 {
			args = args[:197] + "..."
		}

		// Send structured tool start information
		t.callback(ToolCallInfo{
			Type:      "start",
			Name:      info.Name,
			Arguments: args,
		})
	}
	return ctx
}

// OnEnd callback when node ends
func (t *ToolCallCallback) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	if t.callback != nil && info.Name != "" {
		// Format result for better readability
		result := fmt.Sprintf("%v", output)
		if len(result) > 200 {
			result = result[:197] + "..."
		}

		// Send structured tool completion information
		t.callback(ToolCallInfo{
			Type:   "end",
			Name:   info.Name,
			Result: result,
		})
	}
	return ctx
}

// OnError callback when node encounters error
func (t *ToolCallCallback) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	if t.callback != nil && info.Name != "" {
		// Send structured error information
		t.callback(ToolCallInfo{
			Type:  "error",
			Name:  info.Name,
			Error: err.Error(),
		})
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

// Run runs Agent with optimized output formatting
func (r *ReactAgent) Run(prompt string) error {
	if r.agent == nil {
		if err := r.Init(); err != nil {
			return err
		}
	}

	// Use ChatStream method with optimized output formatting
	return r.ChatStream(r.ctx, prompt, func(chunk *StreamChunk) {
		switch chunk.Type {
		case "content":
			if chunk.Content != "" {
				fmt.Print(chunk.Content)
			} else {
				// Empty content marks the end of the stream
				fmt.Println()
			}
		case "tool_start":
			fmt.Printf("\n🔧 Using tool: %s\n", chunk.Tool)
		case "tool_end":
			fmt.Printf("✅ Tool completed: %s\n", chunk.Tool)
		case "error":
			fmt.Printf("\n❌ Error: %s\n", chunk.Content)
		}
	}, func(toolInfo interface{}) {
		// Show detailed tool information with optimized formatting
		switch info := toolInfo.(type) {
		case ToolCallInfo:
			switch info.Type {
			case "start":
				// Handle different node types with specialized formatting
				if info.Name == "ChatModel" {
					fmt.Printf("   🤖 Processing with ChatModel\n")
				} else if info.Name == "Tools" {
					fmt.Printf("   🔧 Processing tool calls\n")
				} else {
					// Regular tool calls
					fmt.Printf("   📋 %s\n", info.Name)
					if info.Arguments != "" {
						// Format arguments for better readability
						formattedArgs := formatArguments(info.Arguments)
						fmt.Printf("   📝 Arguments: %s\n", formattedArgs)
					}
				}
			case "end":
				if info.Name == "ChatModel" {
					fmt.Printf("   ✅ ChatModel response generated\n")
				} else if info.Name == "Tools" {
					fmt.Printf("   ✅ Tool calls processed\n")
				} else {
					// Regular tool results
					if info.Result != "" {
						// Format result for better readability
						formattedResult := formatResult(info.Result)
						fmt.Printf("   📊 Result: %s\n", formattedResult)
					} else {
						fmt.Printf("   ✅ Completed successfully\n")
					}
				}
			case "error":
				fmt.Printf("   ❌ Error: %s\n", info.Error)
			}
		default:
			// Format general callback information
			formattedInfo := formatGeneralInfo(fmt.Sprintf("%v", info))
			if formattedInfo != "" {
				fmt.Printf("   ℹ️  %s\n", formattedInfo)
			}
		}
	})
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

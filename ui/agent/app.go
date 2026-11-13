package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cloudwego/eino/schema"
	"github.com/tk103331/eino-cli/agent"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/logger"
	"github.com/tk103331/eino-cli/session"
	"github.com/tk103331/eino-cli/tools"
)

// AgentApp represents the Agent application structure
type AgentApp struct {
	agentName     string
	modelOverride string // runtime model override via /model
	skill         string // active skill via /skill
	mouseEnabled  bool
	program       *tea.Program
	model         *ViewModel
	agent         agent.Agent
	ctx           context.Context

	// Tool approvals granted "for this session" (in memory only).
	approvals approver

	// cancelRun stops the in-flight turn (Esc twice in the TUI).
	runMu     sync.Mutex
	cancelRun context.CancelFunc

	sessionMu sync.Mutex
	sess      *session.Session // persisted current conversation
}

// NewAgentApp creates a new Agent application
func NewAgentApp(agentName string) (*AgentApp, error) {
	logger.Info("UI-AGENT", fmt.Sprintf("Creating agent app: %s", agentName))

	cfg := config.GetConfig()
	if cfg == nil {
		logger.Error("UI-AGENT", "Global configuration not initialized")
		return nil, fmt.Errorf("global configuration not initialized")
	}

	// Check if Agent configuration exists
	agentConfig, ok := cfg.Agents[agentName]
	if !ok {
		logger.Error("UI-AGENT", fmt.Sprintf("Agent configuration does not exist: %s", agentName))
		return nil, fmt.Errorf("Agent configuration does not exist: %s", agentName)
	}

	logger.Debug("UI-AGENT", fmt.Sprintf("Agent config: Model=%s, Tools=%v, MCP=%v",
		agentConfig.Model, agentConfig.Tools, agentConfig.MCPServers))

	// Interactive tools (ask_user / todolist) need config entries and a TUI host;
	// they are added through the effective config so config files stay clean.
	ensureInteractiveToolConfigs(cfg)

	logger.Info("UI-AGENT", fmt.Sprintf("Successfully created agent: %s", agentName))

	app := &AgentApp{
		agentName:     agentName,
		modelOverride: rememberedModel(cfg, agentName),
		ctx:           context.Background(),
	}
	// The session header records the model actually in use, not the configured
	// default that a remembered /model switch may override.
	app.sess = session.New(session.KindAgent, agentName, app.currentModelName(), "")
	// Build the agent from the effective config (interactive tools included).
	if err := app.rebuildAgent(); err != nil {
		logger.Error("UI-AGENT", fmt.Sprintf("Failed to create agent: %v", err))
		return nil, fmt.Errorf("failed to create Agent: %v", err)
	}

	// Create Agent model, passing in the callback function for sending messages
	agentModel := NewViewModel(app.sendMessage)
	agentModel.SetSubtitle(app.headerSubtitle())
	app.model = agentModel
	agentModel.SetMouseToggle(app.toggleMouse)
	agentModel.SetOnClear(app.startNewSession)
	agentModel.SetOnInterrupt(app.Interrupt)
	app.registerCommands()

	// Create Bubble Tea program
	app.program = tea.NewProgram(agentModel, tea.WithAltScreen())

	logger.Info("UI-AGENT", fmt.Sprintf("Agent app created successfully: %s", agentName))
	return app, nil
}

// headerSubtitle is the label next to the window title: the active agent and
// the model actually in use (a /model switch or a remembered choice wins over
// the configured default).
func (app *AgentApp) headerSubtitle() string {
	return headerLabel(app.agentName, app.currentModelName())
}

// headerLabel joins the agent and model names, skipping empty parts.
func headerLabel(agentName, modelName string) string {
	agentName = strings.TrimSpace(agentName)
	modelName = strings.TrimSpace(modelName)
	switch {
	case agentName == "":
		return modelName
	case modelName == "":
		return agentName
	case agentName == modelName:
		return agentName
	}
	return agentName + " · " + modelName
}

// refreshSubtitle keeps the header in sync after an agent/model switch.
func (app *AgentApp) refreshSubtitle() {
	if app.model != nil {
		app.model.SetSubtitle(app.headerSubtitle())
	}
}

// toggleMouse flips mouse capture and returns the new state.
func (app *AgentApp) toggleMouse() bool {
	app.mouseEnabled = !app.mouseEnabled
	if app.mouseEnabled {
		app.program.EnableMouseCellMotion()
	} else {
		app.program.DisableMouseCellMotion()
	}
	return app.mouseEnabled
}

// Run runs the Agent application
func (app *AgentApp) Run() error {
	return runTUI(app.program)
}

// runTUI starts the bubbletea program with wheel scrolling via the terminal's
// alternate-scroll mode (DECSET 1007): in the alternate screen the wheel is
// translated to arrow keys, so scrolling works while the mouse stays
// uncaptured and native text selection/copy keeps working.
func runTUI(program *tea.Program) error {
	// Best-effort: terminals that do not support 1007 simply ignore it.
	fmt.Print("\x1b[?1007h")
	defer fmt.Print("\x1b[?1007l")
	_, err := program.Run()
	return err
}

// sendMessage sends a message to AI
func (app *AgentApp) sendMessage(message string) error {
	logger.Info("UI-AGENT", fmt.Sprintf("Sending message: %s", truncateForLog(message)))

	// Get the effective agent configuration (config + model/skill overrides)
	agentConfig, ok := app.effectiveAgentConfig()
	if !ok {
		app.program.Send(ErrorMsg(fmt.Sprintf("Agent configuration does not exist: %s", app.agentName)))
		return fmt.Errorf("Agent configuration does not exist: %s", app.agentName)
	}

	// Log the system prompt and replay the persisted turns as context.
	if agentConfig.System != "" {
		logger.Debug("UI-AGENT", fmt.Sprintf("Using system prompt: %s", truncateForLog(agentConfig.System)))
	}
	history := app.historySnapshot()
	app.recordTurn(session.RoleUser, message)

	// Each turn gets its own context so Esc twice can cancel it without
	// affecting the app's lifetime.
	ctx, cancel := context.WithCancel(app.ctx)
	app.runMu.Lock()
	app.cancelRun = cancel
	app.runMu.Unlock()

	// Handle conversation in goroutine to avoid blocking UI
	go func() {
		defer func() {
			cancel()
			app.runMu.Lock()
			app.cancelRun = nil
			app.runMu.Unlock()
		}()
		app.processConversation(ctx, history, message)
	}()

	return nil
}

// Interrupt stops the reply that is currently streaming, if any.
func (app *AgentApp) Interrupt() {
	app.runMu.Lock()
	cancel := app.cancelRun
	app.runMu.Unlock()
	if cancel != nil {
		logger.Info("UI-AGENT", "Interrupting the running reply")
		cancel()
	}
}

// processConversation handles conversation (using streaming output)
func (app *AgentApp) processConversation(ctx context.Context, history []*schema.Message, prompt string) {
	logger.Info("UI-AGENT", "Starting conversation processing")

	if prompt == "" {
		logger.Error("UI-AGENT", "User message not found in conversation")
		app.program.Send(ErrorMsg("User message not found"))
		return
	}

	logger.Info("UI-AGENT", fmt.Sprintf("Processing prompt: %s", truncateForLog(prompt)))

	// Persisted reasoning is flushed as one event at each boundary (reasoning
	// arrives token by token; one JSONL line per token would be wasteful).
	var thought strings.Builder
	flushThought := func() {
		if thought.Len() == 0 {
			return
		}
		app.recordReasoning(thought.String())
		thought.Reset()
	}

	// The eino callback layer can emit the same tool start more than once.
	// Track outstanding calls so the resumed transcript has no duplicates and
	// every start can still be paired with its end.
	pendingTools := make(map[string][]string)
	recordToolStart := func(name, arguments string) {
		flushThought()
		for _, a := range pendingTools[name] {
			if a == arguments {
				return
			}
		}
		pendingTools[name] = append(pendingTools[name], arguments)
		app.recordToolStart(name, arguments)
	}
	recordToolEnd := func(name, result string, failed bool) {
		if list := pendingTools[name]; len(list) > 0 {
			pendingTools[name] = list[1:]
		}
		app.recordToolEnd(name, result, failed)
	}

	// Create tool call callback function
	toolCallback := func(msg interface{}) {
		logger.Debug("UI-CALLBACK", fmt.Sprintf("Received: %T", msg))

		switch v := msg.(type) {
		case agent.ToolCallInfo:
			// Handle ToolCallInfo from ReactAgent
			isInternal := isInternalComponent(v.Name)
			logger.Debug("UI-CALLBACK", fmt.Sprintf("ToolCallInfo: Name=%s, Type=%s, Internal=%v", v.Name, v.Type, isInternal))
			if !isInternal {
				switch v.Type {
				case "start":
					logger.Info("UI-TOOL", fmt.Sprintf("Starting: %s", v.Name))
					logger.Debug("UI-TOOL", fmt.Sprintf("Arguments: %s", v.Arguments))
					recordToolStart(v.Name, v.Arguments)
					app.program.Send(ToolStartMsg{
						Name:      v.Name,
						Arguments: v.Arguments,
					})
				case "end":
					logger.Info("UI-TOOL", fmt.Sprintf("Completed: %s", v.Name))
					logger.Debug("UI-TOOL", fmt.Sprintf("Result: %s", truncateForLog(v.Result)))
					recordToolEnd(v.Name, v.Result, false)
					app.program.Send(ToolEndMsg{
						Name:   v.Name,
						Result: v.Result,
					})
				case "error":
					logger.Error("UI-TOOL", fmt.Sprintf("Tool %s error: %s", v.Name, v.Error))
					recordToolEnd(v.Name, "Error: "+v.Error, true)
					app.program.Send(ErrorMsg(fmt.Sprintf("Tool %s error: %s", v.Name, v.Error)))
				}
			} else {
				logger.Debug("UI-CALLBACK", fmt.Sprintf("Filtered internal component: %s", v.Name))
			}
		case struct {
			Name      string
			Arguments string
		}:
			// Handle legacy format
			isInternal := isInternalComponent(v.Name)
			logger.Debug("UI-CALLBACK", fmt.Sprintf("Legacy start: Name=%s, Internal=%v", v.Name, isInternal))
			if !isInternal {
				logger.Info("UI-TOOL", fmt.Sprintf("Starting (legacy): %s", v.Name))
				logger.Debug("UI-TOOL", fmt.Sprintf("Arguments: %s", v.Arguments))
				recordToolStart(v.Name, v.Arguments)
				app.program.Send(ToolStartMsg{
					Name:      v.Name,
					Arguments: v.Arguments,
				})
			}
		case struct {
			Name   string
			Result string
		}:
			// Handle legacy format
			isInternal := isInternalComponent(v.Name)
			logger.Debug("UI-CALLBACK", fmt.Sprintf("Legacy end: Name=%s, Internal=%v", v.Name, isInternal))
			if !isInternal {
				logger.Info("UI-TOOL", fmt.Sprintf("Completed (legacy): %s", v.Name))
				logger.Debug("UI-TOOL", fmt.Sprintf("Result: %s", truncateForLog(v.Result)))
				recordToolEnd(v.Name, v.Result, false)
				app.program.Send(ToolEndMsg{
					Name:   v.Name,
					Result: v.Result,
				})
			}
		default:
			logger.Warn("UI-CALLBACK", fmt.Sprintf("Unknown callback type: %T", msg))
			if errMsg, ok := msg.(string); ok {
				logger.Error("UI-CALLBACK", fmt.Sprintf("Error message: %s", errMsg))
				app.program.Send(ErrorMsg(errMsg))
			}
		}
	}

	// Create streaming content callback function
	var answer strings.Builder
	chunkCallback := func(chunk *agent.StreamChunk) {
		switch chunk.Type {
		case "content":
			if chunk.Content != "" {
				answer.WriteString(chunk.Content)
				app.program.Send(StreamChunkMsg(chunk.Content))
			} else {
				app.program.Send(StreamEndMsg{})
			}
		case "reasoning":
			if chunk.Content != "" {
				thought.WriteString(chunk.Content)
				app.program.Send(ReasoningChunkMsg(chunk.Content))
			}
		case "error":
			app.program.Send(ErrorMsg(chunk.Content))
		}
	}

	// Use Agent's ChatStream method for streaming conversation. The context
	// carries the host so ask_user/todolist can reach the TUI.
	logger.Info("UI-AGENT", "Calling agent ChatStream")
	ctx = tools.WithHost(ctx, app)
	err := app.agent.ChatStream(ctx, history, prompt, chunkCallback, toolCallback)
	flushThought()
	if err != nil {
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			// Stopped by the user: keep whatever streamed so far instead of
			// reporting a failure.
			logger.Info("UI-AGENT", "Reply stopped by user")
			app.program.Send(StreamEndMsg{})
			if answer.Len() > 0 {
				app.recordTurn(session.RoleAssistant, answer.String())
			}
			app.program.Send(NoticeMsg("已停止当前回复"))
			return
		}
		logger.Error("UI-AGENT", fmt.Sprintf("AI response error: %v", err))
		app.program.Send(ErrorMsg(fmt.Sprintf("AI response error: %v", err)))
	} else {
		logger.Info("UI-AGENT", "ChatStream completed successfully")
		app.recordTurn(session.RoleAssistant, answer.String())
	}
}

// isInternalComponent determines if it is an internal component that should not be displayed to users
func isInternalComponent(name string) bool {
	// Filter out internal ReAct components that don't represent actual tool calls
	// but still show actual tool invocations
	internalComponents := []string{
		"ChatModel",
		"Tools", // This is the ReAct tools orchestrator node, not actual tool calls
	}

	for _, component := range internalComponents {
		if name == component {
			return true
		}
	}
	return false
}

// truncateForLog truncates text for logging to avoid huge log entries
func truncateForLog(text string) string {
	if len(text) > 200 {
		return text[:197] + "..."
	}
	return text
}

// Stop stops the Agent application
func (app *AgentApp) Stop() {
	if app.program != nil {
		app.program.Quit()
	}
}

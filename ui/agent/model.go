package agent

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// MessageType represents the type of a message
type MessageType int

const (
	UserMessage MessageType = iota
	AssistantMessage
	ToolStartMessage
	ToolEndMessage
	ErrorMessage
)

// ToolStatus represents the status of a tool call
type ToolStatus int

const (
	ToolWaiting ToolStatus = iota // Tool is being called (orange)
	ToolSuccess                   // Tool completed successfully (green)
	ToolError                     // Tool failed (red)
)

// Message represents a chat message
type Message struct {
	Type       MessageType
	Content    string
	Name       string     // Tool name (only used for tool messages)
	ToolStatus ToolStatus // Status of tool execution (only used for tool messages)
	Arguments  string     // Tool arguments (only used for tool messages)
	Result     string     // Tool result (only used for tool messages)
	StartTime  int64      // Tool start time (Unix timestamp, only used for tool messages)
	EndTime    int64      // Tool end time (Unix timestamp, only used for tool messages)
}

// ViewModel is the model for the Agent interface
type ViewModel struct {
	messages         []Message
	input            string
	cursor           int
	viewport         int
	width            int
	height           int
	isWaiting        bool
	errorMsg         string
	onSendMsg        func(string) error    // Callback function for sending messages
	streamingContent string                // Currently streaming content
	renderer         *glamour.TermRenderer // Markdown renderer
	scrollOffset     int                   // Scroll offset for up/down key scrolling
}

// Message type definitions
type ResponseMsg string
type StreamChunkMsg string
type StreamEndMsg struct{}
type ErrorMsg string
type ToolStartMsg struct {
	Name      string
	Arguments string
}
type ToolEndMsg struct {
	Name   string
	Result string
}

// NewViewModel creates a new ViewModel
func NewViewModel(onSendMsg func(string) error) *ViewModel {
	// Create glamour renderer - same as chat interface
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
	)

	return &ViewModel{
		messages:     []Message{},
		onSendMsg:    onSendMsg,
		renderer:     renderer,
		scrollOffset: 0,
	}
}

// Init initializes the model
func (m ViewModel) Init() tea.Cmd {
	return nil // Same as chat interface, no initialization commands needed
}

// Update handles messages
func (m ViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.isWaiting {
			// Only allow exit when waiting for response
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "up":
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
			return m, nil
		case "down":
			maxViewport := len(m.messages) - (m.height - 6)
			if maxViewport < 0 {
				maxViewport = 0
			}
			if m.scrollOffset < maxViewport {
				m.scrollOffset++
			}
			return m, nil
		case "pgup":
			// Scroll up by 5 lines
			if m.scrollOffset > 5 {
				m.scrollOffset -= 5
			} else {
				m.scrollOffset = 0
			}
			return m, nil
		case "pgdown":
			// Scroll down by 5 lines
			maxViewport := len(m.messages) - (m.height - 6)
			if maxViewport < 0 {
				maxViewport = 0
			}
			if m.scrollOffset < maxViewport-5 {
				m.scrollOffset += 5
			} else if m.scrollOffset < maxViewport {
				m.scrollOffset = maxViewport
			}
			return m, nil
		case "home":
			// Scroll to top
			m.scrollOffset = 0
			return m, nil
		case "end":
			// Scroll to bottom
			maxViewport := len(m.messages) - (m.height - 6)
			if maxViewport < 0 {
				maxViewport = 0
			}
			m.scrollOffset = maxViewport
			return m, nil
		case "enter":
			if m.input != "" && !m.isWaiting {
				// Add user message
				m.messages = append(m.messages, Message{
					Type:    UserMessage,
					Content: m.input,
				})

				// Send message
				userInput := m.input
				m.input = ""
				m.isWaiting = true
				m.streamingContent = ""
				m.errorMsg = ""

				// Call callback function to send message
				if m.onSendMsg != nil {
					go func() {
						if err := m.onSendMsg(userInput); err != nil {
							// If sending fails, send error message
							m.errorMsg = fmt.Sprintf("Failed to send message: %v", err)
						}
					}()
				}

				return m, nil
			}
		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		default:
			if !m.isWaiting {
				m.input += msg.String()
			}
		}

	case ResponseMsg:
		// Complete response message
		m.messages = append(m.messages, Message{
			Type:    AssistantMessage,
			Content: string(msg),
		})
		m.isWaiting = false
		m.streamingContent = ""
		return m, nil

	case StreamChunkMsg:
		// Streaming response chunk
		m.streamingContent += string(msg)
		return m, nil

	case StreamEndMsg:
		// Stream ended, convert streaming content to formal message
		if m.streamingContent != "" {
			m.messages = append(m.messages, Message{
				Type:    AssistantMessage,
				Content: m.streamingContent,
			})
			m.streamingContent = ""
		}
		m.isWaiting = false
		return m, nil

	case ToolStartMsg:
		// Tool execution started - create waiting state message with timestamp
		startTime := time.Now().Unix()

		m.messages = append(m.messages, Message{
			Type:       ToolStartMessage,
			Content:    "", // Content will be generated dynamically in View
			Name:       msg.Name,
			ToolStatus: ToolWaiting,
			Arguments:  msg.Arguments,
			StartTime:  startTime,
			EndTime:    0,
		})
		return m, nil

	case ToolEndMsg:
		// Tool execution ended - find and update the existing tool message
		toolResult := strings.TrimSpace(msg.Result)

		// Find the most recent tool message with the same name and waiting status
		for i := len(m.messages) - 1; i >= 0; i-- {
			if m.messages[i].Type == ToolStartMessage &&
				m.messages[i].Name == msg.Name &&
				m.messages[i].ToolStatus == ToolWaiting {

				// Determine status based on result
				var newStatus ToolStatus
				var content string

				// Check if result contains error indicators
				if strings.Contains(strings.ToLower(toolResult), "error") ||
					strings.Contains(strings.ToLower(toolResult), "failed") ||
					strings.HasPrefix(toolResult, "❌") {
					newStatus = ToolError
					content = fmt.Sprintf("❌ Tool %s failed", msg.Name)
				} else {
					newStatus = ToolSuccess
					content = fmt.Sprintf("✅ Tool %s completed successfully", msg.Name)
				}

				// Add result if present
				if toolResult != "" {
					if len(toolResult) > 150 {
						toolResult = toolResult[:147] + "..."
					}
					content += fmt.Sprintf("\n📄 Result: %s", toolResult)
				} else {
					if newStatus == ToolSuccess {
						content += "\n✨ Completed without output"
					}
				}

				// Update the existing message
				m.messages[i].Content = "" // Content will be generated dynamically in View
				m.messages[i].ToolStatus = newStatus
				m.messages[i].Result = toolResult
				m.messages[i].EndTime = time.Now().Unix() // Record end time
				return m, nil                             // Exit early, don't create a new message
			}
		}

		// If no waiting tool message was found, create a new one
		content := fmt.Sprintf("✅ Tool %s completed", msg.Name)
		if toolResult != "" {
			if len(toolResult) > 150 {
				toolResult = toolResult[:147] + "..."
			}
			content += fmt.Sprintf("\n📄 Result: %s", toolResult)
		}

		m.messages = append(m.messages, Message{
			Type:       ToolStartMessage, // Use ToolStartMessage for consistent rendering
			Content:    content,
			Name:       msg.Name,
			ToolStatus: ToolSuccess,
			Result:     toolResult,
		})
		return m, nil

	case ErrorMsg:
		// Error message - directly display all error messages (filtering handled at application layer)
		errorText := string(msg)
		m.messages = append(m.messages, Message{
			Type:    ErrorMessage,
			Content: errorText,
		})
		m.isWaiting = false
		m.streamingContent = ""
		m.errorMsg = errorText
		return m, nil
	}

	return m, nil
}

// View renders the interface
func (m ViewModel) View() string {
	// Define color scheme
	primaryColor := "#7C3AED"   // Purple
	secondaryColor := "#06B6D4" // Cyan
	successColor := "#10B981"   // Green
	warningColor := "#F59E0B"   // Amber
	errorColor := "#EF4444"     // Red
	userColor := "#8B5CF6"      // Violet
	mutedColor := "#6B7280"     // Gray

	// Enhanced style definitions
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(primaryColor)).
		Bold(true).
		Padding(0, 2).
		Width(m.width).
		Align(lipgloss.Center).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(primaryColor)).
		MarginBottom(1)

	userStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(userColor)).
		Bold(true).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(userColor)).
		MarginLeft(2).
		MarginRight(2)

	assistantStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(secondaryColor)).
		Bold(true).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(secondaryColor)).
		MarginLeft(2).
		MarginRight(2)

	// Unified tool call styles based on status
	toolWaitingStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(warningColor)).
		Bold(true).
		Italic(true).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(warningColor)).
		MarginLeft(2).
		MarginRight(2).
		Width(m.width - 8)

	toolSuccessStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(successColor)).
		Bold(true).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(successColor)).
		MarginLeft(2).
		MarginRight(2).
		Width(m.width - 8)

	toolErrorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(errorColor)).
		Bold(true).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(errorColor)).
		MarginLeft(2).
		MarginRight(2).
		Width(m.width - 8)

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(errorColor)).
		Bold(true).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(errorColor)).
		MarginLeft(2).
		MarginRight(2).
		Width(m.width - 8)

	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(primaryColor)).
		Padding(0, 1).
		Width(m.width - 4)

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(mutedColor)).
		Faint(true).
		Align(lipgloss.Center).
		MarginTop(1)

	waitingStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(secondaryColor)).
		Italic(true).
		Bold(true).
		Padding(0, 1)

	// Build enhanced header with branding
	logo := "🤖"
	title := fmt.Sprintf("%s Eino CLI Agent", logo)
	if m.width > 50 {
		title = fmt.Sprintf("%s Eino CLI Agent - AI Assistant", logo)
	}

	// Add status indicator to header
	statusIndicator := "● Ready"
	statusColor := successColor
	if m.isWaiting {
		statusIndicator = "● Thinking..."
		statusColor = warningColor
	} else if m.errorMsg != "" {
		statusIndicator = "● Error"
		statusColor = errorColor
	}

	headerContent := fmt.Sprintf("%s %s", title,
		lipgloss.NewStyle().
			Foreground(lipgloss.Color(statusColor)).
			Render(statusIndicator))

	// Build message display area
	var messageLines []string

	// Display message history with enhanced scrolling
	visibleMessages := m.messages
	if m.scrollOffset > 0 && m.scrollOffset < len(m.messages) {
		visibleMessages = m.messages[m.scrollOffset:]
	}

	for _, msg := range visibleMessages {
		switch msg.Type {
		case UserMessage:
			userIcon := "👤 "
			userLabel := userIcon + "You"
			messageLines = append(messageLines, userStyle.Render(userLabel))
			// Add content with proper indentation
			contentLines := strings.Split(msg.Content, "\n")
			for _, line := range contentLines {
				messageLines = append(messageLines, "    "+line)
			}
			messageLines = append(messageLines, "")

		case AssistantMessage:
			aiIcon := "🎯 "
			aiLabel := aiIcon + "Assistant"
			messageLines = append(messageLines, assistantStyle.Render(aiLabel))
			// Use markdown rendering for AI replies
			renderedContent := m.renderMarkdown(msg.Content)
			contentLines := strings.Split(renderedContent, "\n")
			for _, line := range contentLines {
				messageLines = append(messageLines, "    "+line)
			}
			messageLines = append(messageLines, "")

		case ToolStartMessage:
			// Render tool message based on its status
			var toolStyle lipgloss.Style
			switch msg.ToolStatus {
			case ToolWaiting:
				toolStyle = toolWaitingStyle
			case ToolSuccess:
				toolStyle = toolSuccessStyle
			case ToolError:
				toolStyle = toolErrorStyle
			default:
				toolStyle = toolWaitingStyle
			}

			// Generate formatted content with header/parameters/result sections
			formattedContent := m.formatToolCallContent(msg)
			messageLines = append(messageLines, toolStyle.Render(formattedContent))
			messageLines = append(messageLines, "")

		case ToolEndMessage:
			// ToolEndMessage is now handled by updating ToolStartMessage
			// Only render if it's a standalone message (fallback)
			var toolStyle lipgloss.Style
			switch msg.ToolStatus {
			case ToolSuccess:
				toolStyle = toolSuccessStyle
			case ToolError:
				toolStyle = toolErrorStyle
			default:
				toolStyle = toolSuccessStyle
			}

			messageLines = append(messageLines, toolStyle.Render(msg.Content))
			messageLines = append(messageLines, "")

		case ErrorMessage:
			errorIcon := "❌ "
			errorLabel := errorIcon + "Error"
			messageLines = append(messageLines, errorStyle.Render(errorLabel))
			// Add error content with proper indentation
			contentLines := strings.Split(msg.Content, "\n")
			for _, line := range contentLines {
				messageLines = append(messageLines, "    "+line)
			}
			messageLines = append(messageLines, "")
		}
	}

	// Display currently streaming content with enhanced styling
	if m.streamingContent != "" {
		aiIcon := "🎯 "
		aiLabel := aiIcon + "Assistant (typing...)"
		messageLines = append(messageLines, assistantStyle.Render(aiLabel))
		// Use markdown rendering for streaming content as well
		renderedStreamContent := m.renderMarkdown(m.streamingContent)
		contentLines := strings.Split(renderedStreamContent, "\n")
		for _, line := range contentLines {
			messageLines = append(messageLines, "    "+line)
		}
		messageLines = append(messageLines, "")
	}

	// Display enhanced waiting status
	if m.isWaiting {
		thinkingIcon := "🤔 "
		waitingText := fmt.Sprintf("%sAI is thinking...", thinkingIcon)
		messageLines = append(messageLines, waitingStyle.Render(waitingText))
		messageLines = append(messageLines, "")
	}

	// Display error information (if not already shown as message)
	if m.errorMsg != "" {
		// Check if error is already in messages to avoid duplication
		hasErrorMessage := false
		for _, msg := range m.messages {
			if msg.Type == ErrorMessage && msg.Content == m.errorMsg {
				hasErrorMessage = true
				break
			}
		}
		if !hasErrorMessage {
			errorIcon := "⚠️ "
			errorLabel := errorIcon + "System Error"
			messageLines = append(messageLines, errorStyle.Render(errorLabel))
			contentLines := strings.Split(m.errorMsg, "\n")
			for _, line := range contentLines {
				messageLines = append(messageLines, "    "+line)
			}
			messageLines = append(messageLines, "")
		}
	}

	// If no messages, show welcome message
	if len(messageLines) == 0 {
		welcomeIcon := "👋 "
		welcomeText := fmt.Sprintf("%sWelcome to Eino CLI Agent! I'm ready to help you.", welcomeIcon)
		messageLines = append(messageLines,
			lipgloss.NewStyle().
				Foreground(lipgloss.Color(mutedColor)).
				Italic(true).
				Render(welcomeText))
		messageLines = append(messageLines, "")
	}

	// Limit the number of displayed lines
	maxLines := m.height - 6 // Reserve space for header, input box, help and borders
	if maxLines > 0 && len(messageLines) > maxLines {
		messageLines = messageLines[len(messageLines)-maxLines:]
	}

	// Add scroll indicator if needed
	scrollIndicator := ""
	if len(m.messages) > maxLines {
		scrollPosition := fmt.Sprintf("%d/%d", m.scrollOffset+1, len(m.messages))
		scrollIndicator = lipgloss.NewStyle().
			Foreground(lipgloss.Color(mutedColor)).
			Faint(true).
			Render(fmt.Sprintf(" [%s]", scrollPosition))
	}

	messageArea := strings.Join(messageLines, "\n")

	// Build enhanced input area
	inputIcon := "💬 "
	inputPrompt := inputIcon + "> "
	if m.isWaiting {
		inputPrompt = "⏳ Please wait..."
	}

	// Add character count if input is getting long
	charCount := ""
	if len(m.input) > 50 {
		charCount = fmt.Sprintf(" [%d]", len(m.input))
	}

	inputText := inputPrompt + m.input + charCount
	inputArea := inputStyle.Render(inputText)

	// Build enhanced help information
	helpItems := []string{
		"Ctrl+C" + " → " + "Quit",
		"↑/↓" + " → " + "Scroll",
		"Enter" + " → " + "Send",
		"Home/End" + " → " + "Top/Bottom",
	}

	// Add scroll hint if applicable
	if len(m.messages) > maxLines {
		helpItems = append(helpItems, "PageUp/Down → Faster scroll")
	}

	helpText := strings.Join(helpItems, " • ")
	helpArea := helpStyle.Render("📌 " + helpText + scrollIndicator)

	// Combine all components
	header := titleStyle.Render(headerContent)

	return fmt.Sprintf("%s\n%s\n\n%s\n%s", header, messageArea, inputArea, helpArea)
}

// formatToolCallContent generates formatted content for tool calls with header/parameters/result sections
func (m *ViewModel) formatToolCallContent(msg Message) string {
	var sections []string

	// Header section with tool name, status and duration
	var statusIcon, statusText, durationText string
	switch msg.ToolStatus {
	case ToolWaiting:
		statusIcon = "⏳"
		statusText = "Calling"
		durationText = "calculating..."
	case ToolSuccess:
		statusIcon = "✅"
		statusText = "Completed"
		if msg.StartTime > 0 && msg.EndTime > 0 {
			duration := msg.EndTime - msg.StartTime
			if duration < 1 {
				durationText = "< 1s"
			} else {
				durationText = fmt.Sprintf("%ds", duration)
			}
		}
	case ToolError:
		statusIcon = "❌"
		statusText = "Failed"
		if msg.StartTime > 0 && msg.EndTime > 0 {
			duration := msg.EndTime - msg.StartTime
			if duration < 1 {
				durationText = "< 1s"
			} else {
				durationText = fmt.Sprintf("%ds", duration)
			}
		}
	default:
		statusIcon = "⏳"
		statusText = "Calling"
		durationText = "calculating..."
	}

	header := fmt.Sprintf("%s %s: %s (%s)", statusIcon, statusText, msg.Name, durationText)
	sections = append(sections, header)

	// Parameters section (only show if arguments exist)
	if msg.Arguments != "" && msg.Arguments != "{}" {
		sections = append(sections, "")
		sections = append(sections, "📋 Parameters:")
		sections = append(sections, fmt.Sprintf("   %s", msg.Arguments))
	}

	// Result section (only show if tool has completed and result exists)
	if msg.ToolStatus != ToolWaiting && msg.Result != "" {
		sections = append(sections, "")
		sections = append(sections, "📄 Result:")
		// Handle long results by truncating
		result := msg.Result
		if len(result) > 200 {
			result = result[:197] + "..."
		}
		sections = append(sections, fmt.Sprintf("   %s", result))
	} else if msg.ToolStatus == ToolSuccess && msg.Result == "" {
		sections = append(sections, "")
		sections = append(sections, "✨ Completed without output")
	} else if msg.ToolStatus == ToolWaiting {
		sections = append(sections, "")
		sections = append(sections, "⌛ Please wait...")
	}

	return strings.Join(sections, "\n")
}

// renderMarkdown renders markdown content - same as chat interface
func (m *ViewModel) renderMarkdown(content string) string {
	if m.renderer == nil {
		return content // If renderer is not initialized, return original content
	}

	rendered, err := m.renderer.Render(content)
	if err != nil {
		return content // If rendering fails, return original content
	}

	return strings.TrimSpace(rendered)
}

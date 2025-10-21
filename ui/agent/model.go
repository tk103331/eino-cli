package agent

import (
	"encoding/json"
	"fmt"
	"regexp"
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
	scrollOffset     int                   // Scroll offset for up/down key scrolling (line-based)
	renderedLines    []string              // Cached rendered lines for efficient scrolling
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
		messages:      []Message{},
		onSendMsg:     onSendMsg,
		renderer:      renderer,
		scrollOffset:  0,
		renderedLines: []string{},
	}
}

// updateRenderedLines updates the cached rendered lines for efficient scrolling
func (m *ViewModel) updateRenderedLines() {
	var lines []string

	// Define color scheme (same as View function)
	secondaryColor := "#06B6D4" // Cyan
	successColor := "#10B981"   // Green
	warningColor := "#F59E0B"   // Amber
	errorColor := "#EF4444"     // Red
	userColor := "#8B5CF6"      // Violet
	mutedColor := "#6B7280"     // Gray

	// Style definitions (same as View function)
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

	waitingStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(secondaryColor)).
		Italic(true).
		Bold(true).
		Padding(0, 1)

	// Render all messages
	for _, msg := range m.messages {
		switch msg.Type {
		case UserMessage:
			userIcon := "👤 "
			userLabel := userIcon + "You"
			lines = append(lines, userStyle.Render(userLabel))
			contentLines := strings.Split(msg.Content, "\n")
			for _, line := range contentLines {
				lines = append(lines, "    "+line)
			}
			lines = append(lines, "")

		case AssistantMessage:
			aiIcon := "🎯 "
			aiLabel := aiIcon + "Assistant"
			lines = append(lines, assistantStyle.Render(aiLabel))
			renderedContent := m.renderMarkdown(msg.Content)
			contentLines := strings.Split(renderedContent, "\n")
			for _, line := range contentLines {
				lines = append(lines, "    "+line)
			}
			lines = append(lines, "")

		case ToolStartMessage:
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
			formattedContent := m.formatToolCallContent(msg)
			lines = append(lines, toolStyle.Render(formattedContent))
			lines = append(lines, "")

		case ToolEndMessage:
			var toolStyle lipgloss.Style
			switch msg.ToolStatus {
			case ToolSuccess:
				toolStyle = toolSuccessStyle
			case ToolError:
				toolStyle = toolErrorStyle
			default:
				toolStyle = toolSuccessStyle
			}
			lines = append(lines, toolStyle.Render(msg.Content))
			lines = append(lines, "")

		case ErrorMessage:
			errorIcon := "❌ "
			errorLabel := errorIcon + "Error"
			lines = append(lines, errorStyle.Render(errorLabel))
			contentLines := strings.Split(msg.Content, "\n")
			for _, line := range contentLines {
				lines = append(lines, "    "+line)
			}
			lines = append(lines, "")
		}
	}

	// Add streaming content
	if m.streamingContent != "" {
		aiIcon := "🎯 "
		aiLabel := aiIcon + "Assistant (typing...)"
		lines = append(lines, assistantStyle.Render(aiLabel))
		renderedStreamContent := m.renderMarkdown(m.streamingContent)
		contentLines := strings.Split(renderedStreamContent, "\n")
		for _, line := range contentLines {
			lines = append(lines, "    "+line)
		}
		lines = append(lines, "")
	}

	// Add waiting status
	if m.isWaiting {
		thinkingIcon := "🤔 "
		waitingText := fmt.Sprintf("%sAI is thinking...", thinkingIcon)
		lines = append(lines, waitingStyle.Render(waitingText))
		lines = append(lines, "")
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
			lines = append(lines, errorStyle.Render(errorLabel))
			contentLines := strings.Split(m.errorMsg, "\n")
			for _, line := range contentLines {
				lines = append(lines, "    "+line)
			}
			lines = append(lines, "")
		}
	}

	// If no messages, show welcome message
	if len(lines) == 0 {
		welcomeIcon := "👋 "
		welcomeText := fmt.Sprintf("%sWelcome to Eino CLI Agent! I'm ready to help you.", welcomeIcon)
		lines = append(lines,
			lipgloss.NewStyle().
				Foreground(lipgloss.Color(mutedColor)).
				Italic(true).
				Render(welcomeText))
		lines = append(lines, "")
	}

	m.renderedLines = lines
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
			switch msg.Type {
			case tea.KeyCtrlC, tea.KeyEsc:
				return m, tea.Quit
			}
			return m, nil
		}

		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyUp:
			// Scroll up to see older content (increase scroll offset)
			m.updateRenderedLines()
			maxLines := m.height - 6
			if maxLines <= 0 {
				maxLines = 1
			}
			maxScroll := len(m.renderedLines) - maxLines
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.scrollOffset < maxScroll {
				m.scrollOffset++
			}
			return m, nil
		case tea.KeyDown:
			// Scroll down to see newer content (decrease scroll offset)
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
			return m, nil
		case tea.KeyPgUp:
			// Scroll up by 5 lines (to older content)
			m.updateRenderedLines()
			maxLines := m.height - 6
			if maxLines <= 0 {
				maxLines = 1
			}
			maxScroll := len(m.renderedLines) - maxLines
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.scrollOffset < maxScroll-5 {
				m.scrollOffset += 5
			} else if m.scrollOffset < maxScroll {
				m.scrollOffset = maxScroll
			}
			return m, nil
		case tea.KeyPgDown:
			// Scroll down by 5 lines (to newer content)
			if m.scrollOffset > 5 {
				m.scrollOffset -= 5
			} else {
				m.scrollOffset = 0
			}
			return m, nil
		case tea.KeyHome:
			// Scroll to top (oldest content)
			m.updateRenderedLines()
			maxLines := m.height - 6
			if maxLines <= 0 {
				maxLines = 1
			}
			maxScroll := len(m.renderedLines) - maxLines
			if maxScroll < 0 {
				maxScroll = 0
			}
			m.scrollOffset = maxScroll
			return m, nil
		case tea.KeyEnd:
			// Scroll to bottom (newest content)
			m.scrollOffset = 0
			return m, nil
		case tea.KeyEnter:
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

				// Reset scroll to bottom when new message is sent
				m.scrollOffset = 0

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
		case tea.KeyBackspace:
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		case tea.KeyRunes:
			if !m.isWaiting {
				m.input += string(msg.Runes)
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
		// Auto-scroll to bottom when response is complete
		m.scrollOffset = 0
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
		// Auto-scroll to bottom when stream ends
		m.scrollOffset = 0
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

				// Enhanced error detection with better patterns
				isError := false

				// Check for common error patterns
				lowerResult := strings.ToLower(toolResult)
				errorPatterns := []string{
					"error", "failed", "exception", "timeout",
					"not found", "invalid", "unauthorized",
					"forbidden", "denied", "panic", "fatal",
				}

				for _, pattern := range errorPatterns {
					if strings.Contains(lowerResult, pattern) {
						isError = true
						break
					}
				}

				// Also check for common error prefixes
				errorPrefixes := []string{"❌", "⚠️", "❗", "✖️", "×"}
				for _, prefix := range errorPrefixes {
					if strings.HasPrefix(toolResult, prefix) {
						isError = true
						break
					}
				}

				// Try to extract a meaningful error message
				if isError {
					newStatus = ToolError
					errorMsg := m.extractErrorMessage(toolResult)
					if errorMsg != "" {
						content = fmt.Sprintf("❌ Tool %s failed:\n%s", msg.Name, errorMsg)
					} else {
						content = fmt.Sprintf("❌ Tool %s failed", msg.Name)
					}
				} else {
					newStatus = ToolSuccess
					content = fmt.Sprintf("✅ Tool %s completed successfully", msg.Name)
				}

				// Add result if present
				if toolResult != "" {
					if len(toolResult) > 150 {
						toolResult = toolResult[:147] + "..."
					}
					if isError {
						content += fmt.Sprintf("\n📄 Error details: %s", toolResult)
					} else {
						content += fmt.Sprintf("\n📄 Result: %s", toolResult)
					}
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
	// Define color scheme (needed for status indicator)
	primaryColor := "#7C3AED"   // Purple
	successColor := "#10B981"   // Green
	warningColor := "#F59E0B"   // Amber
	errorColor := "#EF4444"     // Red
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

	// Update rendered lines cache
	m.updateRenderedLines()

	// Use line-based scrolling
	var visibleLines []string
	maxLines := m.height - 6 // Reserve space for header, input box, help and borders

	if len(m.renderedLines) > maxLines && maxLines > 0 {
		// Apply scroll offset - show newest content by default (scrollOffset = 0)
		start := len(m.renderedLines) - maxLines - m.scrollOffset
		if start < 0 {
			start = 0
		}
		end := start + maxLines
		if end > len(m.renderedLines) {
			end = len(m.renderedLines)
		}
		visibleLines = m.renderedLines[start:end]
	} else {
		visibleLines = m.renderedLines
	}

	// Add scroll indicator if needed
	scrollIndicator := ""
	if len(m.renderedLines) > maxLines && maxLines > 0 {
		// Show current view range relative to total content
		if len(m.renderedLines) > maxLines && m.scrollOffset > 0 {
			startLine := len(m.renderedLines) - maxLines - m.scrollOffset + 1
			endLine := len(m.renderedLines) - m.scrollOffset
			scrollPosition := fmt.Sprintf("%d-%d/%d", startLine, endLine, len(m.renderedLines))
			scrollIndicator = lipgloss.NewStyle().
				Foreground(lipgloss.Color(mutedColor)).
				Faint(true).
				Render(fmt.Sprintf(" [%s]", scrollPosition))
		} else {
			// Showing newest content (scrollOffset = 0)
			startLine := len(m.renderedLines) - maxLines + 1
			if startLine < 1 {
				startLine = 1
			}
			scrollPosition := fmt.Sprintf("%d-%d/%d", startLine, len(m.renderedLines), len(m.renderedLines))
			scrollIndicator = lipgloss.NewStyle().
				Foreground(lipgloss.Color(mutedColor)).
				Faint(true).
				Render(fmt.Sprintf(" [%s]", scrollPosition))
		}
	}

	messageArea := strings.Join(visibleLines, "\n")

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

// formatToolCallContent generates formatted content for tool calls with simplified display
func (m *ViewModel) formatToolCallContent(msg Message) string {
	var sections []string

	// Simplified header with tool name and status
	var statusIcon string
	switch msg.ToolStatus {
	case ToolWaiting:
		statusIcon = "⏳"
	case ToolSuccess:
		statusIcon = "✅"
	case ToolError:
		statusIcon = "❌"
	default:
		statusIcon = "⏳"
	}

	// Simplified header - remove duration for cleaner display
	header := fmt.Sprintf("%s %s", statusIcon, msg.Name)
	sections = append(sections, header)

	// Show arguments only if they're meaningful (not empty JSON and not too long)
	if msg.Arguments != "" && msg.Arguments != "{}" && len(msg.Arguments) < 100 {
		// Try to format as JSON if it looks like JSON
		arguments := msg.Arguments
		if strings.HasPrefix(arguments, "{") && strings.HasSuffix(arguments, "}") {
			// JSON arguments - try to make them more readable
			var jsonArgs interface{}
			if err := json.Unmarshal([]byte(arguments), &jsonArgs); err == nil {
				if compact, err := json.Marshal(jsonArgs); err == nil {
					arguments = string(compact)
				}
			}
		}
		sections = append(sections, fmt.Sprintf("📝 %s", arguments))
	}

	// Show result with smart truncation
	if msg.ToolStatus != ToolWaiting && msg.Result != "" {
		result := msg.Result
		// For successful tools, show more concise result
		if msg.ToolStatus == ToolSuccess {
			if len(result) > 150 {
				result = result[:147] + "..."
			}
			sections = append(sections, fmt.Sprintf("📄 %s", result))
		} else {
			// For errors, show slightly more detail
			if len(result) > 200 {
				result = result[:197] + "..."
			}
			sections = append(sections, fmt.Sprintf("❌ %s", result))
		}
	} else if msg.ToolStatus == ToolWaiting {
		sections = append(sections, "⌛ Processing...")
	}

	return strings.Join(sections, "\n")
}

// extractErrorMessage tries to extract a meaningful error message from tool output
func (m *ViewModel) extractErrorMessage(toolResult string) string {
	lines := strings.Split(strings.TrimSpace(toolResult), "\n")

	// Look for the most meaningful error line
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip common unhelpful lines
		skipPatterns := []string{
			"stack trace:", "at ", "goroutine ", "created by ",
			"panic:", "runtime error:", "call stack:",
		}

		shouldSkip := false
		lowerLine := strings.ToLower(line)
		for _, pattern := range skipPatterns {
			if strings.Contains(lowerLine, pattern) {
				shouldSkip = true
				break
			}
		}

		if !shouldSkip && len(line) > 5 && len(line) < 100 {
			return line
		}
	}

	// If no good line found, try to extract from JSON error messages
	if strings.Contains(toolResult, "{") && strings.Contains(toolResult, "}") {
		// Look for common JSON error fields
		errorFields := []string{"error", "message", "msg", "description", "detail"}
		for _, field := range errorFields {
			pattern := fmt.Sprintf(`"%s":\s*"([^"]+)"`, field)
			re := regexp.MustCompile(pattern)
			matches := re.FindStringSubmatch(toolResult)
			if len(matches) > 1 {
				errorMsg := matches[1]
				if len(errorMsg) > 10 && len(errorMsg) < 80 {
					return errorMsg
				}
			}
		}
	}

	// If still nothing good, return first non-empty line (truncated if too long)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			if len(line) > 80 {
				return line[:77] + "..."
			}
			return line
		}
	}

	return ""
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

package agent

import (
	"fmt"
	"strings"

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

// Message represents a chat message
type Message struct {
	Type    MessageType
	Content string
	Name    string // Tool name (only used for tool messages)
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
			maxViewport := len(m.messages) - (m.height - 4)
			if maxViewport < 0 {
				maxViewport = 0
			}
			if m.scrollOffset < maxViewport {
				m.scrollOffset++
			}
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
		// Tool execution started
		content := fmt.Sprintf("Calling tool: %s", msg.Name)
		if msg.Arguments != "" && msg.Arguments != "{}" {
			content += fmt.Sprintf("\nArguments: %s", msg.Arguments)
		}
		m.messages = append(m.messages, Message{
			Type:    ToolStartMessage,
			Content: content,
			Name:    msg.Name,
		})
		return m, nil

	case ToolEndMsg:
		// Tool execution ended
		content := fmt.Sprintf("Tool %s completed", msg.Name)
		if msg.Result != "" {
			// Clean up result, remove extra newlines
			result := strings.TrimSpace(msg.Result)
			if len(result) > 200 {
				// If result is too long, truncate and add ellipsis
				result = result[:197] + "..."
			}
			content += fmt.Sprintf("\nResult: %s", result)
		}
		m.messages = append(m.messages, Message{
			Type:    ToolEndMessage,
			Content: content,
			Name:    msg.Name,
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
	// Style definitions - completely reference chat interface
	userStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ff00")).
		Bold(true)

	assistantStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#0099ff")).
		Bold(true)

	toolStartStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffaa00")).
		Bold(true)

	toolEndStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00aa00")).
		Bold(true)

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ff0000")).
		Bold(true)

	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#666666")).
		Padding(0, 1).
		Width(m.width - 4) // Subtract border and padding width

	// Build message display area
	var messageLines []string
	messageLines = append(messageLines, "=== Eino CLI Agent ===")
	messageLines = append(messageLines, "")

	// Display message history - use scrolling logic
	visibleMessages := m.messages
	if m.scrollOffset > 0 && m.scrollOffset < len(m.messages) {
		visibleMessages = m.messages[m.scrollOffset:]
	}

	for _, msg := range visibleMessages {
		switch msg.Type {
		case UserMessage:
			messageLines = append(messageLines, userStyle.Render("You: ")+msg.Content)
		case AssistantMessage:
			// Use markdown rendering for AI replies
			renderedContent := m.renderMarkdown(msg.Content)
			messageLines = append(messageLines, assistantStyle.Render("AI: ")+renderedContent)
		case ToolStartMessage:
			messageLines = append(messageLines, toolStartStyle.Render(msg.Content))
		case ToolEndMessage:
			messageLines = append(messageLines, toolEndStyle.Render(msg.Content))
		case ErrorMessage:
			messageLines = append(messageLines, errorStyle.Render("Error: ")+msg.Content)
		}
		messageLines = append(messageLines, "")
	}

	// Display currently streaming content
	if m.streamingContent != "" {
		// Use markdown rendering for streaming content as well
		renderedStreamContent := m.renderMarkdown(m.streamingContent)
		messageLines = append(messageLines, assistantStyle.Render("AI: ")+renderedStreamContent)
		messageLines = append(messageLines, "")
	}

	// Display waiting status
	if m.isWaiting {
		messageLines = append(messageLines, "AI is thinking...")
		messageLines = append(messageLines, "")
	}

	// Display error information
	if m.errorMsg != "" {
		messageLines = append(messageLines, errorStyle.Render("Error: "+m.errorMsg))
		messageLines = append(messageLines, "")
	}

	// Limit the number of displayed lines
	maxLines := m.height - 4 // Reserve space for input box and border
	if maxLines > 0 && len(messageLines) > maxLines {
		messageLines = messageLines[len(messageLines)-maxLines:]
	}

	messageArea := strings.Join(messageLines, "\n")

	// Build input area
	inputPrompt := "> "
	if m.isWaiting {
		inputPrompt = "Waiting for response... "
	}
	inputArea := inputStyle.Render(inputPrompt + m.input)

	// Build help information
	helpText := "Press Ctrl+C to quit, ↑/↓ to scroll, Enter to send"

	return fmt.Sprintf("%s\n\n%s\n%s", messageArea, inputArea, helpText)
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

package agent

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// MessageType 消息类型
type MessageType int

const (
	UserMessage MessageType = iota
	AssistantMessage
	ToolStartMessage
	ToolEndMessage
	ErrorMessage
)

// Message 表示一条聊天消息
type Message struct {
	Type    MessageType
	Content string
	Name    string // 工具名称（仅用于工具消息）
}

// ViewModel 是Agent界面的模型
type ViewModel struct {
	messages         []Message
	input            string
	cursor           int
	viewport         int
	width            int
	height           int
	isWaiting        bool
	errorMsg         string
	onSendMsg        func(string) error    // 发送消息的回调函数
	streamingContent string                // 当前正在流式接收的内容
	renderer         *glamour.TermRenderer // markdown渲染器
	scrollOffset     int                   // 滚动偏移量，用于上下键滚动
}

// 消息类型定义
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

// NewViewModel 创建新的ViewModel
func NewViewModel(onSendMsg func(string) error) *ViewModel {
	// 创建glamour渲染器 - 与chat界面相同
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

// Init 初始化模型
func (m ViewModel) Init() tea.Cmd {
	return nil // 与chat界面相同，不需要初始化命令
}

// Update 处理消息
func (m ViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.isWaiting {
			// 等待响应时只允许退出
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
				// 添加用户消息
				m.messages = append(m.messages, Message{
					Type:    UserMessage,
					Content: m.input,
				})

				// 发送消息
				userInput := m.input
				m.input = ""
				m.isWaiting = true
				m.streamingContent = ""
				m.errorMsg = ""

				// 调用回调函数发送消息
				if m.onSendMsg != nil {
					go func() {
						if err := m.onSendMsg(userInput); err != nil {
							// 如果发送失败，发送错误消息
							m.errorMsg = fmt.Sprintf("发送消息失败: %v", err)
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
		// 完整响应消息
		m.messages = append(m.messages, Message{
			Type:    AssistantMessage,
			Content: string(msg),
		})
		m.isWaiting = false
		m.streamingContent = ""
		return m, nil

	case StreamChunkMsg:
		// 流式响应块
		m.streamingContent += string(msg)
		return m, nil

	case StreamEndMsg:
		// 流式结束，将流式内容转换为正式消息
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
		// 工具开始执行
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
		// 工具执行结束
		content := fmt.Sprintf("Tool %s completed", msg.Name)
		if msg.Result != "" {
			// 清理结果，移除多余的换行符
			result := strings.TrimSpace(msg.Result)
			if len(result) > 200 {
				// 如果结果太长，截断并添加省略号
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
		// 错误消息 - 直接显示所有错误消息（过滤已在应用层处理）
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

// View 渲染界面
func (m ViewModel) View() string {
	// 样式定义 - 完全参考chat界面
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
		Width(m.width - 4) // 减去边框和内边距的宽度

	// 构建消息显示区域
	var messageLines []string
	messageLines = append(messageLines, "=== Eino CLI Agent ===")
	messageLines = append(messageLines, "")

	// 显示消息历史 - 使用滚动逻辑
	visibleMessages := m.messages
	if m.scrollOffset > 0 && m.scrollOffset < len(m.messages) {
		visibleMessages = m.messages[m.scrollOffset:]
	}

	for _, msg := range visibleMessages {
		switch msg.Type {
		case UserMessage:
			messageLines = append(messageLines, userStyle.Render("You: ")+msg.Content)
		case AssistantMessage:
			// 对AI回复使用markdown渲染
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

	// 显示正在流式接收的内容
	if m.streamingContent != "" {
		// 对流式内容也使用markdown渲染
		renderedStreamContent := m.renderMarkdown(m.streamingContent)
		messageLines = append(messageLines, assistantStyle.Render("AI: ")+renderedStreamContent)
		messageLines = append(messageLines, "")
	}

	// 显示等待状态
	if m.isWaiting {
		messageLines = append(messageLines, "AI is thinking...")
		messageLines = append(messageLines, "")
	}

	// 显示错误信息
	if m.errorMsg != "" {
		messageLines = append(messageLines, errorStyle.Render("Error: "+m.errorMsg))
		messageLines = append(messageLines, "")
	}

	// 限制显示的行数
	maxLines := m.height - 4 // 为输入框和边框留出空间
	if maxLines > 0 && len(messageLines) > maxLines {
		messageLines = messageLines[len(messageLines)-maxLines:]
	}

	messageArea := strings.Join(messageLines, "\n")

	// 构建输入区域
	inputPrompt := "> "
	if m.isWaiting {
		inputPrompt = "Waiting for response... "
	}
	inputArea := inputStyle.Render(inputPrompt + m.input)

	// 构建帮助信息
	helpText := "Press Ctrl+C to quit, ↑/↓ to scroll, Enter to send"

	return fmt.Sprintf("%s\n\n%s\n%s", messageArea, inputArea, helpText)
}

// renderMarkdown 渲染markdown内容 - 与chat界面相同
func (m *ViewModel) renderMarkdown(content string) string {
	if m.renderer == nil {
		return content // 如果渲染器未初始化，返回原始内容
	}

	rendered, err := m.renderer.Render(content)
	if err != nil {
		return content // 如果渲染失败，返回原始内容
	}

	return strings.TrimSpace(rendered)
}

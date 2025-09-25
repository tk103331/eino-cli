package chat

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
	Role    string // "user" 或 "assistant" (保持向后兼容)
	Content string
	Name    string // 工具名称（仅用于工具消息）
}

// ViewModel 是聊天界面的模型
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
}

// NewViewModel 创建新的聊天模型
func NewViewModel(onSendMsg func(string) error) ViewModel {
	// 创建glamour渲染器
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
	)

	return ViewModel{
		messages:         []Message{},
		input:            "",
		cursor:           0,
		viewport:         0,
		width:            80,
		height:           24,
		isWaiting:        false,
		errorMsg:         "",
		onSendMsg:        onSendMsg,
		streamingContent: "",
		renderer:         renderer,
	}
}

// Init 初始化模型
func (m ViewModel) Init() tea.Cmd {
	return nil
}

// Update 处理消息更新
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
			case "ctrl+c", "q":
				return m, tea.Quit
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "enter":
			if strings.TrimSpace(m.input) != "" {
				// 添加用户消息
				userMsg := Message{
					Type:    UserMessage,
					Role:    "user",
					Content: m.input,
				}
				m.messages = append(m.messages, userMsg)

				// 发送消息
				input := m.input
				m.input = ""
				m.isWaiting = true
				m.errorMsg = ""

				// 调用发送消息回调
				if m.onSendMsg != nil {
					go func() {
						if err := m.onSendMsg(input); err != nil {
							// 这里需要通过某种方式将错误传回UI
							// 暂时先忽略，实际使用时需要改进
						}
					}()
				}
			}
			return m, nil

		case "backspace":
			if len(m.input) > 0 {
				// 使用rune来正确处理UTF-8字符的删除
				runes := []rune(m.input)
				if len(runes) > 0 {
					m.input = string(runes[:len(runes)-1])
				}
			}
			return m, nil

		case "up":
			if m.viewport > 0 {
				m.viewport--
			}
			return m, nil

		case "down":
			maxViewport := len(m.messages) - (m.height - 4)
			if maxViewport < 0 {
				maxViewport = 0
			}
			if m.viewport < maxViewport {
				m.viewport++
			}
			return m, nil

		default:
			// 调试：记录所有键盘输入事件
			keyStr := msg.String()

			// 简化过滤逻辑 - 只过滤明确的控制键
			if keyStr == "ctrl+c" || keyStr == "q" {
				return m, tea.Quit
			}

			// 过滤其他控制键但允许所有可见字符
			if strings.HasPrefix(keyStr, "ctrl+") ||
				strings.HasPrefix(keyStr, "alt+") ||
				keyStr == "tab" || keyStr == "esc" {
				return m, nil
			}

			// 直接添加所有其他字符，包括中文
			if keyStr != "" {
				m.input += keyStr
			}
			return m, nil
		}

	case ResponseMsg:
		// 接收到AI完整响应，清空流式内容并结束等待状态
		if m.streamingContent != "" {
			// 如果有流式内容，将其作为最终消息添加
			assistantMsg := Message{
				Type:    AssistantMessage,
				Role:    "assistant",
				Content: m.streamingContent,
			}
			m.messages = append(m.messages, assistantMsg)
			m.streamingContent = ""
		} else {
			// 如果没有流式内容，直接添加完整响应
			assistantMsg := Message{
				Type:    AssistantMessage,
				Role:    "assistant",
				Content: string(msg),
			}
			m.messages = append(m.messages, assistantMsg)
		}
		m.isWaiting = false
		return m, nil

	case StreamChunkMsg:
		// 接收到流式响应增量
		m.streamingContent += string(msg)
		return m, nil

	case ToolStartMsg:
		// 工具开始执行
		m.messages = append(m.messages, Message{
			Type:    ToolStartMessage,
			Content: fmt.Sprintf("🔧 调用工具: %s\n参数: %s", msg.Name, msg.Arguments),
			Name:    msg.Name,
		})
		return m, nil

	case ToolEndMsg:
		// 工具执行结束
		m.messages = append(m.messages, Message{
			Type:    ToolEndMessage,
			Content: fmt.Sprintf("✅ 工具 %s 执行结果:\n%s", msg.Name, msg.Result),
			Name:    msg.Name,
		})
		return m, nil

	case ErrorMsg:
		// 接收到错误消息
		m.errorMsg = string(msg)
		m.isWaiting = false
		// 清空流式内容
		if m.streamingContent != "" {
			assistantMsg := Message{
				Type:    AssistantMessage,
				Role:    "assistant",
				Content: m.streamingContent,
			}
			m.messages = append(m.messages, assistantMsg)
			m.streamingContent = ""
		}
		return m, nil
	}

	return m, nil
}

// renderMarkdown 渲染markdown内容
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

// View 渲染界面
func (m ViewModel) View() string {
	// 样式定义
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
	messageLines = append(messageLines, "=== Eino CLI Chat ===")
	messageLines = append(messageLines, "")

	// 显示消息历史
	visibleMessages := m.messages
	if m.viewport > 0 && m.viewport < len(m.messages) {
		visibleMessages = m.messages[m.viewport:]
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
		default:
			// 向后兼容：基于Role字段处理
			if msg.Role == "user" {
				messageLines = append(messageLines, userStyle.Render("You: ")+msg.Content)
			} else {
				// 对AI回复使用markdown渲染
				renderedContent := m.renderMarkdown(msg.Content)
				messageLines = append(messageLines, assistantStyle.Render("AI: ")+renderedContent)
			}
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
		messageLines = append(messageLines, errorStyle.Render("Error: ")+m.errorMsg)
		messageLines = append(messageLines, "")
	}

	// 限制显示的行数
	maxLines := m.height - 4 // 为输入框和边框留出空间
	if len(messageLines) > maxLines {
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
	helpText := "Press 'q' or Ctrl+C to quit, ↑/↓ to scroll, Enter to send"

	return fmt.Sprintf("%s\n\n%s\n%s", messageArea, inputArea, helpText)
}

// AddMessage 添加消息（保持向后兼容）
func (m *ViewModel) AddMessage(role, content string) {
	var msgType MessageType
	switch role {
	case "user":
		msgType = UserMessage
	case "assistant":
		msgType = AssistantMessage
	default:
		msgType = AssistantMessage
	}
	
	m.messages = append(m.messages, Message{
		Type:    msgType,
		Role:    role,
		Content: content,
	})
}

// SetWaiting 设置等待状态
func (m *ViewModel) SetWaiting(waiting bool) {
	m.isWaiting = waiting
}

// SetError 设置错误信息
func (m *ViewModel) SetError(err string) {
	m.errorMsg = err
	m.isWaiting = false
}

// 消息类型定义
type ResponseMsg string
type StreamChunkMsg string
type ErrorMsg string
type ToolStartMsg struct {
	Name      string
	Arguments string
}
type ToolEndMsg struct {
	Name   string
	Result string
}

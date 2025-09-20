package agent

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// Message 表示一条Agent交互消息
type Message struct {
	Role    string // "user" 或 "assistant"
	Content string
}

// ViewModel 是Agent交互界面的模型
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

// NewViewModel 创建新的Agent交互模型
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
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			if m.input != "" && !m.isWaiting {
				// 添加用户消息
				m.AddMessage("user", m.input)
				userInput := m.input
				m.input = ""
				m.cursor = 0
				m.SetWaiting(true)
				m.SetError("")

				// 发送消息
				if m.onSendMsg != nil {
					go func() {
						if err := m.onSendMsg(userInput); err != nil {
							// 错误会通过ErrorMsg消息发送
						}
					}()
				}
			}
		case "backspace":
			if m.cursor > 0 {
				m.input = m.input[:m.cursor-1] + m.input[m.cursor:]
				m.cursor--
			}
		case "left":
			if m.cursor > 0 {
				m.cursor--
			}
		case "right":
			if m.cursor < len(m.input) {
				m.cursor++
			}
		case "up":
			if m.viewport > 0 {
				m.viewport--
			}
		case "down":
			m.viewport++
		default:
			// 处理普通字符输入
			if !m.isWaiting && len(msg.String()) == 1 {
				char := msg.String()
				m.input = m.input[:m.cursor] + char + m.input[m.cursor:]
				m.cursor++
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case ResponseMsg:
		// 处理完整响应
		m.AddMessage("assistant", string(msg))
		m.SetWaiting(false)
		m.streamingContent = ""

	case StreamChunkMsg:
		// 处理流式响应
		m.streamingContent += string(msg)

	case ErrorMsg:
		// 处理错误消息
		m.SetError(string(msg))
		m.SetWaiting(false)
		m.streamingContent = ""
	}

	return m, nil
}

// renderMarkdown 渲染markdown内容
func (m *ViewModel) renderMarkdown(content string) string {
	if m.renderer == nil {
		return content
	}

	rendered, err := m.renderer.Render(content)
	if err != nil {
		return content
	}
	return rendered
}

// View 渲染视图
func (m ViewModel) View() string {
	var b strings.Builder

	// 标题
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Render("Agent 交互")
	b.WriteString(title + "\n\n")

	// 消息区域
	messageHeight := m.height - 6 // 为输入框和其他UI元素留出空间
	if messageHeight < 1 {
		messageHeight = 1
	}

	messages := make([]string, 0)

	// 显示历史消息
	for _, msg := range m.messages {
		var roleStyle lipgloss.Style
		if msg.Role == "user" {
			roleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
		} else {
			roleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
		}

		role := roleStyle.Render(msg.Role + ": ")
		content := m.renderMarkdown(msg.Content)
		messages = append(messages, role+content)
	}

	// 如果正在等待响应，显示等待状态
	if m.isWaiting {
		waitingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Italic(true)
		if m.streamingContent != "" {
			// 显示流式内容
			assistantStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
			role := assistantStyle.Render("assistant: ")
			content := m.renderMarkdown(m.streamingContent)
			messages = append(messages, role+content)
		} else {
			messages = append(messages, waitingStyle.Render("正在等待Agent响应..."))
		}
	}

	// 应用视口滚动
	startIdx := m.viewport
	if startIdx >= len(messages) {
		startIdx = len(messages) - 1
	}
	if startIdx < 0 {
		startIdx = 0
	}

	endIdx := startIdx + messageHeight
	if endIdx > len(messages) {
		endIdx = len(messages)
	}

	for i := startIdx; i < endIdx; i++ {
		b.WriteString(messages[i] + "\n")
	}

	// 填充空行
	for i := endIdx - startIdx; i < messageHeight; i++ {
		b.WriteString("\n")
	}

	// 错误消息
	if m.errorMsg != "" {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
		b.WriteString(errorStyle.Render("错误: "+m.errorMsg) + "\n")
	}

	// 输入框
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1).
		Width(m.width - 4) // 减去边框和内边距的宽度

	prompt := "> "
	input := m.input
	if m.cursor < len(input) {
		input = input[:m.cursor] + "|" + input[m.cursor:]
	} else {
		input += "|"
	}

	b.WriteString(inputStyle.Render(prompt+input) + "\n")

	// 帮助信息
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	b.WriteString(helpStyle.Render("按 Enter 发送消息，按 q 或 Ctrl+C 退出，↑↓ 滚动消息"))

	return b.String()
}

// AddMessage 添加消息
func (m *ViewModel) AddMessage(role, content string) {
	m.messages = append(m.messages, Message{
		Role:    role,
		Content: content,
	})
}

// SetWaiting 设置等待状态
func (m *ViewModel) SetWaiting(waiting bool) {
	m.isWaiting = waiting
}

// SetError 设置错误消息
func (m *ViewModel) SetError(err string) {
	m.errorMsg = err
}

// ResponseMsg 表示Agent响应消息
type ResponseMsg string

// StreamChunkMsg 表示流式响应的增量消息
type StreamChunkMsg string

// ErrorMsg 表示错误消息
type ErrorMsg string

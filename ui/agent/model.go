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
func NewViewModel(onSendMsg func(string) error) ViewModel {
	// 创建markdown渲染器
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	return ViewModel{
		messages:  []Message{},
		onSendMsg: onSendMsg,
		renderer:  renderer,
	}
}

// Init 初始化模型
func (m ViewModel) Init() tea.Cmd {
	return nil
}

// Update 处理消息
func (m ViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up":
			// 向上滚动
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
			return m, nil
		case "down":
			// 向下滚动
			messageHeight := m.height - 6
			messageLines := []string{}
			for _, msg := range m.messages {
				messageLines = append(messageLines, m.renderMessage(msg)...)
			}
			if m.streamingContent != "" {
				streamMsg := Message{
					Type:    AssistantMessage,
					Content: m.streamingContent,
				}
				messageLines = append(messageLines, m.renderMessage(streamMsg)...)
			}
			
			totalLines := len(messageLines)
			maxOffset := totalLines - messageHeight
			if maxOffset > 0 && m.scrollOffset < maxOffset {
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
		// 错误消息
		m.messages = append(m.messages, Message{
			Type:    ErrorMessage,
			Content: string(msg),
		})
		m.isWaiting = false
		m.streamingContent = ""
		m.errorMsg = string(msg)
		return m, nil
	}

	return m, nil
}

// View 渲染视图
func (m ViewModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "正在初始化..."
	}

	var b strings.Builder

	// 标题
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Render("🤖 AI Agent 对话")
	b.WriteString(title + "\n\n")

	// 消息区域高度
	messageHeight := m.height - 6 // 留出空间给标题、输入框和提示

	// 渲染消息
	messageLines := []string{}
	for _, msg := range m.messages {
		messageLines = append(messageLines, m.renderMessage(msg)...)
	}

	// 如果有流式内容，添加到消息中
	if m.streamingContent != "" {
		streamMsg := Message{
			Type:    AssistantMessage,
			Content: m.streamingContent,
		}
		messageLines = append(messageLines, m.renderMessage(streamMsg)...)
	}

	// 计算需要显示的消息行
	totalLines := len(messageLines)
	startLine := m.scrollOffset
	
	// 如果没有足够的消息行来填充屏幕，自动滚动到底部
	if totalLines <= messageHeight {
		startLine = 0
		m.scrollOffset = 0
	} else {
		// 确保滚动偏移量在有效范围内
		maxOffset := totalLines - messageHeight
		if m.scrollOffset > maxOffset {
			m.scrollOffset = maxOffset
			startLine = maxOffset
		}
		
		// 如果有新消息且当前在底部附近，自动滚动到底部
		if m.scrollOffset >= maxOffset-2 {
			m.scrollOffset = maxOffset
			startLine = maxOffset
		}
	}

	// 显示消息
	for i := startLine; i < totalLines && i-startLine < messageHeight; i++ {
		b.WriteString(messageLines[i] + "\n")
	}

	// 填充空行
	for i := len(messageLines) - startLine; i < messageHeight; i++ {
		b.WriteString("\n")
	}

	// 分隔线
	separator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(strings.Repeat("─", m.width))
	b.WriteString(separator + "\n")

	// 输入框
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1)

	prompt := "💬 输入消息: "
	if m.isWaiting {
		prompt = "⏳ 等待响应... "
		inputStyle = inputStyle.BorderForeground(lipgloss.Color("214"))
	}

	input := inputStyle.Render(prompt + m.input + "█")
	b.WriteString(input + "\n")

	// 帮助信息
	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render("按 Enter 发送消息，按 ↑↓ 滚动消息，按 Ctrl+C 或 q 退出")
	b.WriteString(help)

	return b.String()
}

// renderMessage 渲染单条消息
func (m ViewModel) renderMessage(msg Message) []string {
	var lines []string
	var style lipgloss.Style

	switch msg.Type {
	case UserMessage:
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)
		prefix := "👤 用户: "
		content := style.Render(prefix) + msg.Content
		lines = append(lines, content)

	case AssistantMessage:
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86"))
		prefix := "🤖 助手: "

		// 尝试渲染markdown
		rendered := msg.Content
		if m.renderer != nil {
			if markdownContent, err := m.renderer.Render(msg.Content); err == nil {
				rendered = strings.TrimSpace(markdownContent)
			}
		}

		// 分割成多行
		contentLines := strings.Split(rendered, "\n")
		for i, line := range contentLines {
			if i == 0 {
				lines = append(lines, style.Render(prefix)+line)
			} else {
				lines = append(lines, "     "+line) // 缩进对齐
			}
		}

	case ToolStartMessage:
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)
		lines = append(lines, style.Render(msg.Content))

	case ToolEndMessage:
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46"))
		// 分割成多行
		contentLines := strings.Split(msg.Content, "\n")
		for _, line := range contentLines {
			lines = append(lines, style.Render(line))
		}

	case ErrorMessage:
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)
		prefix := "❌ 错误: "
		lines = append(lines, style.Render(prefix+msg.Content))
	}

	// 添加空行分隔
	lines = append(lines, "")

	return lines
}

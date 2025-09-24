package agent

// 添加导入fmt包
import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// Message 表示一条Agent交互消息
type Message struct {
	Role    string // "user" 或 "assistant"
	Content string
}

// ExecutionStep 表示Agent执行的一个步骤
type ExecutionStep struct {
	Name      string    // 步骤名称
	Status    string    // 状态：pending, running, completed, error
	StartTime time.Time // 开始时间
	EndTime   time.Time // 结束时间
	Output    string    // 输出内容
	Error     string    // 错误信息
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
	
	// 新增字段，用于跟踪执行步骤
	executionSteps   []ExecutionStep       // 执行步骤列表
	currentStep      int                   // 当前执行的步骤索引
	showSteps        bool                  // 是否显示执行步骤
	stepViewport     int                   // 步骤视图的滚动位置
	
	// 步骤详细信息展示
	selectedStep     int                   // 当前选中的步骤索引
	showStepDetails  bool                  // 是否显示步骤详细信息
	detailsViewport  int                   // 详细信息视图的滚动位置
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
		executionSteps:   []ExecutionStep{},
		currentStep:      -1,
		showSteps:        true,
		stepViewport:     0,
		selectedStep:     -1,
		showStepDetails:  false,
		detailsViewport:  0,
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
			if m.showStepDetails && m.selectedStep >= 0 {
				// 在步骤详情视图中，Enter 键返回步骤列表
				m.showStepDetails = false
			} else if m.input != "" && !m.isWaiting {
				// 添加用户消息
				m.AddMessage("user", m.input)
				userInput := m.input
				m.input = ""
				m.cursor = 0
				m.SetWaiting(true)
				m.SetError("")
				
				// 重置执行步骤
				m.executionSteps = []ExecutionStep{}
				m.currentStep = -1
				m.selectedStep = -1
				m.showStepDetails = false

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
			if m.showStepDetails {
				if m.detailsViewport > 0 {
					m.detailsViewport--
				}
			} else if m.viewport > 0 {
				m.viewport--
			}
		case "down":
			if m.showStepDetails {
				m.detailsViewport++
			} else {
				m.viewport++
			}
		case "tab":
			// 切换是否显示执行步骤
			m.showSteps = !m.showSteps
			if !m.showSteps {
				m.showStepDetails = false
			}
		case "esc":
			// ESC 键退出步骤详情视图
			if m.showStepDetails {
				m.showStepDetails = false
			}
		default:
			// 处理数字键选择步骤
			if m.showSteps && !m.showStepDetails && !m.isWaiting && len(msg.String()) == 1 {
				if num, err := strconv.Atoi(msg.String()); err == nil && num > 0 && num <= len(m.executionSteps) {
					m.selectedStep = num - 1
					m.showStepDetails = true
					m.detailsViewport = 0
					return m, nil
				}
			}
			
			// 处理普通字符输入
			if !m.isWaiting && !m.showStepDetails && len(msg.String()) == 1 {
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
		
		// 如果有执行步骤，将最后一个步骤标记为完成
		if len(m.executionSteps) > 0 && m.currentStep >= 0 && m.currentStep < len(m.executionSteps) {
			m.executionSteps[m.currentStep].Status = "completed"
			m.executionSteps[m.currentStep].EndTime = time.Now()
		}

	case StreamChunkMsg:
		// 处理流式响应
		m.streamingContent += string(msg)
		
		// 更新当前步骤的输出
		if m.currentStep >= 0 && m.currentStep < len(m.executionSteps) {
			m.executionSteps[m.currentStep].Output += string(msg)
		}

	case ErrorMsg:
		// 处理错误消息
		m.SetError(string(msg))
		m.SetWaiting(false)
		m.streamingContent = ""
		
		// 如果有执行步骤，将当前步骤标记为错误
		if m.currentStep >= 0 && m.currentStep < len(m.executionSteps) {
			m.executionSteps[m.currentStep].Status = "error"
			m.executionSteps[m.currentStep].Error = string(msg)
			m.executionSteps[m.currentStep].EndTime = time.Now()
		}
		
	case StepStartMsg:
		// 处理步骤开始消息
		step := ExecutionStep{
			Name:      string(msg),
			Status:    "running",
			StartTime: time.Now(),
		}
		m.executionSteps = append(m.executionSteps, step)
		m.currentStep = len(m.executionSteps) - 1
		
	case StepEndMsg:
		// 处理步骤结束消息
		if m.currentStep >= 0 && m.currentStep < len(m.executionSteps) {
			m.executionSteps[m.currentStep].Status = "completed"
			m.executionSteps[m.currentStep].EndTime = time.Now()
		}
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

	// 如果显示步骤详细信息，则渲染详细视图
	if m.showStepDetails && m.selectedStep >= 0 && m.selectedStep < len(m.executionSteps) {
		return m.renderStepDetailsView(&b)
	}

	// 计算消息区域和步骤区域的高度
	messageHeight := m.height - 6 // 为输入框和其他UI元素留出空间
	if m.showSteps && len(m.executionSteps) > 0 {
		// 如果显示执行步骤，分配一部分空间给步骤区域
		messageHeight = (m.height - 6) * 2 / 3 // 消息区域占2/3，步骤区域占1/3
	}
	if messageHeight < 1 {
		messageHeight = 1
	}

	// 渲染消息区域
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

	// 如果显示执行步骤，渲染步骤区域
	if m.showSteps && len(m.executionSteps) > 0 {
		stepHeight := (m.height - 6) - messageHeight
		if stepHeight < 1 {
			stepHeight = 1
		}

		// 步骤区域标题和分隔线
		stepTitle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("99")).
			Render("执行步骤")
		
		// 添加分隔线
		divider := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render(strings.Repeat("─", m.width - 4))
		
		b.WriteString(divider + "\n")
		b.WriteString(stepTitle + "\n")

		// 渲染步骤列表
		steps := make([]string, 0)
		for i, step := range m.executionSteps {
			var statusStyle lipgloss.Style
			var statusIcon string
			
			switch step.Status {
			case "pending":
				statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
				statusIcon = "⏳"
			case "running":
				statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
				statusIcon = "🔄"
			case "completed":
				statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
				statusIcon = "✅"
			case "error":
				statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
				statusIcon = "❌"
			}
			
			// 计算执行时间
			var duration string
			if !step.EndTime.IsZero() {
				duration = step.EndTime.Sub(step.StartTime).Round(time.Millisecond).String()
			} else if !step.StartTime.IsZero() {
				duration = time.Since(step.StartTime).Round(time.Millisecond).String()
			}
			
			// 步骤标题
			stepLine := statusStyle.Render(fmt.Sprintf("%s %d. %s", statusIcon, i+1, step.Name))
			if duration != "" {
				stepLine += statusStyle.Render(fmt.Sprintf(" (%s)", duration))
			}
			
			// 添加提示，表示可以按数字键查看详情
			detailsHint := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(" [按" + strconv.Itoa(i+1) + "查看详情]")
			
			// 计算可用宽度，确保提示不会导致行过长
			availableWidth := m.width - lipgloss.Width(stepLine) - lipgloss.Width(detailsHint) - 4
			if availableWidth > 0 {
				stepLine += detailsHint
			}
			
			steps = append(steps, stepLine)
			
			// 如果是当前步骤且有输出，显示输出预览
			if i == m.currentStep && step.Status == "running" && step.Output != "" {
				outputStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
				outputLines := strings.Split(step.Output, "\n")
				if len(outputLines) > 3 {
					// 只显示最后3行
					outputLines = outputLines[len(outputLines)-3:]
				}
				for _, line := range outputLines {
					if line != "" {
						// 截断过长的行
						if len(line) > m.width - 6 {
							line = line[:m.width-9] + "..."
						}
						steps = append(steps, outputStyle.Render("  "+line))
					}
				}
			}
			
			// 如果有错误，显示错误信息预览
			if step.Status == "error" && step.Error != "" {
				errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Italic(true)
				errorMsg := step.Error
				// 截断过长的错误信息
				if len(errorMsg) > m.width - 10 {
					errorMsg = errorMsg[:m.width-13] + "..."
				}
				steps = append(steps, errorStyle.Render("  错误: "+errorMsg))
			}
		}
		
		// 应用步骤视口滚动
		stepStartIdx := m.stepViewport
		if stepStartIdx >= len(steps) {
			stepStartIdx = len(steps) - 1
		}
		if stepStartIdx < 0 {
			stepStartIdx = 0
		}

		stepEndIdx := stepStartIdx + stepHeight
		if stepEndIdx > len(steps) {
			stepEndIdx = len(steps)
		}

		for i := stepStartIdx; i < stepEndIdx; i++ {
			b.WriteString(steps[i] + "\n")
		}

		// 填充空行
		for i := stepEndIdx - stepStartIdx; i < stepHeight; i++ {
			b.WriteString("\n")
		}
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
	helpText := "按 Enter 发送消息，按 q 或 Ctrl+C 退出，↑↓ 滚动消息"
	if len(m.executionSteps) > 0 {
		helpText += "，按 Tab 切换执行步骤显示"
		if m.showSteps {
			helpText += "，按数字键查看步骤详情"
		}
	}
	b.WriteString(helpStyle.Render(helpText))

	return b.String()
}

// renderStepDetailsView 渲染步骤详细信息视图
func (m ViewModel) renderStepDetailsView(b *strings.Builder) string {
	step := m.executionSteps[m.selectedStep]
	
	// 清空已有内容
	*b = strings.Builder{}
	
	// 标题
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Render("步骤详细信息")
	b.WriteString(title + "\n\n")
	
	// 步骤状态和图标
	var statusStyle lipgloss.Style
	var statusIcon string
	var statusText string
	
	switch step.Status {
	case "pending":
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		statusIcon = "⏳"
		statusText = "等待中"
	case "running":
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
		statusIcon = "🔄"
		statusText = "执行中"
	case "completed":
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
		statusIcon = "✅"
		statusText = "已完成"
	case "error":
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
		statusIcon = "❌"
		statusText = "错误"
	}
	
	// 步骤标题
	stepTitle := statusStyle.Render(fmt.Sprintf("%s 步骤 %d: %s (%s)", 
		statusIcon, m.selectedStep+1, step.Name, statusText))
	b.WriteString(stepTitle + "\n")
	
	// 添加分隔线
	divider := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(strings.Repeat("─", m.width - 4))
	b.WriteString(divider + "\n\n")
	
	// 步骤时间信息
	timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	b.WriteString(timeStyle.Render("开始时间: " + step.StartTime.Format("15:04:05.000")) + "\n")
	
	if !step.EndTime.IsZero() {
		b.WriteString(timeStyle.Render("结束时间: " + step.EndTime.Format("15:04:05.000")) + "\n")
		duration := step.EndTime.Sub(step.StartTime).Round(time.Millisecond)
		b.WriteString(timeStyle.Render("执行时间: " + duration.String()) + "\n")
	} else if step.Status == "running" {
		duration := time.Since(step.StartTime).Round(time.Millisecond)
		b.WriteString(timeStyle.Render("已执行时间: " + duration.String()) + "\n")
	}
	b.WriteString("\n")
	
	// 计算内容区域高度
	contentHeight := m.height - 15 // 为标题、时间信息、输入框和其他UI元素留出空间
	if contentHeight < 1 {
		contentHeight = 1
	}
	
	// 如果有错误，显示错误信息
	if step.Status == "error" && step.Error != "" {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
		b.WriteString(errorStyle.Render("错误信息:\n"))
		
		// 分割错误信息为行，处理可能的长行
		errorLines := strings.Split(step.Error, "\n")
		for _, line := range errorLines {
			// 处理长行
			for len(line) > m.width - 4 {
				b.WriteString(errorStyle.Render(line[:m.width-4]) + "\n")
				line = line[m.width-4:]
			}
			b.WriteString(errorStyle.Render(line) + "\n")
		}
		b.WriteString("\n")
	}
	
	// 显示完整输出
	if step.Output != "" {
		outputStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
		b.WriteString(outputStyle.Render("完整输出:") + "\n")
		
		// 添加输出区域边框
		outputBorder := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1).
			Width(m.width - 6)
		
		// 分割输出内容为行
		outputLines := strings.Split(step.Output, "\n")
		
		// 应用视口滚动
		startIdx := m.detailsViewport
		if startIdx >= len(outputLines) {
			startIdx = len(outputLines) - 1
		}
		if startIdx < 0 {
			startIdx = 0
		}
		
		endIdx := startIdx + contentHeight
		if endIdx > len(outputLines) {
			endIdx = len(outputLines)
		}
		
		// 构建可见的输出内容
		visibleOutput := ""
		for i := startIdx; i < endIdx; i++ {
			visibleOutput += outputLines[i] + "\n"
		}
		
		// 渲染带边框的输出
		b.WriteString(outputBorder.Render(visibleOutput))
		
		// 显示滚动指示器
		if len(outputLines) > contentHeight {
			scrollPercent := float64(startIdx) / float64(len(outputLines)-contentHeight)
			scrollBarStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
			scrollBar := fmt.Sprintf("-- 滚动位置: %.0f%% (共 %d 行) --", scrollPercent*100, len(outputLines))
			b.WriteString("\n" + scrollBarStyle.Render(scrollBar) + "\n")
		}
	} else {
		emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
		b.WriteString(emptyStyle.Render("(无输出内容)") + "\n")
	}
	
	// 填充空行
	for i := 0; i < 3; i++ {
		b.WriteString("\n")
	}
	
	// 帮助信息
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	helpText := "按 Enter 或 ESC 返回，按 ↑↓ 滚动输出内容"
	b.WriteString(helpStyle.Render(helpText))
	
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
	
	// 如果有执行步骤，将当前步骤标记为错误
	if m.currentStep >= 0 && m.currentStep < len(m.executionSteps) {
		m.executionSteps[m.currentStep].Status = "error"
		m.executionSteps[m.currentStep].Error = err
		m.executionSteps[m.currentStep].EndTime = time.Now()
	}
}

// AddStep 添加执行步骤
func (m *ViewModel) AddStep(name string) int {
	step := ExecutionStep{
		Name:      name,
		Status:    "pending",
		StartTime: time.Now(),
	}
	m.executionSteps = append(m.executionSteps, step)
	return len(m.executionSteps) - 1
}

// StartStep 开始执行步骤
func (m *ViewModel) StartStep(index int) {
	if index >= 0 && index < len(m.executionSteps) {
		m.executionSteps[index].Status = "running"
		m.executionSteps[index].StartTime = time.Now()
		m.currentStep = index
	}
}

// CompleteStep 完成执行步骤
func (m *ViewModel) CompleteStep(index int) {
	if index >= 0 && index < len(m.executionSteps) {
		m.executionSteps[index].Status = "completed"
		m.executionSteps[index].EndTime = time.Now()
	}
}

// FailStep 标记步骤执行失败
func (m *ViewModel) FailStep(index int, err string) {
	if index >= 0 && index < len(m.executionSteps) {
		m.executionSteps[index].Status = "error"
		m.executionSteps[index].Error = err
		m.executionSteps[index].EndTime = time.Now()
	}
}

// AddStepOutput 添加步骤输出
func (m *ViewModel) AddStepOutput(index int, output string) {
	if index >= 0 && index < len(m.executionSteps) {
		m.executionSteps[index].Output += output
	}
}

// ResponseMsg 表示Agent响应消息
type ResponseMsg string

// StreamChunkMsg 表示流式响应的增量消息
type StreamChunkMsg string

// ErrorMsg 表示错误消息
type ErrorMsg string

// StepStartMsg 表示步骤开始消息
type StepStartMsg string

// StepEndMsg 表示步骤结束消息
type StepEndMsg string

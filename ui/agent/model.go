package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	glamouransi "github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// MessageType represents the type of a message
type MessageType int

const (
	UserMessage MessageType = iota
	AssistantMessage
	ToolStartMessage
	ToolEndMessage
	ErrorMessage
	ReasoningMessage
	InfoMessage
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
	mdCache    string     // Cached markdown rendering (assistant messages)
}

// ViewModel is the model for the Agent interface
type ViewModel struct {
	messages         []Message
	input            string
	cursor           int  // Input cursor position (rune index)
	width            int  // Terminal width
	height           int  // Terminal height
	isWaiting        bool // Waiting for AI response
	errorMsg         string
	subtitle         string             // Header subtitle (agent or model name)
	onSendMsg        func(string) error // Callback for sending messages
	streamingContent string             // Currently streaming content
	reasoningContent string             // Currently streaming model reasoning
	renderer         *glamour.TermRenderer
	renderWidth      int  // Width the renderer was created for
	renderDark       bool // Theme the renderer was created for

	// Collapsible blocks: reasoning and tool call details. Indices refer to
	// positions in m.messages.
	expanded map[int]bool
	selected int // selected collapsible message index (-1 = none)

	// Host-registered slash commands (agent/model/skill switching etc.)
	slashCommands []SlashCommand
	onSlash       SlashHandler
	paletteSel    int // selected row in the slash command palette

	// Color theme state
	themeName string
	darkMode  bool
	theme     ThemeVariant

	// Ctrl+C confirmation: the first press only arms quit.
	confirmQuit bool

	// Mouse capture toggle (off by default so text selection works).
	onToggleMouse func() bool

	// Efficient line-based rendering cache: rebuilt only when content
	// changes (dirty) and at most once per renderInterval while streaming.
	dirty          bool
	lastRender     time.Time
	renderedLines  []string
	scrollOffset   int // Lines from the newest end (0 = pinned to bottom)
	spinnerFrame   int
	spinnerStarted bool
}

// Message type definitions
type ResponseMsg string
type StreamChunkMsg string
type ReasoningChunkMsg string
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

// spinnerTickMsg drives the waiting spinner animation
type spinnerTickMsg time.Time

// confirmQuitTimeoutMsg cancels a pending Ctrl+C quit confirmation.
type confirmQuitTimeoutMsg struct{}

// confirmQuitTimeout is how long the "press Ctrl+C again" hint stays armed.
const confirmQuitTimeout = 3 * time.Second

// confirmQuitTick schedules the confirmation timeout.
func confirmQuitTick() tea.Cmd {
	return tea.Tick(confirmQuitTimeout, func(time.Time) tea.Msg {
		return confirmQuitTimeoutMsg{}
	})
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// renderInterval throttles expensive markdown re-rendering during streaming.
// The cost of a glamour render grows with the accumulated content, so the
// interval adapts to keep the UI responsive on long outputs.
const (
	baseRenderInterval   = 50 * time.Millisecond
	streamRenderInterval = 150 * time.Millisecond
	largeRenderInterval  = 400 * time.Millisecond
)

func renderIntervalFor(contentLen int) time.Duration {
	switch {
	case contentLen < 4*1024:
		return baseRenderInterval
	case contentLen < 32*1024:
		return streamRenderInterval
	default:
		return largeRenderInterval
	}
}

// markDirty flags the line cache as stale and forces the next View pass to
// rebuild it immediately. Used for state transitions; streaming chunks use
// m.dirty directly so View can throttle the expensive markdown re-render.
func (m *ViewModel) markDirty() {
	m.dirty = true
	m.lastRender = time.Time{}
}

// clearChat empties the conversation, keeping the input intact.
func (m *ViewModel) clearChat() {
	m.messages = nil
	m.streamingContent = ""
	m.reasoningContent = ""
	m.errorMsg = ""
	m.scrollOffset = 0
	m.expanded = make(map[int]bool)
	m.selected = -1
	m.markDirty()
}

// SetMouseToggle registers the callback used by Ctrl+Y to toggle mouse
// capture. It returns the new state (true = mouse captured).
func (m *ViewModel) SetMouseToggle(fn func() bool) {
	m.onToggleMouse = fn
}

// SetSubtitle sets the header subtitle (agent or model name).
func (m *ViewModel) SetSubtitle(subtitle string) {
	m.subtitle = strings.TrimSpace(subtitle)
}

// NewViewModel creates a new ViewModel
func NewViewModel(onSendMsg func(string) error) *ViewModel {
	dark := lipgloss.HasDarkBackground()
	return &ViewModel{
		messages:  []Message{},
		onSendMsg: onSendMsg,
		dirty:     true,
		expanded:  make(map[int]bool),
		selected:  -1,
		themeName: defaultThemeName,
		darkMode:  dark,
		theme:     resolveTheme(defaultThemeName, dark),
	}
}

// ensureRenderer creates (or recreates) the markdown renderer for a width
// and terminal theme.
func (m *ViewModel) ensureRenderer() {
	width := m.width - 10
	if width < 40 {
		width = 40
	}
	dark := m.darkMode
	if m.renderer != nil && m.renderWidth == width && m.renderDark == dark {
		return
	}

	// Base on the builtin dark/light style matching the terminal, then fix
	// readability and density:
	//   - document color ("252"/"0" grays) removed so body text inherits the
	//     terminal foreground and stays readable on any theme;
	//   - document margin (2) and surrounding newlines removed — message
	//     spacing is handled by the message renderer itself;
	//   - code block margin tightened.
	cfg := styles.DarkStyleConfig
	if !dark {
		cfg = styles.LightStyleConfig
	}
	cfg.Document.Color = nil
	cfg.Document.BlockPrefix = ""
	cfg.Document.BlockSuffix = ""
	cfg.Document.Margin = nil
	cfg.CodeBlock.Margin = nil

	// Headings and links follow the active theme.
	primary := m.theme.Primary
	accent := m.theme.Accent
	for _, h := range []*glamouransi.StyleBlock{
		&cfg.Heading, &cfg.H1, &cfg.H2, &cfg.H3, &cfg.H4, &cfg.H5, &cfg.H6,
	} {
		h.Color = &primary
	}
	cfg.Link.Color = &accent
	cfg.LinkText.Color = &accent

	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(cfg),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		m.renderer = nil
		return
	}
	m.renderer = renderer
	m.renderWidth = width
	m.renderDark = dark
	// Width/theme change invalidates cached renders and the line cache.
	for i := range m.messages {
		m.messages[i].mdCache = ""
	}
	m.markDirty()
}

// sendMsgCmd runs the send callback and surfaces failures as an ErrorMsg.
func sendMsgCmd(text string, send func(string) error) tea.Cmd {
	return func() tea.Msg {
		if send == nil {
			return nil
		}
		if err := send(text); err != nil {
			return ErrorMsg(fmt.Sprintf("Failed to send message: %v", err))
		}
		return nil
	}
}

// spinnerTickCmd schedules the next spinner frame.
func spinnerTickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg {
		return spinnerTickMsg(t)
	})
}

// Init initializes the model
func (m *ViewModel) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m *ViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureRenderer()
		m.markDirty()
		// A resize can leave stale cells behind (lines wrapped at the old
		// width); force a clean repaint.
		return m, tea.Batch(m.clampScroll(), func() tea.Msg { return tea.ClearScreen() })

	case spinnerTickMsg:
		if m.isWaiting {
			m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
			return m, spinnerTickCmd()
		}
		m.spinnerStarted = false
		return m, nil

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.scrollOffset += 3
			return m, m.clampScroll()
		case tea.MouseButtonWheelDown:
			m.scrollOffset -= 3
			return m, m.clampScroll()
		}
		return m, nil

	case confirmQuitTimeoutMsg:
		if m.confirmQuit {
			m.confirmQuit = false
			m.markDirty()
		}
		return m, nil

	case tea.KeyMsg:
		// Ctrl+C quits only after a second confirmation press.
		if msg.Type == tea.KeyCtrlC {
			if m.confirmQuit {
				return m, tea.Quit
			}
			m.confirmQuit = true
			m.markDirty()
			return m, confirmQuitTick()
		}
		if m.confirmQuit {
			// Any other key cancels the pending quit.
			m.confirmQuit = false
			m.markDirty()
		}

		// Esc never quits: it only dismisses the command palette/input.
		if msg.Type == tea.KeyEsc {
			if m.paletteActive() {
				m.input = ""
				m.cursor = 0
				m.paletteSel = 0
				m.markDirty()
			}
			return m, nil
		}

		// Ctrl+Y toggles mouse capture. With capture off the terminal's
		// native text selection / copy works; with capture on the wheel
		// scrolls the transcript.
		if msg.Type == tea.KeyCtrlY {
			if m.onToggleMouse != nil {
				if m.onToggleMouse() {
					m.appendCommandMessage("鼠标捕获已启用：滚轮可滚动对话；复制可在拖选时按住 Shift（多数终端），或再按 Ctrl+Y 关闭捕获", false)
				} else {
					m.appendCommandMessage("鼠标捕获已关闭：可直接用鼠标选择并复制文本；滚轮由终端转发（Ctrl+Y 可强制启用鼠标滚动）", false)
				}
			}
			return m, nil
		}

		// Scrolling works in all states, including while waiting for a
		// response — long streaming output can overflow the viewport.
		switch msg.Type {
		case tea.KeyUp:
			if m.hasPalette() {
				m.movePalette(-1)
				return m, nil
			}
			m.scrollOffset++
			return m, m.clampScroll()
		case tea.KeyDown:
			if m.hasPalette() {
				m.movePalette(1)
				return m, nil
			}
			m.scrollOffset--
			return m, m.clampScroll()
		case tea.KeyPgUp:
			m.scrollOffset += m.viewportHeight()
			return m, m.clampScroll()
		case tea.KeyPgDown:
			m.scrollOffset -= m.viewportHeight()
			return m, m.clampScroll()
		case tea.KeyHome:
			m.scrollOffset = 1 << 30
			return m, m.clampScroll()
		case tea.KeyEnd:
			m.scrollOffset = 0
			return m, nil
		}

		if m.isWaiting {
			// While waiting only scroll and fold keys are accepted;
			// editing is blocked.
			switch msg.Type {
			case tea.KeyTab:
				m.selectCollapsible(1)
				return m, nil
			case tea.KeyShiftTab:
				m.selectCollapsible(-1)
				return m, nil
			case tea.KeyCtrlO:
				m.toggleSelectedBlock()
				return m, nil
			}
			return m, nil
		}

		switch msg.Type {
		case tea.KeyTab:
			m.selectCollapsible(1)
			return m, nil
		case tea.KeyShiftTab:
			m.selectCollapsible(-1)
			return m, nil
		case tea.KeyCtrlO:
			m.toggleSelectedBlock()
			return m, nil
		case tea.KeyCtrlD:
			// Quit like a shell exit when the input is empty
			if m.input == "" {
				return m, tea.Quit
			}

		case tea.KeyCtrlL:
			m.clearChat()
			return m, nil

		case tea.KeyLeft:
			if m.cursor > 0 {
				m.cursor--
			}
		case tea.KeyRight:
			if m.cursor < len([]rune(m.input)) {
				m.cursor++
			}
		case tea.KeyCtrlA:
			m.cursor = 0
		case tea.KeyCtrlE:
			m.cursor = len([]rune(m.input))
		case tea.KeyCtrlU:
			m.deleteBeforeCursor()
		case tea.KeyCtrlK:
			m.deleteAfterCursor()
		case tea.KeyCtrlW:
			m.deleteWord()

		case tea.KeyEnter:
			if m.input == "" {
				return m, nil
			}

			// Slash commands: /help, /clear, /agent, /model, /skill ...
			if strings.HasPrefix(strings.TrimSpace(m.input), "/") {
				if item, ok := m.selectedPaletteItem(); ok {
					if item.exec {
						m.input = ""
						m.cursor = 0
						m.paletteSel = 0
						m.executeSlashCommand(item.fill)
					} else {
						// Fill the command and continue typing its argument.
						m.input = item.fill
						m.cursor = len([]rune(item.fill))
						m.paletteSel = 0
						m.markDirty()
					}
					return m, m.clampScroll()
				}
				line := strings.TrimSpace(m.input)
				m.input = ""
				m.cursor = 0
				m.paletteSel = 0
				m.executeSlashCommand(line)
				return m, nil
			}

			m.messages = append(m.messages, Message{
				Type:    UserMessage,
				Content: m.input,
			})

			userInput := m.input
			m.input = ""
			m.cursor = 0
			m.isWaiting = true
			m.streamingContent = ""
			m.reasoningContent = ""
			m.errorMsg = ""
			m.scrollOffset = 0
			m.markDirty()

			cmds := []tea.Cmd{sendMsgCmd(userInput, m.onSendMsg)}
			if !m.spinnerStarted {
				m.spinnerStarted = true
				cmds = append(cmds, spinnerTickCmd())
			}
			return m, tea.Batch(cmds...)

		case tea.KeyBackspace:
			m.backspace()
		case tea.KeyDelete:
			m.deleteAtCursor()
		case tea.KeySpace:
			m.insertRune(' ')
		case tea.KeyRunes:
			if !m.isWaiting {
				for _, r := range msg.Runes {
					// Pasted multi-line text collapses to spaces: the
					// single-line input box cannot show newlines.
					if r == '\n' || r == '\r' {
						r = ' '
					}
					m.insertRune(r)
				}
			}
		}
		return m, nil

	case ResponseMsg:
		// Complete response message; keep any reasoning that preceded it
		m.flushReasoning()
		m.messages = append(m.messages, Message{
			Type:    AssistantMessage,
			Content: string(msg),
		})
		m.isWaiting = false
		m.streamingContent = ""
		m.scrollOffset = 0
		m.markDirty()
		m.lastRender = time.Time{} // Render immediately
		return m, nil

	case StreamChunkMsg:
		// Streaming response chunk
		m.streamingContent += string(msg)
		m.dirty = true // View throttles regeneration to renderInterval
		return m, nil

	case ReasoningChunkMsg:
		// Streaming model reasoning (thinking) chunk
		m.reasoningContent += string(msg)
		m.dirty = true
		return m, nil

	case StreamEndMsg:
		// Stream ended: persist reasoning, then the assistant answer
		m.flushReasoning()
		if m.streamingContent != "" {
			m.messages = append(m.messages, Message{
				Type:    AssistantMessage,
				Content: m.streamingContent,
			})
			m.streamingContent = ""
		}
		m.isWaiting = false
		m.scrollOffset = 0
		m.markDirty()
		m.lastRender = time.Time{}
		return m, nil

	case ToolStartMsg:
		// Persist any pending reasoning first so the thinking block stays
		// above the tool call it led to.
		m.flushReasoning()

		// Tool execution started - create waiting state message with timestamp.
		// The eino callback layer can emit the same start event more than
		// once (possibly interleaved with other events); skip duplicates
		// or a zombie "running..." line would stay behind after completion.
		for i := len(m.messages) - 1; i >= 0; i-- {
			msg2 := m.messages[i]
			if msg2.Type == ToolStartMessage && msg2.ToolStatus != ToolWaiting {
				break // A finished tool call marks the end of the last batch
			}
			if msg2.Type == ToolStartMessage &&
				msg2.Name == msg.Name &&
				msg2.Arguments == msg.Arguments {
				return m, nil
			}
		}
		m.messages = append(m.messages, Message{
			Type:       ToolStartMessage,
			Name:       msg.Name,
			ToolStatus: ToolWaiting,
			Arguments:  msg.Arguments,
			StartTime:  time.Now().Unix(),
		})
		m.markDirty()
		return m, nil

	case ToolEndMsg:
		// Tool execution ended - update every waiting message for this tool
		// (duplicate start events from the eino callback layer describe the
		// same call, so they all get the same final status).
		toolResult := strings.TrimSpace(msg.Result)

		newStatus := ToolSuccess
		if looksLikeError(toolResult) {
			newStatus = ToolError
		}

		updated := false
		for i := len(m.messages) - 1; i >= 0; i-- {
			msg2 := m.messages[i]
			if msg2.Type != ToolStartMessage || msg2.ToolStatus != ToolWaiting {
				continue
			}
			if msg2.Name != msg.Name {
				continue
			}
			// Complete every waiting call for this tool: duplicates from the
			// callback layer must not leave zombie "running..." lines behind.
			m.messages[i].ToolStatus = newStatus
			m.messages[i].Result = toolResult
			m.messages[i].EndTime = time.Now().Unix()
			updated = true
		}

		if updated {
			m.markDirty()
			return m, nil
		}

		// If no waiting tool message was found, create a new one
		m.messages = append(m.messages, Message{
			Type:       ToolStartMessage,
			Name:       msg.Name,
			ToolStatus: newStatus,
			Result:     toolResult,
		})
		m.markDirty()
		return m, nil

	case ErrorMsg:
		// Error message - display all error messages (filtering handled at application layer)
		errorText := string(msg)
		m.flushReasoning()
		m.messages = append(m.messages, Message{
			Type:    ErrorMessage,
			Content: errorText,
		})
		m.isWaiting = false
		m.streamingContent = ""
		m.errorMsg = errorText
		m.markDirty()
		m.lastRender = time.Time{}
		return m, nil
	}

	return m, nil
}

// ---- collapsible block helpers ----

// collapsibleIndices lists message indices that can be folded (reasoning and
// tool call blocks) in conversation order.
func (m *ViewModel) collapsibleIndices() []int {
	var idx []int
	for i, msg := range m.messages {
		if msg.Type == ReasoningMessage || msg.Type == ToolStartMessage {
			idx = append(idx, i)
		}
	}
	return idx
}

// selectCollapsible moves the selection through foldable blocks.
func (m *ViewModel) selectCollapsible(dir int) {
	idx := m.collapsibleIndices()
	if len(idx) == 0 {
		return
	}
	if m.selected < 0 {
		if dir >= 0 {
			m.selected = idx[0]
		} else {
			m.selected = idx[len(idx)-1]
		}
		m.markDirty()
		return
	}
	pos := 0
	for i, v := range idx {
		if v == m.selected {
			pos = i
			break
		}
	}
	pos = (pos + dir + len(idx)) % len(idx)
	m.selected = idx[pos]
	m.markDirty()
}

// toggleSelectedBlock expands or folds the selected block.
func (m *ViewModel) toggleSelectedBlock() {
	if m.selected < 0 || m.selected >= len(m.messages) {
		m.selected = -1
		return
	}
	// Reasoning blocks default to folded once finalized; tools default to
	// folded always. Toggling flips the current state.
	m.expanded[m.selected] = !m.expanded[m.selected]
	m.markDirty()
}

// ---- input editing helpers ----

func (m *ViewModel) insertRune(r rune) {
	m.paletteSel = 0
	runes := []rune(m.input)
	if m.cursor > len(runes) {
		m.cursor = len(runes)
	}
	runes = append(runes[:m.cursor], append([]rune{r}, runes[m.cursor:]...)...)
	m.input = string(runes)
	m.cursor++
}

func (m *ViewModel) backspace() {
	m.paletteSel = 0
	runes := []rune(m.input)
	if m.cursor > 0 && m.cursor <= len(runes) {
		runes = append(runes[:m.cursor-1], runes[m.cursor:]...)
		m.input = string(runes)
		m.cursor--
	}
}

func (m *ViewModel) deleteAtCursor() {
	m.paletteSel = 0
	runes := []rune(m.input)
	if m.cursor < len(runes) {
		runes = append(runes[:m.cursor], runes[m.cursor+1:]...)
		m.input = string(runes)
	}
}

func (m *ViewModel) deleteBeforeCursor() {
	m.paletteSel = 0
	runes := []rune(m.input)
	if m.cursor > len(runes) {
		m.cursor = len(runes)
	}
	m.input = string(runes[m.cursor:])
	m.cursor = 0
}

func (m *ViewModel) deleteAfterCursor() {
	m.paletteSel = 0
	runes := []rune(m.input)
	if m.cursor > len(runes) {
		m.cursor = len(runes)
	}
	m.input = string(runes[:m.cursor])
}

func (m *ViewModel) deleteWord() {
	m.paletteSel = 0
	runes := []rune(m.input)
	if m.cursor > len(runes) {
		m.cursor = len(runes)
	}
	i := m.cursor
	for i > 0 && unicode.IsSpace(runes[i-1]) {
		i--
	}
	for i > 0 && !unicode.IsSpace(runes[i-1]) {
		i--
	}
	runes = append(runes[:i], runes[m.cursor:]...)
	m.input = string(runes)
	m.cursor = i
}

// ---- scroll helpers ----

// paletteView returns the palette rows to render, capped so the layout can
// never exceed the terminal height.
func (m *ViewModel) paletteView() []string {
	hints := m.slashHints()
	maxLines := m.height - 8 // keep room for header, rule, help, input box
	if maxLines < 0 {
		maxLines = 0
	}
	if len(hints) > maxLines {
		hints = hints[:maxLines]
	}
	return hints
}

func (m *ViewModel) viewportHeight() int {
	// Fixed chrome: header(1) + rule(1) + help bar(1) + input box(3).
	h := m.height - 6 - len(m.paletteView())
	if h < 1 {
		h = 1
	}
	return h
}

// clampScroll is a tea.Cmd that keeps the scroll offset within bounds.
func (m *ViewModel) clampScroll() tea.Cmd {
	maxScroll := len(m.renderedLines) - m.viewportHeight()
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scrollOffset > maxScroll {
		m.scrollOffset = maxScroll
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	return nil
}

// looksLikeError detects common error patterns in tool results.
func looksLikeError(result string) bool {
	lower := strings.ToLower(result)
	patterns := []string{
		"error", "failed", "exception", "timeout",
		"not found", "invalid", "unauthorized",
		"forbidden", "denied", "panic", "fatal",
	}
	for _, pattern := range patterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	for _, prefix := range []string{"❌", "⚠️", "❗", "✖️", "×"} {
		if strings.HasPrefix(result, prefix) {
			return true
		}
	}
	return false
}

// ---- rendering ----

// refreshCache regenerates the rendered line cache when the content changed,
// throttled during streaming to keep markdown rendering cheap.
func (m *ViewModel) refreshCache() {
	if !m.dirty {
		return
	}
	interval := renderIntervalFor(len(m.streamingContent))
	if !m.lastRender.IsZero() && time.Since(m.lastRender) < interval {
		return // Too soon; the next View pass will pick it up
	}
	m.updateRenderedLines()
	m.dirty = false
	m.lastRender = time.Now()
}

// updateRenderedLines rebuilds the cached rendered lines for efficient scrolling.
func (m *ViewModel) updateRenderedLines() {
	m.ensureRenderer()
	m.renderedLines = m.renderedLines[:0]

	for i := range m.messages {
		msg := &m.messages[i]
		switch msg.Type {
		case UserMessage:
			m.renderedLines = append(m.renderedLines, m.renderUserMessage(msg.Content)...)
		case AssistantMessage:
			m.renderedLines = append(m.renderedLines, m.renderAssistantMessage(msg)...)
		case ReasoningMessage:
			m.renderedLines = append(m.renderedLines, m.renderReasoningMessage(msg, i, false)...)
		case ToolStartMessage:
			m.renderedLines = append(m.renderedLines, m.renderToolMessage(msg, i)...)
		case ErrorMessage:
			m.renderedLines = append(m.renderedLines, m.renderErrorMessage(msg.Content)...)
		case InfoMessage:
			m.renderedLines = append(m.renderedLines, m.renderInfoMessage(msg.Content)...)
		}
	}

	// Streaming content: reasoning first (expanded while running), then the answer
	if m.reasoningContent != "" {
		live := &Message{Type: ReasoningMessage, Content: m.reasoningContent}
		m.renderedLines = append(m.renderedLines, m.renderReasoningMessage(live, -1, true)...)
	}
	if m.streamingContent != "" {
		lines := m.renderAssistantContent(m.streamingContent, true)
		m.renderedLines = append(m.renderedLines, lines...)
	} else if m.isWaiting && m.reasoningContent == "" {
		// Spinner only shows while nothing streams yet — otherwise the
		// "(typing…)" label already indicates progress.
		spinner := m.warningStyle(true).Render(spinnerFrames[m.spinnerFrame])
		m.renderedLines = append(m.renderedLines,
			m.muted().Render("  "+spinner+" thinking..."), "")
	}

	// System error (avoid duplicating a message that is already shown)
	if m.errorMsg != "" {
		dup := false
		for _, msg := range m.messages {
			if msg.Type == ErrorMessage && msg.Content == m.errorMsg {
				dup = true
				break
			}
		}
		if !dup {
			m.renderedLines = append(m.renderedLines, m.renderErrorMessage("⚠️ "+m.errorMsg)...)
		}
	}

	// Welcome screen when there is nothing to show
	if len(m.renderedLines) == 0 {
		m.renderedLines = append(m.renderedLines,
			"",
			m.muted().Render("  Welcome to Eino CLI Agent — type a message to get started."),
			"",
			m.muted().Render("  Tools, models and agents are configured in config.yml"),
			"",
		)
	}

	// Trailing whitespace is pointless and widens the diff the renderer
	// has to repaint between frames.
	for i := range m.renderedLines {
		m.renderedLines[i] = strings.TrimRight(m.renderedLines[i], " \t")
	}
}

// renderUserMessage renders a compact user message: violet prompt + plain text.
func (m *ViewModel) renderUserMessage(content string) []string {
	prompt := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.User)).
		Bold(true).
		Render("❯")
	lines := []string{prompt + " " + content, ""}
	return wrapLines(lines, m.width)
}

// renderAssistantMessage renders a finalized assistant message with cached markdown.
func (m *ViewModel) renderAssistantMessage(msg *Message) []string {
	if msg.mdCache == "" {
		msg.mdCache = m.renderMarkdown(msg.Content)
	}
	lines := []string{m.assistantLabel(false)}
	lines = append(lines, indentLines(strings.Split(msg.mdCache, "\n"), "  ")...)
	lines = append(lines, "")
	return lines
}

// flushReasoning persists the currently streaming reasoning block as a
// finalized message so it stays above the following answer.
func (m *ViewModel) flushReasoning() {
	if m.reasoningContent == "" {
		return
	}
	m.messages = append(m.messages, Message{
		Type:    ReasoningMessage,
		Content: m.reasoningContent,
	})
	m.reasoningContent = ""
}

// renderReasoningMessage renders the model's thinking as a dimmed italic
// block. While streaming the block is expanded; once finalized it collapses
// to a single summary line until the user expands it.
func (m *ViewModel) renderReasoningMessage(msg *Message, index int, streaming bool) []string {
	headStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.Muted)).
		Bold(true).
		Italic(true)
	body := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.Muted)).
		Italic(true)

	if streaming {
		head := m.reasoningMarker(true, true) + " " + headStyle.Render("◇ Thinking…")
		lines := []string{head}
		lines = append(lines, m.reasoningBodyLines(msg.Content, body)...)
		return lines
	}

	expanded := m.expanded[index]
	label := "◇ Thinking"
	if m.selected == index {
		headStyle = headStyle.Underline(true)
	}
	head := m.reasoningMarker(expanded, m.selected == index) + " " + headStyle.Render(label)
	if !expanded {
		// Folded: show a single-line snippet of the reasoning instead of
		// just a size summary.
		preview := firstLine(msg.Content)
		maxW := m.width - lipgloss.Width(head) - 3
		if maxW < 16 {
			maxW = 16
		}
		if preview != "" {
			head += m.muted().Render(" · " + truncate(preview, maxW))
		}
		return []string{head}
	}

	lines := []string{head}
	lines = append(lines, m.reasoningBodyLines(msg.Content, body)...)
	return lines
}

// reasoningBodyLines wraps and styles the reasoning text as a dim block.
func (m *ViewModel) reasoningBodyLines(content string, style lipgloss.Style) []string {
	var lines []string
	for _, line := range wrapLines(strings.Split(strings.TrimSpace(content), "\n"), m.width) {
		if strings.TrimSpace(line) == "" {
			continue // compact: drop blank lines inside reasoning
		}
		lines = append(lines, "    "+style.Render(line))
	}
	return lines
}

// reasoningMarker returns the fold marker for a reasoning/tool block.
// Selected blocks show the marker in the accent color.
func (m *ViewModel) reasoningMarker(expanded bool, selected bool) string {
	glyph := "▸"
	if expanded {
		glyph = "▾"
	}
	if selected {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Accent)).Bold(true).Render(glyph)
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Muted)).Render(glyph)
}

// renderAssistantContent renders streaming (or generic) markdown content.
func (m *ViewModel) renderAssistantContent(content string, streaming bool) []string {
	lines := []string{m.assistantLabel(streaming)}
	rendered := m.renderMarkdown(content)
	lines = append(lines, indentLines(strings.Split(rendered, "\n"), "  ")...)
	lines = append(lines, "")
	return lines
}

// assistantLabel returns the small assistant header line.
func (m *ViewModel) assistantLabel(streaming bool) string {
	dot := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Accent)).Bold(true).Render("◆")
	label := "Assistant"
	if streaming {
		label = "Assistant " + m.muted().Render("(typing…)")
	}
	return dot + " " + label
}

// renderToolMessage renders a compact tool call line. Finished calls are
// folded to one line; expanding shows the full result.
func (m *ViewModel) renderToolMessage(msg *Message, index int) []string {
	var dotColor string
	switch msg.ToolStatus {
	case ToolWaiting:
		dotColor = m.theme.Warning
	case ToolSuccess:
		dotColor = m.theme.Success
	case ToolError:
		dotColor = m.theme.Error
	}

	dot := lipgloss.NewStyle().Foreground(lipgloss.Color(dotColor)).Bold(true).Render("●")
	name := lipgloss.NewStyle().Bold(true).Render(msg.Name)

	expanded := m.expanded[index]
	parts := []string{m.reasoningMarker(expanded, m.selected == index), dot, name}
	if args := summarizeArgs(msg.Arguments); args != "" {
		parts = append(parts, m.muted().Render(args))
	}
	if msg.ToolStatus == ToolWaiting {
		parts = append(parts, m.muted().Render("running..."))
	} else if msg.EndTime > 0 && msg.StartTime > 0 {
		parts = append(parts, m.muted().Render(fmt.Sprintf("%ds", msg.EndTime-msg.StartTime)))
	}

	lines := []string{strings.Join(parts, " ")}

	// Result details only when expanded; folded calls stay single-line.
	if expanded && msg.ToolStatus != ToolWaiting && msg.Result != "" {
		lines = append(lines, m.toolResultLines(msg)...)
	}
	return lines
}

// toolResultLines renders the (possibly long) tool result, capped to keep the
// TUI responsive.
func (m *ViewModel) toolResultLines(msg *Message) []string {
	style := m.muted()
	if msg.ToolStatus == ToolError {
		style = lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Error))
	}

	const maxLines = 12
	raw := strings.Split(strings.TrimRight(msg.Result, "\n"), "\n")
	var lines []string
	for i, line := range raw {
		if i >= maxLines {
			lines = append(lines, "    "+m.muted().Render(fmt.Sprintf("… 还有 %d 行（已截断）", len(raw)-maxLines)))
			break
		}
		lines = append(lines, "    "+style.Render(truncate(strings.TrimRight(line, " \t"), maxInt(m.width-8, 20))))
	}
	return lines
}

// renderErrorMessage renders error content in red, supporting multiple lines
// (command errors often include a usage line plus candidate lists).
func (m *ViewModel) renderErrorMessage(content string) []string {
	icon := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Error)).Bold(true).Render("✗")
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Error))

	var lines []string
	first := true
	for _, raw := range strings.Split(strings.TrimSpace(content), "\n") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		for _, wrapped := range wrapLines([]string{raw}, m.width) {
			text := strings.TrimSpace(wrapped)
			if first {
				lines = append(lines, icon+" "+style.Render(text))
				first = false
			} else {
				lines = append(lines, "   "+style.Render(text))
			}
		}
	}
	if len(lines) == 0 {
		lines = append(lines, " "+icon)
	}
	lines = append(lines, "")
	return lines
}

// renderInfoMessage renders command output / system notices in muted text.
func (m *ViewModel) renderInfoMessage(content string) []string {
	var lines []string
	for _, line := range wrapLines(strings.Split(strings.TrimSpace(content), "\n"), m.width) {
		if strings.TrimSpace(line) == "" {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, "  "+m.muted().Render(line))
	}
	lines = append(lines, "")
	return lines
}

// View renders the interface
func (m *ViewModel) View() string {
	if m.width == 0 {
		return "Starting..."
	}

	m.refreshCache()

	viewport := m.viewportHeight()
	total := len(m.renderedLines)

	// Line-based scrolling: show the newest lines unless scrolled back.
	start := total - viewport - m.scrollOffset
	if start < 0 {
		start = 0
	}
	end := start + viewport
	if end > total {
		end = total
	}
	visible := m.renderedLines[start:end]
	messageArea := strings.Join(visible, "\n")
	// Pad short conversations so the input box stays glued to the bottom of
	// the terminal instead of floating mid-screen.
	if pad := viewport - len(visible); pad > 0 {
		messageArea += strings.Repeat("\n", pad)
	}

	// ---- Header: title left, status right ----
	statusText := "ready"
	statusDot := m.theme.Success
	if m.isWaiting {
		statusText = "thinking"
		statusDot = m.theme.Warning
	} else if m.errorMsg != "" {
		statusText = "error"
		statusDot = m.theme.Error
	}
	title := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Primary)).Bold(true).Render("◆ Eino CLI Agent")
	if m.subtitle != "" {
		title += m.muted().Render(" · " + m.subtitle)
	}
	dot := lipgloss.NewStyle().Foreground(lipgloss.Color(statusDot)).Render("●")
	status := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Muted)).Render(dot + " " + statusText)
	header := joinRow(title, status, m.width-2)
	rule := m.muted().Render(strings.Repeat("─", maxInt(m.width-2, 3)))

	// ---- Input area ----
	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(m.theme.Primary)).
		Padding(0, 1).
		Width(m.width - 4)

	var inputLine string
	if m.confirmQuit {
		inputLine = m.warningStyle(true).Render("⚠ 再按一次 Ctrl+C 退出（任意其他键取消）")
	} else if m.isWaiting {
		inputLine = m.muted().Render("⏳ waiting for response... (ctrl+c to quit)")
	} else {
		prompt := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Primary)).Bold(true).Render("❯")
		inputLine = prompt + " " + m.renderEditableInput()
		// Keep the input on one line: overflowing content would wrap inside
		// the box and push the layout past the terminal height. Show the tail
		// (near the cursor) when truncated.
		maxCells := m.width - 9 // border(2) + padding(2) + cursor safety
		if w := lipgloss.Width(inputLine); w > maxCells {
			inputLine = ansi.TruncateLeft(inputLine, w-maxCells, "…")
		}
	}
	inputArea := inputBox.Render(inputLine)

	// ---- Slash command palette (rendered above the input box) ----
	palette := m.paletteView()
	paletteArea := ""
	if len(palette) > 0 {
		paletteArea = strings.Join(palette, "\n")
	}

	// ---- Help / status bar ----
	helpLeft := m.muted().Render("↑↓ scroll · tab select · ctrl+o fold · enter send · /help · ctrl+y mouse · ctrl+c quit")

	scrollInfo := ""
	if total > viewport {
		top := start + 1
		if m.scrollOffset == 0 {
			top = maxInt(total-viewport+1, 1)
		}
		scrollInfo = m.muted().Render(fmt.Sprintf("%d-%d/%d", top, end, total))
	}
	helpBar := joinRow(helpLeft, scrollInfo, m.width-2)

	// Layout (top to bottom): header, rule, messages, help bar, command
	// palette, input box. The input box is always the bottom-most element
	// and never moves when the palette appears.
	parts := []string{header, rule, messageArea, helpBar}
	if paletteArea != "" {
		parts = append(parts, paletteArea)
	}
	parts = append(parts, inputArea)

	// Hard-clip every line to the terminal width: wrapped lines break the
	// renderer's cursor accounting and leave artifacts after resizes.
	lines := strings.Split(strings.Join(parts, "\n"), "\n")
	for i, line := range lines {
		if lipgloss.Width(line) > m.width {
			lines[i] = ansi.Truncate(line, m.width, "")
		}
	}
	return strings.Join(lines, "\n")
}

// renderEditableInput renders the input with a visible cursor position.
func (m *ViewModel) renderEditableInput() string {
	runes := []rune(m.input)
	if m.cursor > len(runes) {
		m.cursor = len(runes)
	}
	left := string(runes[:m.cursor])
	var right string
	if m.cursor < len(runes) {
		cursorChar := lipgloss.NewStyle().Reverse(true).Render(string(runes[m.cursor]))
		right = cursorChar + string(runes[m.cursor+1:])
	} else {
		right = lipgloss.NewStyle().Reverse(true).Render(" ")
	}
	return left + right
}

// joinRow places left and right text on one row with space between.
func joinRow(left, right string, width int) string {
	leftLen := lipgloss.Width(left)
	rightLen := lipgloss.Width(right)
	gap := width - leftLen - rightLen
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// ---- shared small styles ----

// indentLines prefixes each line with the given indent.
func indentLines(lines []string, indent string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		if line == "" {
			out[i] = ""
		} else {
			out[i] = indent + line
		}
	}
	return out
}

// wrapLines hard-wraps plain text lines to the terminal width.
func wrapLines(lines []string, width int) []string {
	if width <= 0 {
		return lines
	}
	limit := width - 4
	var out []string
	for _, line := range lines {
		runes := []rune(line)
		for len(runes) > limit {
			out = append(out, string(runes[:limit]))
			runes = runes[limit:]
		}
		out = append(out, string(runes))
	}
	return out
}

// firstLine returns the first non-empty line of a text, truncated to 160 runes.
func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return truncate(line, 160)
		}
	}
	return ""
}

// truncate cuts a string to maxLen runes with an ellipsis.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// summarizeArgs condenses tool arguments into a short preview.
func summarizeArgs(args string) string {
	args = strings.TrimSpace(args)
	if args == "" || args == "{}" {
		return ""
	}
	if strings.HasPrefix(args, "{") && strings.HasSuffix(args, "}") {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(args), &parsed); err == nil {
			var parts []string
			for k, v := range parsed {
				parts = append(parts, fmt.Sprintf("%s=%v", k, v))
			}
			if len(parts) > 0 {
				return truncate(strings.Join(parts, " "), 60)
			}
		}
	}
	return truncate(args, 60)
}

// renderMarkdown renders markdown content, falling back to plain text.
func (m *ViewModel) renderMarkdown(content string) string {
	if m.renderer == nil {
		return content
	}
	rendered, err := m.renderer.Render(content)
	if err != nil {
		return content
	}
	return strings.TrimSpace(rendered)
}

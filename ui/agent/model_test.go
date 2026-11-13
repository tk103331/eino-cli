package agent

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func drive(m *ViewModel, msgs ...tea.Msg) *ViewModel {
	for _, msg := range msgs {
		next, _ := m.Update(msg)
		m = next.(*ViewModel)
	}
	return m
}

func TestTUIRendering(t *testing.T) {
	m := NewViewModel(func(string) error { return nil })
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	// Welcome screen
	if out := m.View(); !strings.Contains(out, "Welcome") {
		t.Fatalf("welcome screen missing:\n%s", out)
	}

	// Type input with cursor editing
	m = drive(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
	if m.input != "hello" || m.cursor != 5 {
		t.Fatalf("typing failed: input=%q cursor=%d", m.input, m.cursor)
	}
	m = drive(m, tea.KeyMsg{Type: tea.KeyLeft}, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	if m.input != "hellXo" {
		t.Fatalf("cursor insert failed: %q", m.input)
	}
	m = drive(m, tea.KeyMsg{Type: tea.KeyCtrlU})
	if m.input != "o" || m.cursor != 0 {
		t.Fatalf("ctrl+u failed: %q", m.input)
	}

	// User message + waiting state + spinner
	m = drive(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hi there")})
	m = drive(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.isWaiting || len(m.messages) != 1 || m.messages[0].Type != UserMessage {
		t.Fatalf("enter/send failed: waiting=%v msgs=%d", m.isWaiting, len(m.messages))
	}
	out := m.View()
	if !strings.Contains(out, "hi there") || !strings.Contains(out, "thinking") {
		t.Fatalf("user msg or spinner missing:\n%s", out)
	}

	// Streaming chunks
	m = drive(m, StreamChunkMsg("**Bold** output"))
	time.Sleep(60 * time.Millisecond) // Past the render throttle window
	out = m.View()
	if !strings.Contains(out, "typing") {
		t.Fatalf("streaming label missing:\n%s", out)
	}

	// Tool call lifecycle
	m = drive(m, ToolStartMsg{Name: "web_search", Arguments: `{"query":"weather"}`})
	out = m.View()
	if !strings.Contains(out, "web_search") || !strings.Contains(out, "running") {
		t.Fatalf("tool start missing:\n%s", out)
	}
	m = drive(m, ToolEndMsg{Name: "web_search", Result: `{"result":"sunny 25C"}`})
	out = m.View()
	if !strings.Contains(out, "✓") == false && !strings.Contains(out, "●") {
		t.Fatalf("tool completion missing:\n%s", out)
	}

	// Stream end finalizes the assistant message
	m = drive(m, StreamEndMsg{})
	if m.isWaiting {
		t.Fatal("still waiting after stream end")
	}
	out = m.View()
	if !strings.Contains(out, "Assistant") {
		t.Fatalf("assistant message missing:\n%s", out)
	}

	// Error path
	m = drive(m, ErrorMsg("something broke"))
	out = m.View()
	if !strings.Contains(out, "something broke") {
		t.Fatalf("error rendering missing:\n%s", out)
	}

	// Scrolling clamps
	m = drive(m, tea.KeyMsg{Type: tea.KeyHome})
	if m.scrollOffset < 0 {
		t.Fatalf("negative scroll: %d", m.scrollOffset)
	}
	m = drive(m, tea.KeyMsg{Type: tea.KeyEnd})
	if m.scrollOffset != 0 {
		t.Fatalf("end key did not reset scroll: %d", m.scrollOffset)
	}

	// Ctrl+L clears
	m = drive(m, tea.KeyMsg{Type: tea.KeyCtrlL})
	if len(m.messages) != 0 {
		t.Fatalf("ctrl+l did not clear: %d", len(m.messages))
	}
	if out := m.View(); !strings.Contains(out, "Welcome") {
		t.Fatalf("welcome not restored after clear:\n%s", out)
	}
}

func TestTUIEnhancements(t *testing.T) {
	m := NewViewModel(func(string) error { return nil })
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	// Subtitle shows in the header
	m.SetSubtitle("search_agent")
	if out := m.View(); !strings.Contains(out, "search_agent") {
		t.Fatalf("subtitle missing from header:\n%s", out)
	}

	// Pasted multi-line text collapses to spaces
	m = drive(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("line one\nline two")})
	if m.input != "line one line two" {
		t.Fatalf("paste newline handling failed: %q", m.input)
	}
	m = drive(m, tea.KeyMsg{Type: tea.KeyCtrlU})

	// Scrolling is allowed while waiting for a response
	m = drive(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hi")})
	m = drive(m, tea.KeyMsg{Type: tea.KeyEnter})
	for i := 0; i < 40; i++ {
		m = drive(m, StreamChunkMsg(fmt.Sprintf("more output line %d\n\n", i)))
	}
	time.Sleep(60 * time.Millisecond) // Past the render throttle window
	m.View()                          // Refresh the rendered-line cache
	before := m.scrollOffset
	m = drive(m, tea.KeyMsg{Type: tea.KeyUp})
	if m.scrollOffset <= before {
		t.Fatalf("scroll blocked while waiting: %d -> %d", before, m.scrollOffset)
	}
	m = drive(m, tea.KeyMsg{Type: tea.KeyEnd})
	if m.scrollOffset != 0 {
		t.Fatalf("end key blocked while waiting: %d", m.scrollOffset)
	}

	// Spinner is hidden once streaming content exists; the "(typing…)"
	// label sits at the top of the streaming block, so scroll up to it.
	m = drive(m, tea.KeyMsg{Type: tea.KeyHome})
	out := m.View()
	if strings.Contains(out, "thinking...") {
		t.Fatalf("spinner shown while streaming:\n%s", out)
	}
	if !strings.Contains(out, "typing") {
		t.Fatalf("typing label missing:\n%s", out)
	}
	m = drive(m, tea.KeyMsg{Type: tea.KeyEnd})

	// Finish the turn, then /clear empties the chat
	m = drive(m, StreamEndMsg{})
	m = drive(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/clear")})
	m = drive(m, tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.messages) != 0 || m.input != "" {
		t.Fatalf("/clear failed: msgs=%d input=%q", len(m.messages), m.input)
	}
	if out := m.View(); !strings.Contains(out, "Welcome") {
		t.Fatalf("welcome not restored after /clear:\n%s", out)
	}
}

func TestReasoningDisplay(t *testing.T) {
	m := NewViewModel(func(string) error { return nil })
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m = drive(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hi")})
	m = drive(m, tea.KeyMsg{Type: tea.KeyEnter})

	// Streaming reasoning shows a Thinking block
	m = drive(m, ReasoningChunkMsg("用户打了个招呼，简单回复即可。\n第二行细节不应在折叠态显示"))
	time.Sleep(60 * time.Millisecond)
	out := m.View()
	if !strings.Contains(out, "Thinking") {
		t.Fatalf("streaming thinking label missing:\n%s", out)
	}
	if !strings.Contains(out, "用户打了个招呼") {
		t.Fatalf("streaming reasoning text missing:\n%s", out)
	}

	// Answer streams after reasoning
	m = drive(m, StreamChunkMsg("你好！有什么可以帮你？"))
	m = drive(m, StreamEndMsg{})

	if len(m.messages) != 3 { // user + reasoning + assistant
		t.Fatalf("messages=%d, want 3", len(m.messages))
	}
	if m.messages[1].Type != ReasoningMessage {
		t.Fatalf("reasoning message missing, got type %v", m.messages[1].Type)
	}
	if m.messages[2].Type != AssistantMessage || !strings.Contains(m.messages[2].Content, "你好") {
		t.Fatalf("assistant answer wrong: %+v", m.messages[2])
	}

	// Once finalized, the thinking block folds to a single-line snippet.
	out = m.View()
	if !strings.Contains(out, "Thinking") {
		t.Fatalf("folded thinking label missing:\n%s", out)
	}
	if !strings.Contains(out, "用户打了个招呼") {
		t.Fatalf("folded thinking snippet missing:\n%s", out)
	}
	if strings.Contains(out, "第二行细节") {
		t.Fatalf("folded thinking must not show later lines:\n%s", out)
	}
	if strings.Contains(m.messages[2].Content, "用户打了个招呼") {
		t.Fatal("reasoning leaked into the assistant answer")
	}

	// Tab selects the first foldable block; ctrl+o expands it.
	m = drive(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.selected != 1 {
		t.Fatalf("tab should select reasoning block, got %d", m.selected)
	}
	m = drive(m, tea.KeyMsg{Type: tea.KeyCtrlO})
	out = m.View()
	if !strings.Contains(out, "用户打了个招呼") || !strings.Contains(out, "第二行细节") {
		t.Fatalf("expanded reasoning text missing:\n%s", out)
	}
}

func TestCollapsibleToolCall(t *testing.T) {
	m := NewViewModel(func(string) error { return nil })
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m = drive(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("run")})
	m = drive(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = drive(m, ToolStartMsg{Name: "shell", Arguments: `{"command":"echo hi"}`})
	m = drive(m, ToolEndMsg{Name: "shell", Result: "RESULT_TOKEN_X"})
	m = drive(m, StreamChunkMsg("done"))
	m = drive(m, StreamEndMsg{})

	// Folded by default: the tool line is visible but its result is hidden.
	out := m.View()
	if !strings.Contains(out, "shell") {
		t.Fatalf("tool line missing:\n%s", out)
	}
	if strings.Contains(out, "RESULT_TOKEN_X") {
		t.Fatalf("tool result should be folded by default:\n%s", out)
	}

	// Select the tool block (second foldable item: reasoning? no — only tool)
	m = drive(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.selected < 0 || m.messages[m.selected].Type != ToolStartMessage {
		t.Fatalf("tab should select the tool block, got %d", m.selected)
	}

	// Expand: the full result becomes visible.
	m = drive(m, tea.KeyMsg{Type: tea.KeyCtrlO})
	out = m.View()
	if !strings.Contains(out, "RESULT_TOKEN_X") {
		t.Fatalf("expanded tool result missing:\n%s", out)
	}

	// Collapse again.
	m = drive(m, tea.KeyMsg{Type: tea.KeyCtrlO})
	out = m.View()
	if strings.Contains(out, "RESULT_TOKEN_X") {
		t.Fatalf("tool result should be folded again:\n%s", out)
	}
}

func TestViewFillsTerminalHeight(t *testing.T) {
	for _, h := range []int{20, 30, 40} {
		m := NewViewModel(func(string) error { return nil })
		m = drive(m, tea.WindowSizeMsg{Width: 100, Height: h})
		m = drive(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hi")})
		m = drive(m, tea.KeyMsg{Type: tea.KeyEnter})
		m = drive(m, StreamChunkMsg("hello"))
		m = drive(m, StreamEndMsg{})

		out := m.View()
		lines := strings.Split(out, "\n")
		if len(lines) != h {
			t.Fatalf("height %d: view has %d lines", h, len(lines))
		}
		last := strings.TrimRight(lines[len(lines)-1], " ")
		if !strings.HasPrefix(last, "↑↓ scroll") {
			t.Fatalf("height %d: last line should be the help bar, got %q", h, last)
		}
		// The input box sits directly above the help bar.
		prev := strings.TrimRight(lines[len(lines)-2], " ")
		if !strings.HasPrefix(prev, "╰") {
			t.Fatalf("height %d: input box bottom border should be above the help bar, got %q", h, prev)
		}
	}

	// With the palette visible the total height must still match exactly.
	m := NewViewModel(func(string) error { return nil })
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.SetSlashCommands([]SlashCommand{
		{Name: "model", Args: "<name>", Values: []string{"a", "b"}},
		{Name: "skill", Args: "<name>", Values: []string{"x"}},
	}, nil)
	m = drive(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	out := m.View()
	if got := len(strings.Split(out, "\n")); got != 30 {
		t.Fatalf("palette layout height = %d, want 30", got)
	}
	lines := strings.Split(out, "\n")
	last := strings.TrimRight(lines[29], " ")
	if !strings.HasPrefix(last, "↑↓ scroll") {
		t.Fatalf("help bar should stay at the bottom with palette visible: %q", last)
	}
	if prev := strings.TrimRight(lines[28], " "); !strings.HasPrefix(prev, "╰") {
		t.Fatalf("input box should stay above the help bar with palette visible: %q", prev)
	}
}

func TestViewClipsLinesToWidth(t *testing.T) {
	long := strings.Repeat("这是一段很长很长的输入内容", 12)

	m := NewViewModel(func(string) error { return nil })
	m = drive(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m = drive(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(long)})

	check := func(width int) {
		out := m.View()
		lines := strings.Split(out, "\n")
		if len(lines) != 24 {
			t.Fatalf("width %d: %d lines, want 24", width, len(lines))
		}
		for i, line := range lines {
			if w := lipgloss.Width(line); w > width {
				t.Fatalf("width %d: line %d is %d cells wide (wraps on resize)", width, i, w)
			}
		}
	}
	check(80)

	// Narrower terminal: every line must fit the new width.
	m = drive(m, tea.WindowSizeMsg{Width: 50, Height: 24})
	check(50)

	// Wider again.
	m = drive(m, tea.WindowSizeMsg{Width: 120, Height: 24})
	check(120)
}

func TestMouseToggleShortcut(t *testing.T) {
	m := NewViewModel(func(string) error { return nil })
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	enabled := false
	m.SetMouseToggle(func() bool {
		enabled = !enabled
		return enabled
	})

	m = drive(m, tea.KeyMsg{Type: tea.KeyCtrlY})
	if !enabled {
		t.Fatal("first Ctrl+Y should enable mouse capture")
	}
	if !strings.Contains(m.View(), "鼠标捕获已启用") {
		t.Fatalf("enable hint missing:\n%s", m.View())
	}

	m = drive(m, tea.KeyMsg{Type: tea.KeyCtrlY})
	if enabled {
		t.Fatal("second Ctrl+Y should disable mouse capture")
	}
	if !strings.Contains(m.View(), "鼠标捕获已关闭") {
		t.Fatalf("disable hint missing:\n%s", m.View())
	}
}

func TestMouseClickTogglesToolBlock(t *testing.T) {
	m := NewViewModel(func(string) error { return nil })
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.SetTranscript([]Message{
		{Type: UserMessage, Content: "question"},
		{Type: ToolStartMessage, Name: "search", Arguments: `{"q":"x"}`, ToolStatus: ToolSuccess, Result: "the result"},
	})
	m.View()

	lineY := func(needle string) int {
		for i, line := range m.renderedLines {
			if strings.Contains(line, needle) {
				return i - m.viewStart + 2 // header + rule
			}
		}
		t.Fatalf("rendered line %q not found", needle)
		return -1
	}

	// Clicking the folded tool line expands it and shows the result.
	m = drive(m, tea.MouseMsg{X: 5, Y: lineY("search"), Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if !m.expanded[1] {
		t.Fatal("left click should expand the tool block")
	}
	if !strings.Contains(m.View(), "the result") {
		t.Fatalf("expanded tool result missing:\n%s", m.View())
	}

	// Clicking again folds it.
	row := lineY("search")
	m = drive(m, tea.MouseMsg{X: 5, Y: row, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if m.expanded[1] {
		t.Fatal("second click should fold the tool block")
	}

	// Clicking a non-foldable line (user message) is a no-op.
	m = drive(m, tea.MouseMsg{X: 5, Y: lineY("question"), Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if m.expanded[0] {
		t.Fatal("user message should not be foldable")
	}

	// Clicks on the header/help chrome are ignored.
	m = drive(m, tea.MouseMsg{X: 5, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if m.expanded[0] || m.expanded[1] {
		t.Fatalf("chrome click should not toggle anything: %v", m.expanded)
	}
}

func TestErrorMessageFoldsByDefault(t *testing.T) {
	m := NewViewModel(func(string) error { return nil })
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m = drive(m, ErrorMsg("error: connect failed\nnode path: [chat]"))

	out := m.View()
	if !strings.Contains(out, "connect failed") {
		t.Fatalf("folded error should show its first line:\n%s", out)
	}
	if strings.Contains(out, "node path") {
		t.Fatalf("error details should be folded by default:\n%s", out)
	}

	// Tab selects the error block; Ctrl+O expands it in place.
	m = drive(m, tea.KeyMsg{Type: tea.KeyTab}, tea.KeyMsg{Type: tea.KeyCtrlO})
	if out := m.View(); !strings.Contains(out, "node path") {
		t.Fatalf("expanded error should show every line:\n%s", out)
	}
}

func TestSelectedFoldableRowIsHighlighted(t *testing.T) {
	m := NewViewModel(func(string) error { return nil })
	m = drive(m, tea.WindowSizeMsg{Width: 80, Height: 30})
	m = drive(m,
		ToolStartMsg{Name: "filesystem", Arguments: `{"operation":"list","path":"."}`},
		ToolEndMsg{Name: "filesystem", Result: "ok"},
	)

	// Tab selects the newest foldable row; it must be highlighted across the
	// full terminal width, not just on the fold marker.
	m = drive(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.selected < 0 {
		t.Fatal("tab did not select a foldable row")
	}

	found := false
	for _, line := range strings.Split(m.View(), "\n") {
		if !strings.Contains(line, "filesystem") {
			continue
		}
		found = true
		if w := lipgloss.Width(line); w != 80 {
			t.Fatalf("selected row width = %d, want the full 80 columns", w)
		}
	}
	if !found {
		t.Fatalf("tool row missing:\n%s", m.View())
	}
}

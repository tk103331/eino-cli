package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tk103331/eino-cli/session"
	"github.com/tk103331/eino-cli/tools"
)

func TestSetTranscript(t *testing.T) {
	m := NewViewModel(func(string) error { return nil })
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	m.SetTranscript([]Message{
		{Type: UserMessage, Content: "first question"},
		{Type: AssistantMessage, Content: "first answer"},
		{Type: UserMessage, Content: "second question"},
	})

	if len(m.messages) != 3 {
		t.Fatalf("transcript not loaded: %d messages", len(m.messages))
	}
	out := m.View()
	for _, want := range []string{"first question", "first answer", "second question"} {
		if !strings.Contains(out, want) {
			t.Fatalf("transcript missing %q:\n%s", want, out)
		}
	}
}

func TestClearNotifiesHost(t *testing.T) {
	m := NewViewModel(func(string) error { return nil })
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	cleared := 0
	m.SetOnClear(func() { cleared++ })

	// Ctrl+L clears and notifies.
	m = drive(m, tea.KeyMsg{Type: tea.KeyCtrlL})
	if cleared != 1 || len(m.messages) != 0 {
		t.Fatalf("ctrl+l clear failed: cleared=%d msgs=%d", cleared, len(m.messages))
	}

	// /clear does the same through the slash handler.
	m.SetTranscript([]Message{{Type: UserMessage, Content: "x"}})
	m = drive(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/clear")})
	m = drive(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cleared != 2 || len(m.messages) != 0 {
		t.Fatalf("/clear failed: cleared=%d msgs=%d", cleared, len(m.messages))
	}
}

func TestAgentTurnPersistence(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	app := &AgentApp{agentName: "builtin"}
	app.recordTurn(session.RoleUser, "first question")
	app.recordReasoning("thinking about it")
	app.recordToolStart("search", `{"q":"go"}`)
	app.recordToolEnd("search", "search result", false)
	app.recordTurn(session.RoleAssistant, "first answer")

	history := app.historySnapshot()
	if len(history) != 2 {
		t.Fatalf("history should have 2 model turns, got %d", len(history))
	}
	if history[0].Role != "user" || history[0].Content != "first question" {
		t.Fatalf("user turn wrong: %+v", history[0])
	}
	if history[1].Role != "assistant" || history[1].Content != "first answer" {
		t.Fatalf("assistant turn wrong: %+v", history[1])
	}

	sessions, err := session.List(session.KindAgent, 0)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("session not persisted: %d (err=%v)", len(sessions), err)
	}
	if sessions[0].ID != app.currentSessionID() || sessions[0].TurnCount() != 2 {
		t.Fatalf("persisted session mismatch: %+v", sessions[0])
	}

	// Reasoning and the tool call must come back in the resumed transcript.
	loaded, err := session.Load(sessions[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	transcript := sessionTranscript(loaded)
	var types []MessageType
	for _, msg := range transcript {
		types = append(types, msg.Type)
	}
	want := []MessageType{UserMessage, ReasoningMessage, ToolStartMessage, AssistantMessage}
	if len(types) != len(want) {
		t.Fatalf("transcript types = %v, want %v", types, want)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Fatalf("transcript types = %v, want %v", types, want)
		}
	}
	if tool := transcript[2]; tool.Name != "search" || tool.ToolStatus != ToolSuccess || tool.Result != "search result" {
		t.Fatalf("tool message not restored: %+v", tool)
	}

	// /clear starts a new session without touching the saved one.
	app.startNewSession()
	if app.currentSessionID() == sessions[0].ID {
		t.Fatal("startNewSession should allocate a new id")
	}
	if len(app.historySnapshot()) != 0 {
		t.Fatal("new session should have no history")
	}
}

func TestQuestionAndTodoPersistence(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	app := &AgentApp{agentName: "builtin"}
	app.recordTurn(session.RoleUser, "start")
	app.recordQuestion("which db?", "sqlite")
	err := app.UpdateTodos(context.Background(), []tools.TodoItem{
		{Content: "collect requirements", Status: tools.TodoCompleted},
		{Content: "write tests", Status: tools.TodoPending},
	})
	if err != nil {
		t.Fatal(err)
	}

	sessions, err := session.List(session.KindAgent, 0)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("session not persisted: %d (err=%v)", len(sessions), err)
	}
	loaded, err := session.Load(sessions[0].ID)
	if err != nil {
		t.Fatal(err)
	}

	// The latest todo snapshot is restored by /resume.
	todos := loaded.Todos()
	if len(todos) != 2 || todos[0].Status != session.TodoStatusCompleted {
		t.Fatalf("todos not persisted: %+v", todos)
	}

	// The Q&A shows up as an info message in the resumed transcript.
	found := false
	for _, msg := range sessionTranscript(loaded) {
		if msg.Type == InfoMessage && strings.Contains(msg.Content, "which db?") && strings.Contains(msg.Content, "sqlite") {
			found = true
		}
	}
	if !found {
		t.Fatalf("question transcript missing: %+v", sessionTranscript(loaded))
	}
}

func TestResolveSessionByIndexAndPrefix(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	older := session.New(session.KindAgent, "a", "m", "")
	older.ID = "20200101-000000-aaaaaa" // stable ID so the prefix test is unambiguous
	if err := older.AddUser("old"); err != nil {
		t.Fatal(err)
	}
	newer := session.New(session.KindAgent, "a", "m", "")
	if err := newer.AddUser("new"); err != nil {
		t.Fatal(err)
	}

	// 1 is the newest session.
	got, err := resolveSession(session.KindAgent, "1")
	if err != nil || got.ID != newer.ID {
		t.Fatalf("index resolve failed: %+v (err=%v)", got, err)
	}
	got, err = resolveSession(session.KindAgent, "20200101-000000-aa")
	if err != nil || got.ID != older.ID {
		t.Fatalf("prefix resolve failed: %+v (err=%v)", got, err)
	}
	if _, err := resolveSession(session.KindAgent, "9"); err == nil {
		t.Fatal("out-of-range index should fail")
	}
}

func TestSessionListText(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if text := sessionListText(session.KindAgent, 5); !strings.Contains(text, "暂无历史会话") {
		t.Fatalf("empty list text wrong: %q", text)
	}

	s := session.New(session.KindAgent, "builtin", "ds41", "")
	if err := s.AddUser("会话内容摘要"); err != nil {
		t.Fatal(err)
	}
	text := sessionListText(session.KindAgent, 5)
	for _, want := range []string{s.ID, "builtin", "会话内容摘要", "/resume"} {
		if !strings.Contains(text, want) {
			t.Fatalf("list text missing %q:\n%s", want, text)
		}
	}
}

// Ensure the session clock helper stays stable for status output.
func TestSessionClock(t *testing.T) {
	ts := time.Date(2026, 11, 13, 9, 5, 0, 0, time.Local)
	if got := sessionClock(ts); got != "2026-11-13 09:05" {
		t.Fatalf("sessionClock = %q", got)
	}
}

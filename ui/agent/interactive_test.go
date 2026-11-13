package agent

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tk103331/eino-cli/tools"
	"github.com/tk103331/eino-cli/tools/approval"
)

func receiveAnswer(t *testing.T, reply chan string) string {
	t.Helper()
	select {
	case answer := <-reply:
		return answer
	default:
		t.Fatal("no answer was sent to the waiting tool")
		return ""
	}
}

func TestAskQuestionOptions(t *testing.T) {
	m := NewViewModel(func(string) error { return nil })
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	reply := make(chan string, 1)
	m = drive(m, AskQuestionMsg{Question: "which db?", Options: []string{"sqlite", "postgres"}, Reply: reply})

	out := m.View()
	if !strings.Contains(out, "which db?") {
		t.Fatalf("question not rendered:\n%s", out)
	}
	if !strings.Contains(out, "postgres") || !strings.Contains(out, "❯") {
		t.Fatalf("options not rendered:\n%s", out)
	}

	// Down selects the second option; Enter submits it.
	m = drive(m, tea.KeyMsg{Type: tea.KeyDown})
	m = drive(m, tea.KeyMsg{Type: tea.KeyEnter})
	if got := receiveAnswer(t, reply); got != "postgres" {
		t.Fatalf("answer = %q, want postgres", got)
	}
	if m.question != nil {
		t.Fatal("question should be closed after answering")
	}
	if !strings.Contains(m.View(), "答：postgres") {
		t.Fatalf("Q&A not in transcript:\n%s", m.View())
	}
}

func TestAskQuestionFreeTextAndCancel(t *testing.T) {
	m := NewViewModel(func(string) error { return nil })
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	// No options: type a custom answer.
	reply := make(chan string, 1)
	m = drive(m, AskQuestionMsg{Question: "project name?", Reply: reply})
	m = drive(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("eino")})
	m = drive(m, tea.KeyMsg{Type: tea.KeyEnter})
	if got := receiveAnswer(t, reply); got != "eino" {
		t.Fatalf("answer = %q, want eino", got)
	}

	// With options, typing switches to a free-text answer.
	reply = make(chan string, 1)
	m = drive(m, AskQuestionMsg{Question: "which db?", Options: []string{"sqlite", "postgres"}, Reply: reply})
	m = drive(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("mysql")})
	m = drive(m, tea.KeyMsg{Type: tea.KeyEnter})
	if got := receiveAnswer(t, reply); got != "mysql" {
		t.Fatalf("free-text answer = %q, want mysql", got)
	}

	// Esc cancels: the tool receives an empty answer.
	reply = make(chan string, 1)
	m = drive(m, AskQuestionMsg{Question: "continue?", Reply: reply})
	m = drive(m, tea.KeyMsg{Type: tea.KeyEsc})
	if got := receiveAnswer(t, reply); got != "" {
		t.Fatalf("cancel answer = %q, want empty", got)
	}
	if !strings.Contains(m.View(), "未回答") {
		t.Fatalf("cancel not shown in transcript:\n%s", m.View())
	}
}

func TestApprovalPanelOptionsOnly(t *testing.T) {
	m := NewViewModel(func(string) error { return nil })
	m = drive(m, tea.WindowSizeMsg{Width: 120, Height: 30})

	reply := make(chan string, 1)
	m = drive(m, AskQuestionMsg{
		Question: "shell · 运行命令：rm -rf build",
		Detail:   `{"command":"rm -rf build"}`,
		Options:  []string{approval.AllowOnce, approval.AllowSession, approval.Deny},
		Reply:    reply,
		Approval: true,
	})

	out := m.View()
	for _, want := range []string{"工具审批", "rm -rf build", approval.AllowSession, approval.Deny, "esc 拒绝", "approval"} {
		if !strings.Contains(out, want) {
			t.Fatalf("approval panel missing %q:\n%s", want, out)
		}
	}

	// Typing must not switch the prompt to a free-text answer.
	m = drive(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("yes")})
	if m.input != "" || m.question == nil || m.question.typing {
		t.Fatalf("approval prompt should ignore typing (input=%q state=%+v)", m.input, m.question)
	}

	// Down selects "本会话允许", Enter confirms.
	m = drive(m, tea.KeyMsg{Type: tea.KeyDown})
	m = drive(m, tea.KeyMsg{Type: tea.KeyEnter})
	if got := receiveAnswer(t, reply); got != approval.AllowSession {
		t.Fatalf("answer = %q, want %q", got, approval.AllowSession)
	}
	if m.question != nil {
		t.Fatal("approval prompt should close after answering")
	}
	view := m.View()
	if !strings.Contains(view, "⚠") || !strings.Contains(view, "→ "+approval.AllowSession) {
		t.Fatalf("approval not recorded in transcript:\n%s", view)
	}
}

func TestApprovalEscDenies(t *testing.T) {
	m := NewViewModel(func(string) error { return nil })
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	reply := make(chan string, 1)
	m = drive(m, AskQuestionMsg{
		Question: "shell · 运行命令：rm",
		Options:  []string{approval.AllowOnce, approval.AllowSession, approval.Deny},
		Reply:    reply,
		Approval: true,
	})
	m = drive(m, tea.KeyMsg{Type: tea.KeyEsc})

	if got := receiveAnswer(t, reply); got != "" {
		t.Fatalf("esc should send an empty (denied) answer, got %q", got)
	}
	if !strings.Contains(m.View(), "未回答") {
		t.Fatalf("denied approval not shown in transcript:\n%s", m.View())
	}
}

func TestApproverSessionGrant(t *testing.T) {
	a := &approver{granted: map[string]bool{"shell": true}}
	ok, err := a.approve(context.Background(), nil, approval.Request{Tool: "shell"})
	if err != nil || !ok {
		t.Fatalf("session grant should allow without prompting: %v %v", ok, err)
	}

	a.reset()
	if _, err := a.approve(context.Background(), nil, approval.Request{Tool: "shell"}); err == nil {
		t.Fatal("without a UI the approver should fail instead of silently allowing")
	}
}

func TestTodoPanel(t *testing.T) {
	m := NewViewModel(func(string) error { return nil })
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	m = drive(m, TodoUpdateMsg{Items: []TodoItem{
		{Content: "collect requirements", Status: TodoCompleted},
		{Content: "implement tool", Status: TodoInProgress},
		{Content: "write tests", Status: TodoPending},
	}})

	out := m.View()
	for _, want := range []string{"任务 1/3", "✔ collect requirements", "▸ implement tool", "○ write tests"} {
		if !strings.Contains(out, want) {
			t.Fatalf("task panel missing %q:\n%s", want, out)
		}
	}

	// Clearing the list hides the panel.
	m.SetTodos(nil)
	if strings.Contains(m.View(), "任务 1/3") {
		t.Fatalf("task panel should be hidden after clearing:\n%s", m.View())
	}
}

func TestFinishedTodoPanelCollapsesOnNextInput(t *testing.T) {
	m := NewViewModel(func(string) error { return nil })
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	done := []TodoItem{
		{Content: "collect requirements", Status: TodoCompleted},
		{Content: "write tests", Status: TodoCompleted},
	}
	m = drive(m, TodoUpdateMsg{Items: done})
	if out := m.View(); !strings.Contains(out, "✔ collect requirements") {
		t.Fatalf("finished list should stay expanded until the next input:\n%s", out)
	}

	// The next user turn folds the finished list.
	m = drive(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("next task")}, tea.KeyMsg{Type: tea.KeyEnter})
	out := m.View()
	if strings.Contains(out, "collect requirements") {
		t.Fatalf("finished list should be folded on the next input:\n%s", out)
	}
	if !strings.Contains(out, "任务 2/2") || !strings.Contains(out, "已折叠") {
		t.Fatalf("folded panel summary missing:\n%s", out)
	}

	// New pending work expands the panel again.
	m = drive(m, ResponseMsg("ok"))
	m = drive(m, TodoUpdateMsg{Items: []TodoItem{{Content: "follow up", Status: TodoPending}}})
	if out := m.View(); !strings.Contains(out, "follow up") {
		t.Fatalf("new work should expand the panel:\n%s", out)
	}

	// Clicking the folded panel toggles it (with mouse capture enabled).
	m = drive(m, TodoUpdateMsg{Items: done})
	m = drive(m, ResponseMsg("done"), tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("again")}, tea.KeyMsg{Type: tea.KeyEnter})
	if strings.Contains(m.View(), "collect requirements") {
		t.Fatal("expected the finished list to be folded")
	}
	y := 2 + m.viewportHeight() // header + rule + transcript
	m = drive(m, tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: 5, Y: y})
	if !strings.Contains(m.View(), "collect requirements") {
		t.Fatalf("click should expand the folded panel:\n%s", m.View())
	}
}

func TestInteractiveToolsRegisteredForAgents(t *testing.T) {
	ac := appendUnique(nil, tools.AskUserToolName, tools.AskUserToolName, tools.TodoToolName)
	if len(ac) != 2 {
		t.Fatalf("appendUnique dedupe failed: %v", ac)
	}
}

package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/tools/approval"
)

// fakeHost records interactions from the interactive tools.
type fakeHost struct {
	answer   string
	asked    string
	options  []string
	todos    []TodoItem
	askCalls int

	approved      bool
	approvalCalls int
	approvalReq   approval.Request
}

func (h *fakeHost) AskUser(ctx context.Context, question string, options []string) (string, error) {
	h.askCalls++
	h.asked = question
	h.options = options
	return h.answer, nil
}

func (h *fakeHost) UpdateTodos(ctx context.Context, items []TodoItem) error {
	h.todos = items
	return nil
}

func (h *fakeHost) ApproveTool(ctx context.Context, req approval.Request) (bool, error) {
	h.approvalCalls++
	h.approvalReq = req
	return h.approved, nil
}

func TestAskUserTool(t *testing.T) {
	tool, err := NewAskUserTool(AskUserToolName, config.Tool{})
	if err != nil {
		t.Fatal(err)
	}
	if info, _ := tool.Info(context.Background()); info.Name != AskUserToolName || info.Desc == "" {
		t.Fatalf("bad tool info: %+v", info)
	}

	// Without a host the tool must fail instead of blocking.
	if _, err := tool.InvokableRun(context.Background(), `{"question":"which db?"}`); err == nil {
		t.Fatal("ask_user without host should fail")
	}

	host := &fakeHost{answer: "sqlite"}
	out, err := tool.InvokableRun(WithHost(context.Background(), host),
		`{"question":"which db?","options":["sqlite","postgres","  "]}`)
	if err != nil {
		t.Fatal(err)
	}
	if host.asked != "which db?" || len(host.options) != 2 {
		t.Fatalf("host got question=%q options=%v", host.asked, host.options)
	}
	if !strings.Contains(out, "sqlite") {
		t.Fatalf("answer not returned to the model: %q", out)
	}

	// Empty answer means the user cancelled.
	host = &fakeHost{}
	out, err = tool.InvokableRun(WithHost(context.Background(), host), `{"question":"q"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "取消") {
		t.Fatalf("cancel result wrong: %q", out)
	}

	// Missing question is a tool error.
	if _, err := tool.InvokableRun(WithHost(context.Background(), host), `{}`); err == nil {
		t.Fatal("empty question should fail")
	}
}

func TestTodoTool(t *testing.T) {
	tool, err := NewTodoTool(TodoToolName, config.Tool{})
	if err != nil {
		t.Fatal(err)
	}

	args := `{"todos":[
		{"content":"collect requirements","status":"done"},
		{"content":"implement tool","status":"in-progress"},
		{"content":"write tests","status":"pending"},
		{"content":"   ","status":"pending"}
	]}`
	host := &fakeHost{}
	out, err := tool.InvokableRun(WithHost(context.Background(), host), args)
	if err != nil {
		t.Fatal(err)
	}

	if len(host.todos) != 3 {
		t.Fatalf("expected 3 non-empty tasks, got %d", len(host.todos))
	}
	wantStatuses := []string{TodoCompleted, TodoInProgress, TodoPending}
	for i, want := range wantStatuses {
		if host.todos[i].Status != want {
			t.Fatalf("task %d status = %q, want %q", i, host.todos[i].Status, want)
		}
	}
	if !strings.Contains(out, "1/3") || !strings.Contains(out, "✔ collect requirements") {
		t.Fatalf("summary wrong: %q", out)
	}

	// An empty list clears the tasks.
	if _, err := tool.InvokableRun(WithHost(context.Background(), host), `{"todos":[]}`); err != nil {
		t.Fatal(err)
	}
	if len(host.todos) != 0 {
		t.Fatalf("expected cleared task list, got %d", len(host.todos))
	}

	// Without a host the tool must fail.
	if _, err := tool.InvokableRun(context.Background(), `{"todos":[]}`); err == nil {
		t.Fatal("todo without host should fail")
	}
	// Invalid JSON is rejected.
	if _, err := tool.InvokableRun(WithHost(context.Background(), &fakeHost{}), `{bad`); err == nil {
		t.Fatal("invalid JSON should fail")
	}
}

func TestInteractiveToolFactory(t *testing.T) {
	ask, err := CreateTool(AskUserToolName, config.Tool{Type: AskUserToolType})
	if err != nil || ask == nil {
		t.Fatalf("askuser type not registered: %v", err)
	}
	todo, err := CreateTool(TodoToolName, config.Tool{Type: TodoToolType})
	if err != nil || todo == nil {
		t.Fatalf("todo type not registered: %v", err)
	}
}

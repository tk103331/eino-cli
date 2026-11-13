package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/tools/approval"
)

// Names and config types of the host-interactive tools. The TUI registers
// these automatically for agent sessions; config entries with the same type
// can be added to other modes (run/ACP) but need a Host in the context.
const (
	AskUserToolName = "ask_user"
	AskUserToolType = "askuser"
	TodoToolName    = "todolist"
	TodoToolType    = "todolist"
)

// Todo statuses. The model is asked to pass the complete list on every call.
const (
	TodoPending    = "pending"
	TodoInProgress = "in_progress"
	TodoCompleted  = "completed"
)

// TodoItem is one entry of the task list.
type TodoItem struct {
	Content string `json:"content"`
	Status  string `json:"status"`
}

// Host is implemented by interactive front-ends (the TUI) that can talk to
// the user and display the task list. Tools receive it through the context.
type Host interface {
	// AskUser asks the user a question and returns the answer ("" when the
	// user cancels).
	AskUser(ctx context.Context, question string, options []string) (string, error)
	// UpdateTodos replaces the displayed task list.
	UpdateTodos(ctx context.Context, items []TodoItem) error
	// ApproveTool asks the user whether a dangerous tool call may run. It
	// reports false when the user denies the request.
	ApproveTool(ctx context.Context, req approval.Request) (bool, error)
}

// hostContextKey avoids collisions with other context values.
type hostContextKey struct{}

// WithHost attaches an interactive host to a context.
func WithHost(ctx context.Context, h Host) context.Context {
	return context.WithValue(ctx, hostContextKey{}, h)
}

// HostFromContext returns the host attached to a context, or nil.
func HostFromContext(ctx context.Context) Host {
	h, _ := ctx.Value(hostContextKey{}).(Host)
	return h
}

// ---- ask_user ----

// AskUserTool asks the user a question through the host and waits for an answer.
type AskUserTool struct {
	info *schema.ToolInfo
}

// NewAskUserTool creates the ask_user tool.
func NewAskUserTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	desc := cfg.Description
	if desc == "" {
		desc = "Ask the user a question and wait for the answer. Use it when a decision " +
			"or missing information blocks progress. Provide options when there are a few " +
			"clear choices; otherwise the user answers in free text."
	}
	return &AskUserTool{info: &schema.ToolInfo{
		Name: name,
		Desc: desc,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"question": {
				Type:     schema.String,
				Desc:     "The question to ask the user.",
				Required: true,
			},
			"options": {
				Type:     schema.Array,
				Desc:     "Optional answer choices. When provided the user can pick one or type a custom answer.",
				ElemInfo: &schema.ParameterInfo{Type: schema.String},
			},
		}),
	}}, nil
}

// Info returns the tool metadata.
func (t *AskUserTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.info, nil
}

// InvokableRun asks the question and returns the user's answer.
func (t *AskUserTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Question string   `json:"question"`
		Options  []string `json:"options"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse ask_user arguments: %w", err)
	}
	question := strings.TrimSpace(args.Question)
	if question == "" {
		return "", fmt.Errorf("ask_user requires a non-empty question")
	}

	host := HostFromContext(ctx)
	if host == nil {
		return "", fmt.Errorf("ask_user is only available in interactive sessions")
	}

	answer, err := host.AskUser(ctx, question, cleanOptions(args.Options))
	if err != nil {
		return "", fmt.Errorf("ask_user failed: %w", err)
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return "用户取消了回答（未提供答案）。请根据已有信息继续，或换一种方式推进。", nil
	}
	return "用户回答：" + answer, nil
}

// cleanOptions trims blank option strings.
func cleanOptions(options []string) []string {
	out := make([]string, 0, len(options))
	for _, opt := range options {
		if opt = strings.TrimSpace(opt); opt != "" {
			out = append(out, opt)
		}
	}
	return out
}

// ---- todo ----

// TodoTool maintains the task list shown by the host.
type TodoTool struct {
	info *schema.ToolInfo
}

// NewTodoTool creates the todo tool.
func NewTodoTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	desc := cfg.Description
	if desc == "" {
		desc = "Create and update the task list for the current work. Pass the COMPLETE " +
			"list on every call; each item has `content` and `status` (pending, in_progress, completed)."
	}
	return &TodoTool{info: &schema.ToolInfo{
		Name: name,
		Desc: desc,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"todos": {
				Type:     schema.Array,
				Desc:     "The complete task list, in order.",
				Required: true,
				ElemInfo: &schema.ParameterInfo{
					Type: schema.Object,
					SubParams: map[string]*schema.ParameterInfo{
						"content": {
							Type:     schema.String,
							Desc:     "Short description of the task.",
							Required: true,
						},
						"status": {
							Type: schema.String,
							Desc: "Task status.",
							Enum: []string{TodoPending, TodoInProgress, TodoCompleted},
						},
					},
				},
			},
		}),
	}}, nil
}

// Info returns the tool metadata.
func (t *TodoTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.info, nil
}

// InvokableRun replaces the host's task list and returns a summary.
func (t *TodoTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Todos []TodoItem `json:"todos"`
	}
	if strings.TrimSpace(argumentsInJSON) != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
			return "", fmt.Errorf("failed to parse todo arguments: %w", err)
		}
	}

	items := make([]TodoItem, 0, len(args.Todos))
	for _, item := range args.Todos {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		items = append(items, TodoItem{Content: content, Status: normalizeTodoStatus(item.Status)})
	}

	host := HostFromContext(ctx)
	if host == nil {
		return "", fmt.Errorf("todo is only available in interactive sessions")
	}
	if err := host.UpdateTodos(ctx, items); err != nil {
		return "", fmt.Errorf("todo update failed: %w", err)
	}

	var done int
	var b strings.Builder
	for _, item := range items {
		if item.Status == TodoCompleted {
			done++
		}
	}
	fmt.Fprintf(&b, "任务列表已更新：%d/%d 完成", done, len(items))
	for _, item := range items {
		b.WriteString("\n" + todoMark(item.Status) + " " + item.Content)
	}
	return b.String(), nil
}

// normalizeTodoStatus maps common spellings onto the three supported statuses.
func normalizeTodoStatus(status string) string {
	switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(status), "-", "_")) {
	case "completed", "complete", "done":
		return TodoCompleted
	case "in_progress", "inprogress", "doing", "active":
		return TodoInProgress
	default:
		return TodoPending
	}
}

// todoMark renders the status glyph used in tool results.
func todoMark(status string) string {
	switch status {
	case TodoCompleted:
		return "✔"
	case TodoInProgress:
		return "▸"
	default:
		return "○"
	}
}

package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/session"
	"github.com/tk103331/eino-cli/tools"
	"github.com/tk103331/eino-cli/tools/approval"
)

// ---- host-interactive tools (ask_user / todolist) ----

// approvalModeLabel reports the effective approval mode for /status.
func approvalModeLabel() string {
	mode := approval.ModeDangerous
	if cfg := config.GetConfig(); cfg != nil && cfg.Settings.Approval != nil {
		if configured := strings.TrimSpace(cfg.Settings.Approval.Mode); configured != "" {
			mode = configured
		}
	}
	switch (approval.Policy{Mode: mode}).ModeOrDefault() {
	case approval.ModeOff:
		return "off（不审批）"
	case approval.ModeAll:
		return "all（每次工具调用都确认）"
	default:
		return "dangerous（危险操作确认）"
	}
}

// approver shows tool-approval prompts and remembers per-tool grants for the
// current session.
type approver struct {
	// prompt serializes prompts: eino may run tool calls in parallel, but the
	// panel can only show one question at a time.
	prompt sync.Mutex

	// data guards granted and is separate from prompt so reset never blocks
	// behind a prompt that is still waiting for the user.
	data    sync.Mutex
	granted map[string]bool
}

// approve asks the user whether req may run. A "for this session" answer is
// remembered until the session is replaced.
func (a *approver) approve(ctx context.Context, program *tea.Program, req approval.Request) (bool, error) {
	if a.isGranted(req.Tool) {
		return true, nil
	}

	a.prompt.Lock()
	defer a.prompt.Unlock()
	if a.isGranted(req.Tool) { // another goroutine may have granted meanwhile
		return true, nil
	}

	answer, err := askApproval(ctx, program, req)
	if err != nil {
		return false, err
	}
	switch answer {
	case approval.AllowSession:
		a.grant(req.Tool)
		return true, nil
	case approval.AllowOnce:
		return true, nil
	default: // 拒绝, esc, or a cancelled turn
		return false, nil
	}
}

func (a *approver) isGranted(name string) bool {
	a.data.Lock()
	defer a.data.Unlock()
	return a.granted[name]
}

func (a *approver) grant(name string) {
	a.data.Lock()
	defer a.data.Unlock()
	if a.granted == nil {
		a.granted = make(map[string]bool)
	}
	a.granted[name] = true
}

// reset drops the session grants.
func (a *approver) reset() {
	a.data.Lock()
	defer a.data.Unlock()
	a.granted = nil
}

// askApproval sends an approval prompt to the TUI and blocks until the user
// decides (or the turn is cancelled, which counts as a denial).
func askApproval(ctx context.Context, program *tea.Program, req approval.Request) (string, error) {
	if program == nil {
		return "", fmt.Errorf("interactive UI is not running")
	}

	question := req.Summary
	if req.Tool != "" {
		question = req.Tool + " · " + question
	}
	detail := req.Detail
	const maxDetail = 400
	if runes := []rune(detail); len(runes) > maxDetail {
		detail = string(runes[:maxDetail]) + "…"
	}

	reply := make(chan string, 1)
	program.Send(AskQuestionMsg{
		Question: question,
		Detail:   detail,
		Options:  []string{approval.AllowOnce, approval.AllowSession, approval.Deny},
		Reply:    reply,
		Approval: true,
	})
	select {
	case answer := <-reply:
		return answer, nil
	case <-ctx.Done():
		program.Send(CancelQuestionMsg{})
		return "", ctx.Err()
	}
}

// ensureInteractiveToolConfigs registers the config entries required by the
// built-in interactive tools. Existing user entries with the same names win.
func ensureInteractiveToolConfigs(cfg *config.Config) {
	if cfg == nil {
		return
	}
	if cfg.Tools == nil {
		cfg.Tools = make(map[string]config.Tool)
	}
	if _, ok := cfg.Tools[tools.AskUserToolName]; !ok {
		cfg.Tools[tools.AskUserToolName] = config.Tool{Type: tools.AskUserToolType}
	}
	if _, ok := cfg.Tools[tools.TodoToolName]; !ok {
		cfg.Tools[tools.TodoToolName] = config.Tool{Type: tools.TodoToolType}
	}
}

// appendUnique appends tool names that are not in the list yet.
func appendUnique(list []string, names ...string) []string {
	seen := make(map[string]bool, len(list))
	for _, name := range list {
		seen[name] = true
	}
	for _, name := range names {
		if !seen[name] {
			list = append(list, name)
			seen[name] = true
		}
	}
	return list
}

// askUser sends the question to the TUI and blocks until the answer arrives
// (or the turn is cancelled).
func askUser(ctx context.Context, program *tea.Program, question string, options []string) (string, error) {
	if program == nil {
		return "", fmt.Errorf("interactive UI is not running")
	}
	reply := make(chan string, 1)
	program.Send(AskQuestionMsg{Question: question, Options: options, Reply: reply})
	select {
	case answer := <-reply:
		return answer, nil
	case <-ctx.Done():
		// The turn was cancelled before the user answered: dismiss the panel
		// so the TUI does not stay stuck in question mode.
		program.Send(CancelQuestionMsg{})
		return "", ctx.Err()
	}
}

// AskUser implements tools.Host for agent sessions.
func (app *AgentApp) AskUser(ctx context.Context, question string, options []string) (string, error) {
	answer, err := askUser(ctx, app.program, question, options)
	if err == nil {
		app.recordQuestion(question, answer)
	}
	return answer, err
}

// UpdateTodos implements tools.Host for agent sessions.
func (app *AgentApp) UpdateTodos(ctx context.Context, items []tools.TodoItem) error {
	if app.program != nil {
		app.program.Send(TodoUpdateMsg{Items: uiTodos(items)})
	}
	app.sessionMu.Lock()
	app.sess = ensureSession(app.sess, session.KindAgent, app.agentName, app.currentModelName(), app.skill)
	err := app.sess.AddTodos(sessionTodos(items))
	s := app.sess
	app.sessionMu.Unlock()
	logSaveError("UI-AGENT", s, err)
	return nil
}

// ApproveTool implements tools.Host for agent sessions.
func (app *AgentApp) ApproveTool(ctx context.Context, req approval.Request) (bool, error) {
	return app.approvals.approve(ctx, app.program, req)
}

// recordQuestion persists an ask_user exchange.
func (app *AgentApp) recordQuestion(question, answer string) {
	app.sessionMu.Lock()
	defer app.sessionMu.Unlock()
	app.sess = ensureSession(app.sess, session.KindAgent, app.agentName, app.currentModelName(), app.skill)
	logSaveError("UI-AGENT", app.sess, app.sess.AddQuestion(question, answer))
}

// uiTodos converts tool items into TUI panel items.
func uiTodos(items []tools.TodoItem) []TodoItem {
	out := make([]TodoItem, 0, len(items))
	for _, item := range items {
		status := TodoPending
		switch item.Status {
		case tools.TodoCompleted:
			status = TodoCompleted
		case tools.TodoInProgress:
			status = TodoInProgress
		}
		out = append(out, TodoItem{Content: item.Content, Status: status})
	}
	return out
}

// sessionTodos converts tool items into persisted session items.
func sessionTodos(items []tools.TodoItem) []session.TodoItem {
	out := make([]session.TodoItem, 0, len(items))
	for _, item := range items {
		out = append(out, session.TodoItem{Content: item.Content, Status: item.Status})
	}
	return out
}

// viewTodos converts a persisted task list into TUI panel items.
func viewTodos(items []session.TodoItem) []TodoItem {
	out := make([]TodoItem, 0, len(items))
	for _, item := range items {
		status := TodoPending
		switch item.Status {
		case session.TodoStatusCompleted:
			status = TodoCompleted
		case session.TodoStatusInProgress:
			status = TodoInProgress
		}
		out = append(out, TodoItem{Content: item.Content, Status: status})
	}
	return out
}

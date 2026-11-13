package agent

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/tk103331/eino-cli/logger"
	"github.com/tk103331/eino-cli/session"
)

// ---- AgentApp persistence state ----

// startNewSession begins a fresh conversation (on startup, after /clear, or
// when the configuration changes).
func (app *AgentApp) startNewSession() {
	app.approvals.reset()
	app.sessionMu.Lock()
	app.sess = session.New(session.KindAgent, app.agentName, app.currentModelName(), app.skill)
	app.sessionMu.Unlock()
}

// recordTurn appends a user or assistant turn to the current session.
func (app *AgentApp) recordTurn(role, text string) {
	app.sessionMu.Lock()
	defer app.sessionMu.Unlock()
	app.sess = ensureSession(app.sess, session.KindAgent, app.agentName, app.currentModelName(), app.skill)
	var err error
	if role == session.RoleUser {
		err = app.sess.AddUser(text)
	} else {
		err = app.sess.AddAssistant(text)
	}
	logSaveError("UI-AGENT", app.sess, err)
}

// recordReasoning appends a model thinking block.
func (app *AgentApp) recordReasoning(text string) {
	app.sessionMu.Lock()
	defer app.sessionMu.Unlock()
	app.sess = ensureSession(app.sess, session.KindAgent, app.agentName, app.currentModelName(), app.skill)
	logSaveError("UI-AGENT", app.sess, app.sess.AddReasoning(text))
}

// recordToolStart appends a tool call start.
func (app *AgentApp) recordToolStart(name, arguments string) {
	app.sessionMu.Lock()
	defer app.sessionMu.Unlock()
	app.sess = ensureSession(app.sess, session.KindAgent, app.agentName, app.currentModelName(), app.skill)
	logSaveError("UI-AGENT", app.sess, app.sess.AddToolStart(name, arguments))
}

// recordToolEnd appends a tool call completion.
func (app *AgentApp) recordToolEnd(name, result string, failed bool) {
	app.sessionMu.Lock()
	defer app.sessionMu.Unlock()
	app.sess = ensureSession(app.sess, session.KindAgent, app.agentName, app.currentModelName(), app.skill)
	logSaveError("UI-AGENT", app.sess, app.sess.AddToolEnd(name, result, failed))
}

// historySnapshot returns the persisted turns for replay into the model.
func (app *AgentApp) historySnapshot() []*schema.Message {
	app.sessionMu.Lock()
	defer app.sessionMu.Unlock()
	return sessionHistory(app.sess)
}

// syncSessionConfig records agent/model/skill switches in the session.
func (app *AgentApp) syncSessionConfig() {
	app.sessionMu.Lock()
	defer app.sessionMu.Unlock()
	if app.sess == nil || app.sess.TurnCount() == 0 {
		return // the header already carries the current settings
	}
	logSaveError("UI-AGENT", app.sess,
		app.sess.SetConfig(app.agentName, app.currentModelName(), app.skill))
}

// currentSessionID returns the active session ID ("" when unset).
func (app *AgentApp) currentSessionID() string {
	app.sessionMu.Lock()
	defer app.sessionMu.Unlock()
	if app.sess == nil {
		return ""
	}
	return app.sess.ID
}

// ensureSession returns s or a new session when none exists yet.
func ensureSession(s *session.Session, kind, agent, model, skill string) *session.Session {
	if s == nil {
		return session.New(kind, agent, model, skill)
	}
	return s
}

// logSaveError reports a best-effort persistence failure.
func logSaveError(component string, s *session.Session, err error) {
	if err == nil {
		return
	}
	id := ""
	if s != nil {
		id = s.ID
	}
	logger.Warn(component, fmt.Sprintf("failed to append to session %s: %v", id, err))
}

// ---- session helpers ----

// sessionHistory converts persisted turns into model messages. Reasoning and
// tool events are UI-only and are not replayed into the model.
func sessionHistory(s *session.Session) []*schema.Message {
	if s == nil {
		return nil
	}
	events := s.Events()
	out := make([]*schema.Message, 0, len(events))
	for _, ev := range events {
		switch ev.Type {
		case session.EventUser:
			if strings.TrimSpace(ev.Content) != "" {
				out = append(out, schema.UserMessage(ev.Content))
			}
		case session.EventAssistant:
			if strings.TrimSpace(ev.Content) != "" {
				out = append(out, schema.AssistantMessage(ev.Content, nil))
			}
		}
	}
	return out
}

// sessionTranscript rebuilds the full transcript, including reasoning blocks
// and tool calls, for display after /resume.
func sessionTranscript(s *session.Session) []Message {
	if s == nil {
		return nil
	}
	events := s.Events()
	out := make([]Message, 0, len(events))

	// pending tool calls by name so a start/end pair shares its timestamps.
	pending := make(map[string][]int)
	for _, ev := range events {
		switch ev.Type {
		case session.EventUser:
			out = append(out, Message{Type: UserMessage, Content: ev.Content})
		case session.EventAssistant:
			out = append(out, Message{Type: AssistantMessage, Content: ev.Content})
		case session.EventReasoning:
			out = append(out, Message{Type: ReasoningMessage, Content: ev.Content})
		case session.EventQuestion:
			text := "❓ " + ev.Content
			if strings.TrimSpace(ev.Result) == "" {
				text += "\n（未回答）"
			} else {
				text += "\n答：" + ev.Result
			}
			out = append(out, Message{Type: InfoMessage, Content: text})
		case session.EventTool:
			if ev.Status == session.ToolStart {
				out = append(out, Message{
					Type:       ToolStartMessage,
					Name:       ev.Name,
					Arguments:  ev.Arguments,
					ToolStatus: ToolWaiting,
					StartTime:  ev.Time.Unix(),
				})
				pending[ev.Name] = append(pending[ev.Name], len(out)-1)
				continue
			}

			status := ToolSuccess
			if ev.Status == session.ToolError || looksLikeError(ev.Result) {
				status = ToolError
			}
			// Pair with the oldest waiting call of the same tool, if any.
			idx := -1
			if waiting := pending[ev.Name]; len(waiting) > 0 {
				idx = waiting[0]
				pending[ev.Name] = waiting[1:]
			}
			if idx < 0 {
				out = append(out, Message{
					Type:       ToolStartMessage,
					Name:       ev.Name,
					ToolStatus: status,
					Result:     ev.Result,
					EndTime:    ev.Time.Unix(),
				})
				continue
			}
			out[idx].ToolStatus = status
			out[idx].Result = ev.Result
			out[idx].EndTime = ev.Time.Unix()
		}
	}

	// A start without an end means the previous process stopped mid-turn.
	// Show it as finished rather than as a tool that is "running..." forever.
	for _, waiting := range pending {
		for _, idx := range waiting {
			if out[idx].ToolStatus == ToolWaiting {
				out[idx].ToolStatus = ToolSuccess
			}
		}
	}
	return out
}

// recentSessionIDs lists the newest session IDs, used as palette candidates.
func recentSessionIDs(kind string, limit int) []string {
	sessions, err := session.List(kind, limit)
	if err != nil {
		return nil
	}
	ids := make([]string, 0, len(sessions))
	for _, s := range sessions {
		ids = append(ids, s.ID)
	}
	return ids
}

// sessionListText renders the recent sessions of one kind for /resume.
func sessionListText(kind string, limit int) string {
	sessions, err := session.List(kind, limit)
	if err != nil {
		return "读取会话历史失败: " + err.Error()
	}
	if len(sessions) == 0 {
		return "暂无历史会话。发送一条消息后会自动保存。"
	}

	var b strings.Builder
	b.WriteString("最近会话：")
	for i, s := range sessions {
		label := s.Agent
		if label == "" {
			label = s.Kind
		}
		fmt.Fprintf(&b, "\n  %d. %s  %s  %s", i+1, s.ID, s.Updated.Local().Format("01-02 15:04"), label)
		if s.Model != "" {
			b.WriteString(" · " + s.Model)
		}
		fmt.Fprintf(&b, " · %d 条", s.TurnCount())
		if msg := s.LastUserMessage(); msg != "" {
			b.WriteString(" · " + truncate(firstLine(msg), 40))
		}
	}
	b.WriteString("\n使用 /resume <编号|id> 恢复会话")
	return b.String()
}

// resolveSession resolves a user argument to a stored session: a 1-based
// number from the list, an exact ID, or an unambiguous ID prefix.
func resolveSession(kind, arg string) (*session.Session, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return nil, fmt.Errorf("empty session reference")
	}
	if n, err := strconv.Atoi(arg); err == nil {
		sessions, err := session.List(kind, 0)
		if err != nil {
			return nil, err
		}
		if n < 1 || n > len(sessions) {
			return nil, fmt.Errorf("会话编号超出范围: %s（共 %d 个）", arg, len(sessions))
		}
		return sessions[n-1], nil
	}
	return session.Find(kind, arg)
}

// sessionClock formats a session time for status lines.
func sessionClock(t time.Time) string {
	return t.Local().Format("2006-01-02 15:04")
}

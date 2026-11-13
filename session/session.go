// Package session persists interactive conversations so they can be resumed
// later from the CLI (/resume).
//
// Storage is append-only JSON Lines: one session is one <id>.jsonl file under
// ~/.eino-cli/sessions, and every turn is written as its own line. User and
// assistant text, model reasoning, tool calls, ask_user exchanges and task-list
// snapshots are all recorded; nothing is ever rewritten, so an interrupted
// turn leaves the file intact.
package session

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tk103331/eino-cli/state"
)

// Event types stored in a session file.
const (
	EventSession   = "session"   // header line: id, kind, agent, model, skill
	EventConfig    = "config"    // later agent/model/skill change
	EventUser      = "user"      // user turn
	EventAssistant = "assistant" // assistant turn
	EventReasoning = "reasoning" // model thinking (may precede a tool call or answer)
	EventTool      = "tool"      // tool call start/end
	EventQuestion  = "question"  // ask_user question (Content) and answer (Result)
	EventTodo      = "todo"      // full task list snapshot
)

// Tool statuses for EventTool.
const (
	ToolStart = "start"
	ToolEnd   = "end"
	ToolError = "error"
)

// Session kinds. KindChat is kept for reading sessions written by older
// versions that still had a standalone chat mode; new sessions are agents.
const (
	KindAgent = "agent"
	KindChat  = "chat"
)

// Turn roles used by the recordTurn helpers.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// Task statuses persisted in todo events.
const (
	TodoStatusPending    = "pending"
	TodoStatusInProgress = "in_progress"
	TodoStatusCompleted  = "completed"
)

// Event is one line of a session file. Fields are used depending on Type.
type Event struct {
	Type      string     `json:"type"`
	Time      time.Time  `json:"time"`
	ID        string     `json:"id,omitempty"`
	Kind      string     `json:"kind,omitempty"`
	Agent     string     `json:"agent,omitempty"`
	Model     string     `json:"model,omitempty"`
	Skill     string     `json:"skill,omitempty"`
	Content   string     `json:"content,omitempty"`
	Name      string     `json:"name,omitempty"`
	Arguments string     `json:"arguments,omitempty"`
	Result    string     `json:"result,omitempty"`
	Status    string     `json:"status,omitempty"`
	Todos     []TodoItem `json:"todos,omitempty"`
}

// TodoItem is one persisted task-list entry.
type TodoItem struct {
	Content string `json:"content"`
	Status  string `json:"status"`
}

// Session is a persisted, append-only conversation.
type Session struct {
	ID      string
	Kind    string
	Agent   string
	Model   string
	Skill   string
	Created time.Time
	Updated time.Time

	mu      sync.Mutex
	events  []Event
	started bool // header line already written
}

// New creates an empty session with a time-ordered, human-readable ID. The
// file is created lazily on the first append.
func New(kind, agent, model, skill string) *Session {
	now := time.Now()
	return &Session{
		ID:      newID(now),
		Kind:    kind,
		Agent:   agent,
		Model:   model,
		Skill:   skill,
		Created: now,
		Updated: now,
	}
}

// newID builds an ID that sorts chronologically and stays short enough to type:
// 20261113-160405-a1b2c3.
func newID(t time.Time) string {
	var b [3]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fall back to nanoseconds when the entropy source is unavailable.
		return fmt.Sprintf("%s-%06d", t.Format("20060102-150405"), t.Nanosecond()%1_000_000)
	}
	return t.Format("20060102-150405") + "-" + hex.EncodeToString(b[:])
}

// AddUser appends a user turn.
func (s *Session) AddUser(text string) error {
	return s.addText(EventUser, text)
}

// AddAssistant appends an assistant turn.
func (s *Session) AddAssistant(text string) error {
	return s.addText(EventAssistant, text)
}

// AddReasoning appends a model thinking block.
func (s *Session) AddReasoning(text string) error {
	return s.addText(EventReasoning, text)
}

func (s *Session) addText(typ, text string) error {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return s.Append(Event{Type: typ, Content: text})
}

// AddToolStart appends the start of a tool call.
func (s *Session) AddToolStart(name, arguments string) error {
	if strings.TrimSpace(name) == "" {
		return nil
	}
	return s.Append(Event{Type: EventTool, Name: name, Arguments: arguments, Status: ToolStart})
}

// AddToolEnd appends the completion of a tool call. failed reports an error
// result, which is preserved as the event status.
func (s *Session) AddToolEnd(name, result string, failed bool) error {
	if strings.TrimSpace(name) == "" {
		return nil
	}
	status := ToolEnd
	if failed {
		status = ToolError
	}
	return s.Append(Event{Type: EventTool, Name: name, Result: result, Status: status})
}

// SetConfig records an agent/model/skill change. Replaying the file keeps the
// values from the last config event.
func (s *Session) SetConfig(agent, model, skill string) error {
	return s.Append(Event{Type: EventConfig, Agent: agent, Model: model, Skill: skill})
}

// AddQuestion records an ask_user question and the user's answer ("" when the
// user cancelled).
func (s *Session) AddQuestion(question, answer string) error {
	question = strings.TrimSpace(question)
	if question == "" {
		return nil
	}
	return s.Append(Event{Type: EventQuestion, Content: question, Result: answer})
}

// AddTodos records a complete task-list snapshot.
func (s *Session) AddTodos(items []TodoItem) error {
	clone := make([]TodoItem, len(items))
	copy(clone, items)
	return s.Append(Event{Type: EventTodo, Todos: clone})
}

// Todos returns the most recent task-list snapshot, or nil.
func (s *Session) Todos() []TodoItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.events) - 1; i >= 0; i-- {
		if s.events[i].Type == EventTodo {
			out := make([]TodoItem, len(s.events[i].Todos))
			copy(out, s.events[i].Todos)
			return out
		}
	}
	return nil
}

// Append writes one event as a JSON line. The session header is emitted
// automatically before the first event.
func (s *Session) Append(ev Event) error {
	if !validID(s.ID) {
		return fmt.Errorf("invalid session id: %q", s.ID)
	}
	path := filePath(s.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if ev.Time.IsZero() {
		ev.Time = time.Now()
	}

	var buf []byte
	if !s.started {
		header := Event{
			Type:  EventSession,
			Time:  s.Created,
			ID:    s.ID,
			Kind:  s.Kind,
			Agent: s.Agent,
			Model: s.Model,
			Skill: s.Skill,
		}
		line, err := json.Marshal(header)
		if err != nil {
			return err
		}
		buf = append(buf, line...)
		buf = append(buf, '\n')
	}

	line, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	buf = append(buf, line...)
	buf = append(buf, '\n')

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(buf)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}

	s.started = true
	s.events = append(s.events, ev)
	if ev.Time.After(s.Updated) {
		s.Updated = ev.Time
	}
	if ev.Type == EventConfig {
		s.Agent = ev.Agent
		s.Model = ev.Model
		s.Skill = ev.Skill
	}
	return nil
}

// Events returns a copy of the recorded events.
func (s *Session) Events() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Event, len(s.events))
	copy(out, s.events)
	return out
}

// TurnCount returns the number of user + assistant turns.
func (s *Session) TurnCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return turnCount(s.events)
}

// LastUserMessage returns the most recent user turn, or "".
func (s *Session) LastUserMessage() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.events) - 1; i >= 0; i-- {
		if s.events[i].Type == EventUser {
			return s.events[i].Content
		}
	}
	return ""
}

func turnCount(events []Event) int {
	n := 0
	for _, ev := range events {
		if ev.Type == EventUser || ev.Type == EventAssistant {
			n++
		}
	}
	return n
}

// Load reads one session by exact ID, replaying every JSON line.
func Load(id string) (*Session, error) {
	if !validID(id) {
		return nil, fmt.Errorf("invalid session id: %q", id)
	}
	f, err := os.Open(filePath(id))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	s := &Session{ID: id, started: true}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024) // tool results can be long
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue // skip a corrupted line instead of losing the session
		}
		if ev.Type == EventSession {
			if ev.ID != "" {
				s.ID = ev.ID
			}
			s.Kind = ev.Kind
			s.Agent = ev.Agent
			s.Model = ev.Model
			s.Skill = ev.Skill
			s.Created = ev.Time
		}
		if ev.Type == EventConfig {
			s.Agent = ev.Agent
			s.Model = ev.Model
			s.Skill = ev.Skill
		}
		s.events = append(s.events, ev)
		if ev.Time.After(s.Updated) {
			s.Updated = ev.Time
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if s.Kind == "" {
		s.Kind = KindAgent
	}
	if s.Created.IsZero() {
		s.Created = s.Updated
	}
	return s, nil
}

// List returns sessions of the given kind (all kinds when kind is empty),
// newest first, capped at limit (<= 0 means no cap).
func List(kind string, limit int) ([]*Session, error) {
	dir := Dir()
	if dir == "" {
		return nil, errors.New("cannot determine session directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	sessions := make([]*Session, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		id := strings.TrimSuffix(name, ".jsonl")
		if !validID(id) {
			continue
		}
		s, err := Load(id)
		if err != nil {
			continue // ignore unreadable/corrupted files
		}
		if kind != "" && s.Kind != kind {
			continue
		}
		sessions = append(sessions, s)
	}

	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].Updated.Equal(sessions[j].Updated) {
			return sessions[i].ID > sessions[j].ID
		}
		return sessions[i].Updated.After(sessions[j].Updated)
	})
	if limit > 0 && len(sessions) > limit {
		sessions = sessions[:limit]
	}
	return sessions, nil
}

// Find resolves a session by exact ID or unique ID prefix within a kind.
func Find(kind, prefix string) (*Session, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return nil, errors.New("empty session id")
	}
	if validID(prefix) {
		if s, err := Load(prefix); err == nil {
			if kind == "" || s.Kind == kind {
				return s, nil
			}
		}
	}

	sessions, err := List(kind, 0)
	if err != nil {
		return nil, err
	}
	var matches []*Session
	for _, s := range sessions {
		if strings.HasPrefix(s.ID, prefix) {
			matches = append(matches, s)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("session not found: %s", prefix)
	case 1:
		return matches[0], nil
	default:
		return nil, fmt.Errorf("ambiguous session id %q (%d matches)", prefix, len(matches))
	}
}

// Dir returns the session storage directory.
func Dir() string {
	d := state.Dir()
	if d == "" {
		return ""
	}
	return filepath.Join(d, "sessions")
}

// filePath returns the JSONL path for a session ID.
func filePath(id string) string {
	return filepath.Join(Dir(), id+".jsonl")
}

// validID restricts IDs to characters that are safe in a file name.
func validID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, r := range id {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r == '-':
		default:
			return false
		}
	}
	return true
}

package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

// readLines returns the raw JSONL lines of a session file.
func readLines(t *testing.T, id string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(Dir(), id+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func TestSessionAppendOnlyRoundTrip(t *testing.T) {
	testHome(t)

	s := New(KindAgent, "builtin", "ds41", "translator")
	if s.ID == "" || !validID(s.ID) {
		t.Fatalf("bad session id: %q", s.ID)
	}
	if err := s.AddUser("hello"); err != nil {
		t.Fatal(err)
	}
	// The file appears after the first append and starts with a header line.
	if got := len(readLines(t, s.ID)); got != 2 {
		t.Fatalf("expected header + user line, got %d lines", got)
	}

	if err := s.AddReasoning("let me think"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddToolStart("search", `{"q":"go"}`); err != nil {
		t.Fatal(err)
	}
	if err := s.AddToolEnd("search", "result text", false); err != nil {
		t.Fatal(err)
	}
	if err := s.AddAssistant("hi there"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddUser("   "); err != nil {
		t.Fatal(err) // blank turns are skipped
	}

	loaded, err := Load(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Agent != "builtin" || loaded.Model != "ds41" || loaded.Skill != "translator" {
		t.Fatalf("metadata lost: %+v", loaded)
	}
	events := loaded.Events()
	types := make([]string, 0, len(events))
	for _, ev := range events {
		types = append(types, ev.Type)
	}
	want := []string{EventSession, EventUser, EventReasoning, EventTool, EventTool, EventAssistant}
	if strings.Join(types, ",") != strings.Join(want, ",") {
		t.Fatalf("event order = %v, want %v", types, want)
	}
	if got := loaded.TurnCount(); got != 2 {
		t.Fatalf("turn count = %d", got)
	}
	if got := loaded.LastUserMessage(); got != "hello" {
		t.Fatalf("last user message = %q", got)
	}
	if events[3].Name != "search" || events[4].Status != ToolEnd {
		t.Fatalf("tool events wrong: %+v %+v", events[3], events[4])
	}

	// Appending more never rewrites the earlier lines.
	before := readLines(t, s.ID)
	if err := s.AddUser("another"); err != nil {
		t.Fatal(err)
	}
	after := readLines(t, s.ID)
	if len(after) != len(before)+1 {
		t.Fatalf("append should add exactly one line: %d -> %d", len(before), len(after))
	}
	for i, line := range before {
		if after[i] != line {
			t.Fatalf("earlier line %d changed", i)
		}
	}
}

func TestTodoAndQuestionEvents(t *testing.T) {
	testHome(t)

	s := New(KindAgent, "a", "m", "")
	if err := s.AddTodos([]TodoItem{{Content: "first", Status: TodoStatusPending}}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddTodos([]TodoItem{
		{Content: "first", Status: TodoStatusCompleted},
		{Content: "second", Status: TodoStatusInProgress},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddQuestion("which db?", "sqlite"); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	// The latest snapshot wins.
	todos := loaded.Todos()
	if len(todos) != 2 || todos[0].Status != TodoStatusCompleted || todos[1].Status != TodoStatusInProgress {
		t.Fatalf("latest todo snapshot wrong: %+v", todos)
	}

	var found bool
	for _, ev := range loaded.Events() {
		if ev.Type == EventQuestion && ev.Content == "which db?" && ev.Result == "sqlite" {
			found = true
		}
	}
	if !found {
		t.Fatal("question event missing")
	}
}

func TestConfigEventOverridesHeader(t *testing.T) {
	testHome(t)

	s := New(KindAgent, "a", "m1", "")
	if err := s.AddUser("q"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetConfig("b", "m2", "translator"); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Agent != "b" || loaded.Model != "m2" || loaded.Skill != "translator" {
		t.Fatalf("config event not applied: %+v", loaded)
	}
}

func TestListAndFind(t *testing.T) {
	testHome(t)

	older := New(KindAgent, "a", "m", "")
	older.ID = "20200101-000000-aaaaaa" // stable ID so the prefix test is unambiguous
	if err := older.AddUser("old question"); err != nil {
		t.Fatal(err)
	}
	// Timestamps differ by construction when events are appended.
	time.Sleep(2 * time.Millisecond)

	newer := New(KindChat, "", "m2", "")
	if err := newer.AddUser("new question"); err != nil {
		t.Fatal(err)
	}

	all, err := List("", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(all))
	}
	if all[0].Updated.Before(all[1].Updated) {
		t.Fatalf("list not newest-first: %+v", all)
	}

	agents, err := List(KindAgent, 0)
	if err != nil || len(agents) != 1 || agents[0].ID != older.ID {
		t.Fatalf("kind filter wrong: %+v (err=%v)", agents, err)
	}

	if limited, _ := List("", 1); len(limited) != 1 {
		t.Fatalf("limit wrong: %+v", limited)
	}

	found, err := Find(KindAgent, older.ID[:8])
	if err != nil || found.ID != older.ID {
		t.Fatalf("prefix find failed: %+v (err=%v)", found, err)
	}
	if _, err := Find(KindAgent, "nope"); err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestListEmpty(t *testing.T) {
	testHome(t)
	sessions, err := List("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected no sessions, got %d", len(sessions))
	}
}

func TestCorruptedLineSkipped(t *testing.T) {
	testHome(t)

	s := New(KindAgent, "a", "m", "")
	if err := s.AddUser("survives"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(Dir(), s.ID+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("{not valid json\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	loaded, err := Load(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.LastUserMessage(); got != "survives" {
		t.Fatalf("session lost after corrupted line: %q", got)
	}
}

func TestInvalidIDs(t *testing.T) {
	testHome(t)
	for _, id := range []string{"", "../escape", "a/b", "has space"} {
		if _, err := Load(id); err == nil {
			t.Fatalf("Load(%q) should fail", id)
		}
	}
	s := &Session{ID: "../escape"}
	if err := s.Append(Event{Type: EventUser, Content: "x"}); err == nil {
		t.Fatal("Append with unsafe id should fail")
	}
}

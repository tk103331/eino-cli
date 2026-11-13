package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/tools"
)

func feedAll(f *ThinkFilter, chunks ...string) (visible, thought string) {
	for _, c := range chunks {
		v, th := f.Feed(c)
		visible += v
		thought += th
	}
	v, th := f.Flush()
	return visible + v, thought + th
}

func TestThinkFilterPassthrough(t *testing.T) {
	f := NewThinkFilter()
	v, th := feedAll(f, "hello ", "world")
	if v != "hello world" || th != "" {
		t.Fatalf("passthrough failed: visible=%q thought=%q", v, th)
	}
}

func TestThinkFilterBasic(t *testing.T) {
	f := NewThinkFilter()
	v, th := feedAll(f, "answer1 <thi", "nk>reasoning here</thi", "nk> answer2")
	if v != "answer1  answer2" || th != "reasoning here" {
		t.Fatalf("basic split failed: visible=%q thought=%q", v, th)
	}
}

func TestThinkFilterWholeChunk(t *testing.T) {
	f := NewThinkFilter()
	v, th := feedAll(f, "<think>let me think</think>final")
	if v != "final" || th != "let me think" {
		t.Fatalf("whole chunk failed: visible=%q thought=%q", v, th)
	}
}

func TestThinkFilterUnterminated(t *testing.T) {
	f := NewThinkFilter()
	v, th := feedAll(f, "visible <think>still thinking")
	if v != "visible " || th != "still thinking" {
		t.Fatalf("unterminated failed: visible=%q thought=%q", v, th)
	}
}

func TestThinkFilterNoLeak(t *testing.T) {
	f := NewThinkFilter()
	v, _ := feedAll(f, "<think>secret plan</think>public answer")
	if strings.Contains(v, "secret") || strings.Contains(v, "think") {
		t.Fatalf("reasoning leaked into visible: %q", v)
	}
}

func TestBuildMessagesWithHistory(t *testing.T) {
	r := &ReactAgent{config: &config.Agent{System: "sys"}}
	history := []*schema.Message{
		schema.UserMessage("first"),
		schema.AssistantMessage("answer", nil),
		nil, // nil entries must be skipped
	}
	msgs := r.buildMessages(history, "second")

	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}
	if msgs[0].Role != schema.System || msgs[0].Content != "sys" {
		t.Fatalf("system message wrong: %+v", msgs[0])
	}
	if msgs[1].Content != "first" || msgs[2].Content != "answer" || msgs[3].Content != "second" {
		t.Fatalf("history ordering wrong: %+v", msgs)
	}

	// No system prompt configured: the list starts with history.
	r = &ReactAgent{config: &config.Agent{}}
	if msgs := r.buildMessages(nil, "only"); len(msgs) != 1 || msgs[0].Content != "only" {
		t.Fatalf("no-system build wrong: %+v", msgs)
	}
}

func TestWithActiveModelCarriesTheRunModel(t *testing.T) {
	r := &ReactAgent{config: &config.Agent{Model: "ds"}}
	ctx := r.withActiveModel(context.Background())
	if got := tools.ActiveModel(ctx); got != "ds" {
		t.Fatalf("active model = %q, want ds", got)
	}
}

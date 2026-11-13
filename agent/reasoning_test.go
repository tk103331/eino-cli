package agent

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/tk103331/eino-cli/config"
)

// reasoningModel streams a reasoning chunk followed by the visible answer,
// the way reasoning models (e.g. deepseek-reasoner) do.
type reasoningModel struct {
	chunks []*schema.Message
}

func (m *reasoningModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return m.chunks[len(m.chunks)-1], nil
}

func (m *reasoningModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return schema.StreamReaderFromArray(m.chunks), nil
}

func (m *reasoningModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func newReasoningAgent(t *testing.T, chunks []*schema.Message) *ReactAgent {
	t.Helper()
	ra, err := react.NewAgent(context.Background(), &react.AgentConfig{
		ToolCallingModel: &reasoningModel{chunks: chunks},
	})
	if err != nil {
		t.Fatal(err)
	}
	return &ReactAgent{
		agentName: "test",
		config:    &config.Agent{Model: "reasoning"},
		agent:     ra,
		ctx:       context.Background(),
	}
}

// ReasoningContent from the model must reach the UI as reasoning chunks; it
// used to be dropped because only <think> tags in Content were understood.
func TestChatStreamEmitsReasoningContent(t *testing.T) {
	r := newReasoningAgent(t, []*schema.Message{
		schema.AssistantMessage("", nil),
		{Role: schema.Assistant, ReasoningContent: "先想一下", Content: ""},
		{Role: schema.Assistant, ReasoningContent: "", Content: "答案"},
	})

	var reasoning, answer strings.Builder
	err := r.ChatStream(context.Background(), nil, "hi", func(chunk *StreamChunk) {
		switch chunk.Type {
		case "reasoning":
			reasoning.WriteString(chunk.Content)
		case "content":
			answer.WriteString(chunk.Content)
		}
	}, nil)
	if err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("ChatStream: %v", err)
	}
	if !strings.Contains(reasoning.String(), "先想一下") {
		t.Fatalf("reasoning missing: %q", reasoning.String())
	}
	if !strings.Contains(answer.String(), "答案") {
		t.Fatalf("answer missing: %q", answer.String())
	}
}

// ChatWithCallback must surface reasoning with the same "reasoning" event the
// TUI already handles.
func TestChatWithCallbackEmitsReasoningContent(t *testing.T) {
	r := newReasoningAgent(t, []*schema.Message{
		schema.AssistantMessage("", nil),
		{Role: schema.Assistant, ReasoningContent: "思考中", Content: "回答"},
	})

	var events []string
	_, err := r.ChatWithCallback(context.Background(), nil, "hi", func(msg interface{}) {
		if m, ok := msg.(map[string]interface{}); ok && m["type"] == "reasoning" {
			events = append(events, m["content"].(string))
		}
	})
	if err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("ChatWithCallback: %v", err)
	}
	if len(events) != 1 || events[0] != "思考中" {
		t.Fatalf("reasoning events = %v", events)
	}
}

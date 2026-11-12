package agent

import (
	"strings"
	"testing"
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

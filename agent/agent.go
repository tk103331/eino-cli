package agent

import (
	"context"
	"strings"
)

// StreamChunk represents a data chunk for streaming output
type StreamChunk struct {
	Content string
	Type    string // "content", "reasoning", "tool_start", "tool_end", "error"
	Tool    string // Tool name (only used for tool-related messages)
}

// Agent defines the agent interface used in the CLI
type Agent interface {
	// Run runs the agent
	Run(prompt string) error
	// Chat performs conversation, returns response content
	Chat(ctx context.Context, prompt string) (string, error)
	// ChatWithCallback performs conversation, supporting tool call callbacks
	ChatWithCallback(ctx context.Context, prompt string, callback func(interface{})) (string, error)
	// ChatStream performs streaming conversation, handles streaming output through chunk callback
	ChatStream(ctx context.Context, prompt string, chunkCallback func(*StreamChunk), toolCallback func(interface{})) error
}

// Some OpenAI-compatible gateways (e.g. MiniMax via a proxy) embed the
// model's reasoning in delta.content delimited by think tags instead of using
// the standard reasoning_content field. The markers are assembled from parts
// so the intent stays obvious even in code review.
const (
	thinkTag   = "think"
	thinkOpen  = "<" + thinkTag + ">"
	thinkClose = "</" + thinkTag + ">"
)

// ThinkFilter splits streamed content into visible answer text and reasoning
// text. Markers may span chunk boundaries; partial markers are buffered until
// they can be classified.
type ThinkFilter struct {
	inThink bool
	buf     string
}

// NewThinkFilter creates a filter state for one streaming turn.
func NewThinkFilter() *ThinkFilter { return &ThinkFilter{} }

// Feed processes one chunk and returns the visible answer content and the
// reasoning content extracted from it (either may be empty).
func (f *ThinkFilter) Feed(s string) (visible, thought string) {
	f.buf += s
	var vis, th strings.Builder
	for {
		if f.inThink {
			idx := strings.Index(f.buf, thinkClose)
			if idx < 0 {
				// Not closed yet: keep a possible partial close marker tail.
				keep := partialSuffixLen(f.buf, thinkClose)
				th.WriteString(f.buf[:len(f.buf)-keep])
				f.buf = f.buf[len(f.buf)-keep:]
				return vis.String(), th.String()
			}
			th.WriteString(f.buf[:idx])
			f.buf = f.buf[idx+len(thinkClose):]
			f.inThink = false
			f.buf = strings.TrimLeft(f.buf, "\n")
			continue
		}
		idx := strings.Index(f.buf, thinkOpen)
		if idx < 0 {
			keep := partialSuffixLen(f.buf, thinkOpen)
			vis.WriteString(f.buf[:len(f.buf)-keep])
			f.buf = f.buf[len(f.buf)-keep:]
			return vis.String(), th.String()
		}
		vis.WriteString(f.buf[:idx])
		f.buf = f.buf[idx+len(thinkOpen):]
		f.inThink = true
	}
}

// Flush drains any buffered content when the stream ends. An unterminated
// thinking section is returned as reasoning rather than dropped.
func (f *ThinkFilter) Flush() (visible, thought string) {
	if f.inThink {
		th := f.buf
		f.buf = ""
		return "", th
	}
	v := f.buf
	f.buf = ""
	return v, ""
}

// partialSuffixLen returns the length of the longest suffix of s that is a
// proper prefix of marker, so a split marker can be reassembled later.
func partialSuffixLen(s, marker string) int {
	max := len(marker) - 1
	if max > len(s) {
		max = len(s)
	}
	for l := max; l > 0; l-- {
		if strings.HasPrefix(marker, s[len(s)-l:]) {
			return l
		}
	}
	return 0
}

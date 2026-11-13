package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/tools/approval"
)

// classifiedTool flags calls whose arguments contain "danger".
type classifiedTool struct {
	name string
	runs int
}

func (c *classifiedTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: c.name}, nil
}

func (c *classifiedTool) InvokableRun(ctx context.Context, arguments string, _ ...tool.Option) (string, error) {
	c.runs++
	return "ran " + c.name, nil
}

func (c *classifiedTool) ApprovalRequest(arguments string) (approval.Request, bool) {
	if !strings.Contains(arguments, "danger") {
		return approval.Request{}, false
	}
	return approval.Request{Summary: "危险操作"}, true
}

// plainTool has no classifier.
type plainTool struct {
	name string
	runs int
}

func (p *plainTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: p.name}, nil
}

func (p *plainTool) InvokableRun(ctx context.Context, arguments string, _ ...tool.Option) (string, error) {
	p.runs++
	return "ran " + p.name, nil
}

func TestApprovalAllowsOnce(t *testing.T) {
	inner := &classifiedTool{name: "shell"}
	wrapped, err := WithApproval(inner, approval.Policy{Mode: approval.ModeDangerous})
	if err != nil {
		t.Fatal(err)
	}
	host := &fakeHost{approved: true}
	out, err := wrapped.InvokableRun(WithHost(context.Background(), host), `{"command":"danger"}`)
	if err != nil || out != "ran shell" {
		t.Fatalf("run = %q, %v", out, err)
	}
	if host.approvalCalls != 1 || host.approvalReq.Summary != "危险操作" || host.approvalReq.Tool != "shell" {
		t.Fatalf("approval request = %+v (calls %d)", host.approvalReq, host.approvalCalls)
	}
	if inner.runs != 1 {
		t.Fatalf("tool should have run once, ran %d", inner.runs)
	}
}

func TestApprovalDenyKeepsTurnAlive(t *testing.T) {
	inner := &classifiedTool{name: "shell"}
	wrapped, _ := WithApproval(inner, approval.Policy{Mode: approval.ModeDangerous})
	host := &fakeHost{approved: false}

	out, err := wrapped.InvokableRun(WithHost(context.Background(), host), `{"command":"danger"}`)
	if err != nil {
		t.Fatalf("denial must not be an error: %v", err)
	}
	if !strings.Contains(out, "denied permission") {
		t.Fatalf("denial result = %q", out)
	}
	if inner.runs != 0 {
		t.Fatalf("denied tool must not run, ran %d", inner.runs)
	}
}

func TestApprovalSkipsSafeCalls(t *testing.T) {
	inner := &classifiedTool{name: "shell"}
	wrapped, _ := WithApproval(inner, approval.Policy{Mode: approval.ModeDangerous})
	host := &fakeHost{approved: false}

	out, err := wrapped.InvokableRun(WithHost(context.Background(), host), `{"command":"ls"}`)
	if err != nil || out != "ran shell" {
		t.Fatalf("safe call = %q, %v", out, err)
	}
	if host.approvalCalls != 0 {
		t.Fatalf("safe call should not prompt (calls %d)", host.approvalCalls)
	}
}

func TestApprovalAllModeCoversPlainTools(t *testing.T) {
	inner := &plainTool{name: "search"}
	wrapped, _ := WithApproval(inner, approval.Policy{Mode: approval.ModeAll})
	host := &fakeHost{approved: true}

	if _, err := wrapped.InvokableRun(WithHost(context.Background(), host), `{}`); err != nil {
		t.Fatal(err)
	}
	if host.approvalCalls != 1 || host.approvalReq.Summary != "调用工具 search" {
		t.Fatalf("all mode prompt = %+v (calls %d)", host.approvalReq, host.approvalCalls)
	}
}

func TestApprovalTransparentCases(t *testing.T) {
	// No host: run/ACP mode keeps working without prompts.
	inner := &classifiedTool{name: "shell"}
	wrapped, _ := WithApproval(inner, approval.Policy{Mode: approval.ModeDangerous})
	if out, err := wrapped.InvokableRun(context.Background(), `{"command":"danger"}`); err != nil || out != "ran shell" {
		t.Fatalf("hostless run = %q, %v", out, err)
	}

	// Mode off returns the original tool.
	plain := &plainTool{name: "shell"}
	same, err := WithApproval(plain, approval.Policy{Mode: approval.ModeOff})
	if err != nil || same != plain {
		t.Fatalf("off mode should not wrap: %v %v", same, err)
	}

	// Interactive tools are never gated.
	ask, err := NewAskUserTool(AskUserToolName, config.Tool{})
	if err != nil {
		t.Fatal(err)
	}
	unwrapped, err := WithApproval(ask, approval.Policy{Mode: approval.ModeAll})
	if err != nil || unwrapped != ask {
		t.Fatalf("ask_user must never be gated: %v %v", unwrapped, err)
	}

	// The allow list exempts a dangerous tool.
	allowed := &classifiedTool{name: "shell"}
	wrapped, _ = WithApproval(allowed, approval.Policy{
		Mode:  approval.ModeDangerous,
		Allow: map[string]bool{"shell": true},
	})
	host := &fakeHost{approved: false}
	if out, err := wrapped.InvokableRun(WithHost(context.Background(), host), `{"command":"danger"}`); err != nil || out != "ran shell" {
		t.Fatalf("allow list = %q, %v", out, err)
	}
	if host.approvalCalls != 0 {
		t.Fatalf("allow-listed tool should not prompt (calls %d)", host.approvalCalls)
	}
}

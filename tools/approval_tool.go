package tools

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/tk103331/eino-cli/tools/approval"
)

// approvalTool wraps a tool so its dangerous calls are confirmed by the user
// before they run.
//
// The wrapper is transparent when there is no interactive host in the context
// (run/ACP mode) or when the policy does not cover the tool, so the same agent
// definition works with and without a TUI.
type approvalTool struct {
	inner  tool.InvokableTool
	name   string
	policy approval.Policy
}

// WithApproval wraps inner with the given approval policy. Tools that only talk
// to the user (ask_user, todolist) are never gated.
func WithApproval(inner tool.InvokableTool, policy approval.Policy) (tool.InvokableTool, error) {
	if inner == nil {
		return nil, fmt.Errorf("approval: nil tool")
	}
	info, err := inner.Info(context.Background())
	if err != nil {
		return nil, fmt.Errorf("approval: cannot read tool info: %w", err)
	}
	name := ""
	if info != nil {
		name = info.Name
	}
	if name == AskUserToolName || name == TodoToolName {
		return inner, nil
	}
	if policy.ModeOrDefault() == approval.ModeOff {
		return inner, nil
	}
	return &approvalTool{inner: inner, name: name, policy: policy}, nil
}

func (t *approvalTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.inner.Info(ctx)
}

func (t *approvalTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	classifier, _ := t.inner.(approval.Classifier)
	req, needs := t.policy.NeedsApproval(t.name, argumentsInJSON, classifier)
	if !needs {
		return t.inner.InvokableRun(ctx, argumentsInJSON, opts...)
	}

	host := HostFromContext(ctx)
	if host == nil {
		// Non-interactive run (run/ACP): nobody can answer, so keep the tool
		// usable and let the caller's own sandbox rules apply.
		return t.inner.InvokableRun(ctx, argumentsInJSON, opts...)
	}

	allowed, err := host.ApproveTool(ctx, req)
	if err != nil {
		return "", err
	}
	if !allowed {
		// A result (not an error) keeps the ReAct turn alive so the model can
		// propose a different, safe plan.
		return fmt.Sprintf("error: the user denied permission to run tool %q", t.name), nil
	}
	return t.inner.InvokableRun(ctx, argumentsInJSON, opts...)
}

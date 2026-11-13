package agent

import (
	"github.com/cloudwego/eino/components/tool"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/tools"
	"github.com/tk103331/eino-cli/tools/approval"
)

// createTool creates tool instances
func createTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	return tools.CreateTool(name, cfg)
}

// approvalPolicy builds the tool-approval policy from global settings. Without
// explicit configuration dangerous tools (shell commands, file writes) ask for
// confirmation in interactive sessions.
func approvalPolicy() approval.Policy {
	policy := approval.Policy{Mode: approval.ModeDangerous}
	cfg := config.GetConfig()
	if cfg == nil || cfg.Settings.Approval == nil {
		return policy
	}
	policy.Mode = cfg.Settings.Approval.Mode
	policy.Allow = nameSet(cfg.Settings.Approval.Allow)
	policy.Always = nameSet(cfg.Settings.Approval.Always)
	return policy
}

// wrapApprovals applies the approval policy to every configured tool. The
// wrapper is transparent outside interactive sessions, where no host is
// attached to the context.
func wrapApprovals(configured []tool.BaseTool) ([]tool.BaseTool, error) {
	policy := approvalPolicy()
	for i, tl := range configured {
		invokable, ok := tl.(tool.InvokableTool)
		if !ok {
			continue
		}
		wrapped, err := tools.WithApproval(invokable, policy)
		if err != nil {
			return configured, err
		}
		configured[i] = wrapped
	}
	return configured, nil
}

// nameSet converts a name list into a lookup set.
func nameSet(names []string) map[string]bool {
	if len(names) == 0 {
		return nil
	}
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}

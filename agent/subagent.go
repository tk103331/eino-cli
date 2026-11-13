package agent

import (
	"context"
	"fmt"

	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/tools"
)

// The subagent tool delegates to this package, which owns model and agent
// construction. Registering it from an init keeps tools free of an import
// cycle back into agent.
func init() {
	tools.SetSubagentRunner(runSubagent)
}

// runSubagent builds a child agent — a configured one, or a temporary one from
// the requested system prompt/model/tools — and runs a single turn without
// history. The child inherits ctx, so approvals, ask_user and cancellation
// still reach the interactive host.
func runSubagent(ctx context.Context, spec tools.SubagentSpec) (string, error) {
	cfg := config.GetConfig()
	if cfg == nil {
		return "", fmt.Errorf("global configuration not initialized")
	}

	name := "subagent"
	childCfg := config.Agent{}
	if spec.Agent != "" {
		configured, ok := cfg.Agents[spec.Agent]
		if !ok {
			return "", fmt.Errorf("agent configuration does not exist: %s", spec.Agent)
		}
		name = spec.Agent
		childCfg = configured
		// A configured agent that follows the default model inherits the
		// parent's current model; an explicitly pinned model stays in charge.
		if spec.Model != "" && (configured.Model == "" || configured.Model == cfg.DefaultModelName()) {
			if _, ok := cfg.Models[spec.Model]; !ok {
				return "", fmt.Errorf("model configuration does not exist: %s", spec.Model)
			}
			childCfg.Model = spec.Model
		}
	} else {
		model, err := subagentModel(cfg, spec.Model)
		if err != nil {
			return "", err
		}
		for _, toolName := range spec.Tools {
			if _, ok := cfg.Tools[toolName]; !ok {
				return "", fmt.Errorf("tool configuration does not exist: %s", toolName)
			}
		}
		childCfg = config.Agent{
			System: spec.System,
			Model:  model,
			Tools:  append([]string(nil), spec.Tools...),
		}
	}

	child := NewReactAgent(name, &childCfg)
	if err := child.Init(); err != nil {
		return "", err
	}
	return child.Chat(ctx, nil, spec.Prompt)
}

// subagentModel resolves the model for a temporary agent: the requested one,
// else the configured default, else the first model alphabetically.
func subagentModel(cfg *config.Config, requested string) (string, error) {
	if requested != "" {
		if _, ok := cfg.Models[requested]; !ok {
			return "", fmt.Errorf("model configuration does not exist: %s", requested)
		}
		return requested, nil
	}
	model := cfg.DefaultModelName()
	if model == "" {
		return "", fmt.Errorf("no model configured for a temporary subagent")
	}
	return model, nil
}

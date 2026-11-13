package cmd

import (
	"fmt"

	"github.com/cloudwego/eino-ext/callbacks/langfuse"
	"github.com/cloudwego/eino/callbacks"
	"github.com/spf13/cobra"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/state"
	"github.com/tk103331/eino-cli/tools"
	"github.com/tk103331/eino-cli/ui/agent"
)

// builtinAgentName is the implicit agent used when the config does not define
// any agent and no previous agent is available.
const builtinAgentName = "Eino"

// builtinAgentSystem is the system prompt of the built-in agent.
const builtinAgentSystem = "You are Eino CLI, a concise and helpful AI assistant. " +
	"Answer clearly and briefly. Use tools only when they are available and relevant."

// builtinToolDefaults are the tools enabled on the built-in agent by default.
// The agent refers to them by name; the config entry is only created when the
// user has not defined one, so custom settings always win.
var builtinToolDefaults = []struct {
	name string
	typ  string
}{
	{"commandline", "commandline"},
	{"filesystem", "filesystem"},
	{"httprequest", "httprequest"},
	{"browseruse", "browseruse"},
	{tools.SubagentToolName, tools.SubagentToolType},
	{tools.AskUserToolName, tools.AskUserToolType},
	{tools.TodoToolName, tools.TodoToolType},
}

// ensureBuiltinAgent registers the built-in agent when it is missing and
// returns its name. Its model is default_model when configured, otherwise the
// first model in the config (sorted for determinism).
func ensureBuiltinAgent(cfg *config.Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("global configuration not initialized")
	}
	if _, ok := cfg.Agents[builtinAgentName]; ok {
		return builtinAgentName, nil
	}

	model := cfg.DefaultModelName()
	if model == "" {
		return "", fmt.Errorf("没有可用的模型配置：请在 config.yml 的 models 中添加模型")
	}

	if cfg.Tools == nil {
		cfg.Tools = make(map[string]config.Tool)
	}
	defaultTools := make([]string, 0, len(builtinToolDefaults))
	for _, def := range builtinToolDefaults {
		if _, ok := cfg.Tools[def.name]; !ok {
			cfg.Tools[def.name] = config.Tool{Type: def.typ}
		}
		defaultTools = append(defaultTools, def.name)
	}

	if cfg.Agents == nil {
		cfg.Agents = make(map[string]config.Agent)
	}
	cfg.Agents[builtinAgentName] = config.Agent{
		System: builtinAgentSystem,
		Model:  model,
		Tools:  defaultTools,
	}
	return builtinAgentName, nil
}

// resolveDefaultAgent picks the agent for a no-argument start: the last used
// agent if it still exists, otherwise the built-in agent.
func resolveDefaultAgent(cfg *config.Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("global configuration not initialized")
	}
	if last := state.LastAgent(); last != "" {
		if _, ok := cfg.Agents[last]; ok {
			return last, nil
		}
	}
	return ensureBuiltinAgent(cfg)
}

// dropAgentTools removes the named tools from an agent's tool list. Tool
// config entries are left in place, so other agents keep working.
func dropAgentTools(cfg *config.Config, agentName string, drop ...string) {
	if cfg == nil {
		return
	}
	agent, ok := cfg.Agents[agentName]
	if !ok || len(agent.Tools) == 0 {
		return
	}
	skip := make(map[string]struct{}, len(drop))
	for _, name := range drop {
		skip[name] = struct{}{}
	}
	kept := make([]string, 0, len(agent.Tools))
	for _, name := range agent.Tools {
		if _, removed := skip[name]; removed {
			continue
		}
		kept = append(kept, name)
	}
	agent.Tools = kept
	cfg.Agents[agentName] = agent
}

// startAgentSession launches the interactive TUI for an agent and remembers
// it as the last used agent.
func startAgentSession(agentName string) error {
	cfg := config.GetConfig()
	if cfg == nil {
		return fmt.Errorf("global configuration not initialized")
	}
	if _, ok := cfg.Agents[agentName]; !ok {
		return fmt.Errorf("Agent configuration does not exist: %s", agentName)
	}

	agentApp, err := agent.NewAgentApp(agentName)
	if err != nil {
		return fmt.Errorf("failed to create Agent application: %w", err)
	}

	if err := state.SetLastAgent(agentName); err != nil {
		// State persistence is best-effort; the session still works.
		fmt.Printf("Warning: failed to save last agent: %v\n", err)
	}

	fmt.Printf("Starting interactive session with Agent %s...\n", agentName)
	fmt.Println("Use 'q' or 'Ctrl+C' to exit")
	if err := agentApp.Run(); err != nil {
		return fmt.Errorf("failed to run interactive interface: %w", err)
	}
	return nil
}

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Start an interactive agent session",
	Long: `Start an interactive agent session with the specified agent using a TUI interface.

Without arguments it opens the last used agent; if none is available it falls
back to the built-in Eino agent, which has commandline, filesystem,
httprequest, browseruse, subagent, ask_user and todolist enabled.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.GetConfig()

		if cfg.Settings.Langfuse != nil {
			handler, flusher := langfuse.NewLangfuseHandler(cfg.Settings.Langfuse)
			defer flusher()
			callbacks.AppendGlobalHandlers(handler) // Set langfuse as global callback
		}

		// Get parameters
		agentName, _ := cmd.Flags().GetString("agent")
		if agentName != "" {
			return startAgentSession(agentName)
		}

		// No flags: resume the last agent, or start the built-in agent.
		name, err := resolveDefaultAgent(cfg)
		if err != nil {
			return err
		}
		return startAgentSession(name)
	},
}

func init() {
	// Add agent subcommand to root command
	RootCmd.AddCommand(agentCmd)

	// Add parameters for agent subcommand
	agentCmd.Flags().StringP("agent", "a", "", "Specify the Agent name to use")
}

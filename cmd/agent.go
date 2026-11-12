package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cloudwego/eino-ext/callbacks/langfuse"
	"github.com/cloudwego/eino/callbacks"
	"github.com/spf13/cobra"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/state"
	"github.com/tk103331/eino-cli/ui/agent"
)

// builtinAgentName is the implicit agent used when the config does not define
// any agent and no previous agent is available.
const builtinAgentName = "builtin"

// builtinAgentSystem is the system prompt of the built-in basic agent.
const builtinAgentSystem = "You are Eino CLI, a concise and helpful AI assistant. " +
	"Answer clearly and briefly. Use tools only when they are available and relevant."

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

	model := cfg.DefaultModel
	if _, ok := cfg.Models[model]; model == "" || !ok {
		names := make([]string, 0, len(cfg.Models))
		for name := range cfg.Models {
			names = append(names, name)
		}
		if len(names) == 0 {
			return "", fmt.Errorf("没有可用的模型配置：请在 config.yml 的 models 中添加模型")
		}
		sort.Strings(names)
		model = names[0]
	}

	if cfg.Agents == nil {
		cfg.Agents = make(map[string]config.Agent)
	}
	cfg.Agents[builtinAgentName] = config.Agent{
		System: builtinAgentSystem,
		Model:  model,
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
	Short: "Start interactive agent or chat session",
	Long: `Start an interactive agent session with the specified agent using a TUI interface.

Without arguments it opens the last used agent; if none is available it falls
back to the built-in basic agent. Also supports direct chat with models.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.GetConfig()

		if cfg.Settings.Langfuse != nil {
			handler, flusher := langfuse.NewLangfuseHandler(cfg.Settings.Langfuse)
			defer flusher()
			callbacks.AppendGlobalHandlers(handler) // Set langfuse as global callback
		}

		// Get parameters
		agentName, _ := cmd.Flags().GetString("agent")
		chatName, _ := cmd.Flags().GetString("chat")
		modelName, _ := cmd.Flags().GetString("model")
		toolsStr, _ := cmd.Flags().GetString("tools")

		// Prioritize using agent mode
		if agentName != "" {
			return startAgentSession(agentName)
		}

		if chatName != "" || modelName != "" {
			// Use chat mode
			var system string
			var tools []string

			if chatName != "" {
				// Use chats preset
				preset, ok := cfg.Chats[chatName]
				if !ok {
					return fmt.Errorf("chat preset does not exist: %s", chatName)
				}
				modelName = preset.Model
				tools = append(tools, preset.Tools...)
				system = preset.System
			} else {
				// Parse tool list
				if toolsStr != "" {
					tools = strings.Split(toolsStr, ",")
					// Remove whitespace
					for i, tool := range tools {
						tools[i] = strings.TrimSpace(tool)
					}
				}
				if modelName == "" {
					return fmt.Errorf("must specify --model or --chat preset name")
				}
			}

			// Create chat application
			chatApp := agent.NewChatApp(modelName, tools, system)

			// Run chat interface
			fmt.Printf("Starting chat session with Model %s...\n", modelName)
			if err := chatApp.Run(); err != nil {
				return fmt.Errorf("failed to run chat interface: %w", err)
			}
			return nil
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
	agentCmd.Flags().StringP("chat", "c", "", "Specify chat preset name (from config file chats)")
	agentCmd.Flags().StringP("model", "m", "", "Specify the Model to chat with (required when --chat is not specified)")
	agentCmd.Flags().StringP("tools", "t", "", "Specify available tools, separated by commas (optional when --chat is not specified)")
}

package cmd

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino-ext/callbacks/langfuse"
	"github.com/cloudwego/eino/callbacks"
	"github.com/spf13/cobra"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/ui/agent"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Start interactive agent or chat session",
	Long:  `Start an interactive agent session with the specified agent using a TUI interface. Also supports direct chat with models.`,
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
			// Verify if agent name exists
			if _, ok := cfg.Agents[agentName]; !ok {
				return fmt.Errorf("Agent configuration does not exist: %s", agentName)
			}

			// Create Agent interactive application
			agentApp, err := agent.NewAgentApp(agentName)
			if err != nil {
				return fmt.Errorf("failed to create Agent application: %w", err)
			}

			// Run interactive interface
			fmt.Printf("Starting interactive session with Agent %s...\n", agentName)
			fmt.Println("Use 'q' or 'Ctrl+C' to exit")

			if err := agentApp.Run(); err != nil {
				return fmt.Errorf("failed to run interactive interface: %w", err)
			}
		} else if chatName != "" || modelName != "" {
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
		} else {
			return fmt.Errorf("must specify --agent name or --chat/--model to start chat")
		}

		return nil
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

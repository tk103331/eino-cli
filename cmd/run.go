package cmd

import (
	"fmt"

	"github.com/cloudwego/eino-ext/callbacks/langfuse"
	"github.com/cloudwego/eino/callbacks"
	"github.com/spf13/cobra"
	"github.com/tk103331/eino-cli/agent"
	"github.com/tk103331/eino-cli/config"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the agent",
	Long:  `Run the agent with specified parameters`,
	RunE: func(cmd *cobra.Command, args []string) error {

		cfg := config.GetConfig()

		if cfg.Settings.Langfuse != nil {
			handler, flusher := langfuse.NewLangfuseHandler(cfg.Settings.Langfuse)
			defer flusher()
			callbacks.AppendGlobalHandlers(handler) // Set langfuse as global callback
		}

		// Get parameters
		agentName, _ := cmd.Flags().GetString("agent")
		prompt, _ := cmd.Flags().GetString("prompt")

		// Create Agent factory
		factory := agent.NewFactory(cfg)

		// Create Agent
		agent, err := factory.CreateAgent(agentName)
		if err != nil {
			return fmt.Errorf("failed to create Agent: %w", err)
		}

		// Run Agent
		fmt.Printf("Running Agent: %s with prompt: %s\n", agentName, prompt)
		if err := agent.Run(prompt); err != nil {
			return fmt.Errorf("failed to run Agent: %w", err)
		}

		return nil
	},
}

func init() {
	// Add run subcommand to root command
	RootCmd.AddCommand(runCmd)

	// Add parameters for run subcommand
	runCmd.Flags().StringP("agent", "a", "", "Specify the Agent to run")
	runCmd.Flags().StringP("prompt", "p", "", "Specify the prompt for Agent")

	// Set required parameters
	runCmd.MarkFlagRequired("agent")
	runCmd.MarkFlagRequired("prompt")
}

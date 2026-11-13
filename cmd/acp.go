package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino-ext/callbacks/langfuse"
	"github.com/cloudwego/eino/callbacks"
	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/spf13/cobra"
	"github.com/tk103331/eino-cli/acp"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/logger"
	"github.com/tk103331/eino-cli/tools"
)

var acpCmd = &cobra.Command{
	Use:   "acp",
	Short: "Run Eino CLI as an Agent Client Protocol (ACP) agent over stdio",
	Long: `Run Eino CLI as an Agent Client Protocol (ACP) agent.

ACP (https://agentclientprotocol.com) lets editors such as Zed drive Eino CLI
through a standardized JSON-RPC 2.0 protocol over stdio. The configured agent,
including its tools and MCP servers, is exposed to the client as ACP sessions.

Without --agent the built-in Eino agent is served.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.GetConfig()

		if cfg.Settings.Langfuse != nil {
			handler, flusher := langfuse.NewLangfuseHandler(cfg.Settings.Langfuse)
			defer flusher()
			callbacks.AppendGlobalHandlers(handler)
		}

		agentName, _ := cmd.Flags().GetString("agent")

		if agentName == "" {
			// Without --agent, serve the built-in Eino agent so `eino acp`
			// works as soon as a model is configured.
			name, err := ensureBuiltinAgent(cfg)
			if err != nil {
				return err
			}
			agentName = name
			// Interactive-only tools have no ACP counterpart (there is no way
			// to ask the user), so the built-in agent does without them here.
			dropAgentTools(cfg, agentName, tools.AskUserToolName, tools.TodoToolName)
		}

		if _, ok := cfg.Agents[agentName]; !ok {
			names := make([]string, 0, len(cfg.Agents))
			for name := range cfg.Agents {
				names = append(names, name)
			}
			return fmt.Errorf("Agent configuration does not exist: %s (configured agents: %s)",
				agentName, strings.Join(names, ", "))
		}

		// stdout/stderr of this process carries the JSON-RPC protocol and
		// diagnostics respectively; all application logging goes to a file.
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		// The SDK spawns its own read loop and signals disconnect via Done().
		svc := acp.NewService(agentName, cfg)
		conn := acpsdk.NewAgentSideConnection(svc, os.Stdout, os.Stdin)
		svc.SetAgentConnection(conn)
		logger.Debug("ACP", "Serving Agent Client Protocol over stdio")

		<-conn.Done()
		_ = ctx
		return nil
	},
}

func init() {
	RootCmd.AddCommand(acpCmd)
	acpCmd.Flags().StringP("agent", "a", "", "Specify the Agent name to expose over ACP (default: the built-in Eino agent)")
}

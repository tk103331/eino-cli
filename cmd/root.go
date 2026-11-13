package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/mcp"
	"github.com/tk103331/eino-cli/version"
)

var (
	configPath string
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:     "eino-cli",
	Short:   "Eino CLI tool",
	Long:    `A command line interface for Eino`,
	Version: version.Version,
	// Running without a subcommand opens the interactive agent: the last used
	// agent when available, otherwise the built-in Eino agent.
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return cmd.Help()
		}
		cfg := config.GetConfig()
		name, err := resolveDefaultAgent(cfg)
		if err != nil {
			return err
		}
		return startAgentSession(name)
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration file
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("failed to load configuration file: %w", err)
		}

		// Register the built-in agent when possible so it is always selectable
		// (e.g. from the /agent palette). Failures are non-fatal here.
		if _, err := ensureBuiltinAgent(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Note: built-in agent unavailable: %v\n", err)
		}

		// Asynchronously initialize MCP manager (does not block command execution)
		go func() {
			// Use command context for cancellation when command ends
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			if err := mcp.InitializeGlobalManager(ctx, cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to initialize MCP manager asynchronously: %v\n", err)
			}
		}()

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Get user home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	defaultConfigPath := filepath.Join(homeDir, ".eino-cli", "config.yml")

	// Add global parameters
	RootCmd.PersistentFlags().StringVar(&configPath, "config", defaultConfigPath, "Configuration file path")
}

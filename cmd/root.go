package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/mcp"
	"github.com/tk103331/eino-cli/version"
)

var (
	configPath string
	workDir    string
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "eino-cli [directory]",
	Short: "Eino CLI tool",
	Long: `A command line interface for Eino.

Optionally pass a working directory; tools resolve relative paths against it.
The same directory can be given with --cwd/-C.`,
	Version: version.Version,
	Args:    cobra.MaximumNArgs(1),
	// Running without a subcommand opens the interactive agent: the last used
	// agent when available, otherwise the built-in Eino agent.
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.GetConfig()
		name, err := resolveDefaultAgent(cfg)
		if err != nil {
			return err
		}
		return startAgentSession(name)
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Enter the requested working directory first: tools resolve relative
		// paths against it, and skills are loaded after this point. Every
		// command accepts `[directory]` except run, which keeps its own
		// argument surface (--prompt/--agent).
		dir := workDir
		if cmd.Name() != "run" {
			d, err := workDirFromArgs(args, workDir)
			if err != nil {
				return err
			}
			dir = d
		}
		if err := enterWorkDir(dir); err != nil {
			return err
		}

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

// workDirFromArgs merges the --cwd flag with the optional positional
// directory (`eino-cli [directory]`). Specifying both is an error.
func workDirFromArgs(args []string, flagDir string) (string, error) {
	flagDir = strings.TrimSpace(flagDir)
	if len(args) > 1 {
		return "", fmt.Errorf("only one working directory can be given (got %d arguments)", len(args))
	}
	if len(args) == 0 {
		return flagDir, nil
	}
	if flagDir != "" {
		return "", fmt.Errorf("--cwd and a positional directory cannot be used together")
	}
	return args[0], nil
}

// enterWorkDir switches the process to dir when it is set. An empty value keeps
// the current directory; a leading "~" is expanded to the home directory.
func enterWorkDir(dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil
	}
	if dir == "~" || strings.HasPrefix(dir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to resolve home directory: %w", err)
		}
		dir = filepath.Join(home, strings.TrimPrefix(dir, "~"))
	}
	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("failed to change working directory to %s: %w", dir, err)
	}
	return nil
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
	RootCmd.PersistentFlags().StringVarP(&workDir, "cwd", "C", "", "Run in this working directory (default: the current one)")
}

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/mcp"
)

var (
	configPath string
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "eino-cli",
	Short: "Eino CLI tool",
	Long:  `A command line interface for Eino`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// 加载配置文件
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("加载配置文件失败: %w", err)
		}

		// 初始化MCP管理器
		if err := mcp.InitializeGlobalManager(context.Background(), cfg); err != nil {
			return fmt.Errorf("初始化MCP管理器失败: %w", err)
		}

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
	// 获取用户主目录
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	defaultConfigPath := filepath.Join(homeDir, ".eino-cli", "config.yml")
	
	// 添加全局参数
	RootCmd.PersistentFlags().StringVar(&configPath, "config", defaultConfigPath, "配置文件路径")
}
package cmd

import (
	"fmt"

	"github.com/cloudwego/eino-ext/callbacks/langfuse"
	"github.com/cloudwego/eino/callbacks"
	"github.com/spf13/cobra"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/ui/agent"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Start interactive agent session",
	Long:  `Start an interactive agent session with the specified agent using a TUI interface`,
	RunE: func(cmd *cobra.Command, args []string) error {

		cfg := config.GetConfig()

		if cfg.Settings.Langfuse != nil {
			handler, flusher := langfuse.NewLangfuseHandler(cfg.Settings.Langfuse)
			defer flusher()
			callbacks.AppendGlobalHandlers(handler) // 设置langfuse为全局callback
		}

		// 获取参数
		agentName, _ := cmd.Flags().GetString("agent")

		// 验证agent名称是否存在
		if _, ok := cfg.Agents[agentName]; !ok {
			return fmt.Errorf("Agent配置不存在: %s", agentName)
		}

		// 创建Agent交互应用
		agentApp := agent.NewAgentApp(agentName)

		// 运行交互界面
		fmt.Printf("启动与Agent %s 的交互会话...\n", agentName)
		fmt.Println("使用 'q' 或 'Ctrl+C' 退出")

		if err := agentApp.Run(); err != nil {
			return fmt.Errorf("运行交互界面失败: %w", err)
		}

		return nil
	},
}

func init() {
	// 添加 agent 子命令到根命令
	RootCmd.AddCommand(agentCmd)

	// 为 agent 子命令添加参数
	agentCmd.Flags().StringP("agent", "a", "", "指定要使用的Agent名称")

	// 设置必需的参数
	agentCmd.MarkFlagRequired("agent")
}

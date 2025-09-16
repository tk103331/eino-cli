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
			callbacks.AppendGlobalHandlers(handler) // 设置langfuse为全局callback
		}

		// 获取参数
		agentName, _ := cmd.Flags().GetString("agent")
		prompt, _ := cmd.Flags().GetString("prompt")

		// 创建Agent工厂
		factory := agent.NewFactory(cfg)

		// 创建Agent
		agent, err := factory.CreateAgent(agentName)
		if err != nil {
			return fmt.Errorf("创建Agent失败: %w", err)
		}

		// 运行Agent
		fmt.Printf("运行Agent: %s 使用提示词: %s\n", agentName, prompt)
		if err := agent.Run(prompt); err != nil {
			return fmt.Errorf("运行Agent失败: %w", err)
		}

		return nil
	},
}

func init() {
	// 添加 run 子命令到根命令
	RootCmd.AddCommand(runCmd)

	// 为 run 子命令添加参数
	runCmd.Flags().StringP("agent", "a", "", "指定要运行的Agent")
	runCmd.Flags().StringP("prompt", "p", "", "指定Agent的提示词")

	// 设置必需的参数
	runCmd.MarkFlagRequired("agent")
	runCmd.MarkFlagRequired("prompt")
}
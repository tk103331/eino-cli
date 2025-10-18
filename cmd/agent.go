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
			callbacks.AppendGlobalHandlers(handler) // 设置langfuse为全局callback
		}

		// 获取参数
		agentName, _ := cmd.Flags().GetString("agent")
		chatName, _ := cmd.Flags().GetString("chat")
		modelName, _ := cmd.Flags().GetString("model")
		toolsStr, _ := cmd.Flags().GetString("tools")

		// 优先使用agent模式
		if agentName != "" {
			// 验证agent名称是否存在
			if _, ok := cfg.Agents[agentName]; !ok {
				return fmt.Errorf("Agent配置不存在: %s", agentName)
			}

			// 创建Agent交互应用
			agentApp, err := agent.NewAgentApp(agentName)
			if err != nil {
				return fmt.Errorf("创建Agent应用失败: %w", err)
			}

			// 运行交互界面
			fmt.Printf("启动与Agent %s 的交互会话...\n", agentName)
			fmt.Println("使用 'q' 或 'Ctrl+C' 退出")

			if err := agentApp.Run(); err != nil {
				return fmt.Errorf("运行交互界面失败: %w", err)
			}
		} else if chatName != "" || modelName != "" {
			// 使用chat模式
			var system string
			var tools []string

			if chatName != "" {
				// 使用 chats 预设
				preset, ok := cfg.Chats[chatName]
				if !ok {
					return fmt.Errorf("chat 预设不存在: %s", chatName)
				}
				modelName = preset.Model
				tools = append(tools, preset.Tools...)
				system = preset.System
			} else {
				// 解析工具列表
				if toolsStr != "" {
					tools = strings.Split(toolsStr, ",")
					// 去除空格
					for i, tool := range tools {
						tools[i] = strings.TrimSpace(tool)
					}
				}
				if modelName == "" {
					return fmt.Errorf("必须指定 --model 或者 --chat 预设名称")
				}
			}

			// 创建聊天应用
			chatApp := agent.NewChatApp(modelName, tools, system)

			// 运行聊天界面
			fmt.Printf("启动与Model %s 的聊天会话...\n", modelName)
			if err := chatApp.Run(); err != nil {
				return fmt.Errorf("运行聊天界面失败: %w", err)
			}
		} else {
			return fmt.Errorf("必须指定 --agent 名称或者 --chat/--model 进行聊天")
		}

		return nil
	},
}

func init() {
	// 添加 agent 子命令到根命令
	RootCmd.AddCommand(agentCmd)

	// 为 agent 子命令添加参数
	agentCmd.Flags().StringP("agent", "a", "", "指定要使用的Agent名称")
	agentCmd.Flags().StringP("chat", "c", "", "指定 chat 预设名称（来自配置文件 chats）")
	agentCmd.Flags().StringP("model", "m", "", "指定要聊天的Model（未指定 --chat 时必填）")
	agentCmd.Flags().StringP("tools", "t", "", "指定可以使用的工具，多个工具用逗号分隔（未指定 --chat 时可选）")
}

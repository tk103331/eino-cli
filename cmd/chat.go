package cmd

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino-ext/callbacks/langfuse"
	"github.com/cloudwego/eino/callbacks"
	"github.com/spf13/cobra"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/ui/chat"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start interactive chat with model",
	Long:  `Start an interactive chat session with the specified model using a TUI interface`,
	RunE: func(cmd *cobra.Command, args []string) error {

		cfg := config.GetConfig()

		if cfg.Settings.Langfuse != nil {
			handler, flusher := langfuse.NewLangfuseHandler(cfg.Settings.Langfuse)
			defer flusher()
			callbacks.AppendGlobalHandlers(handler) // 设置langfuse为全局callback
		}

		// 获取参数
		presetName, _ := cmd.Flags().GetString("chat")
		modelName, _ := cmd.Flags().GetString("model")
		toolsStr, _ := cmd.Flags().GetString("tools")

		var system string
		var tools []string

		if presetName != "" {
			// 使用 chats 预设
			preset, ok := cfg.Chats[presetName]
			if !ok {
				return fmt.Errorf("chat 预设不存在: %s", presetName)
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
		chatApp := chat.NewChatApp(modelName, tools, system)

		// 运行聊天界面
		fmt.Printf("启动与Model %s 的聊天会话...\n", modelName)
		if err := chatApp.Run(); err != nil {
			return fmt.Errorf("运行聊天界面失败: %w", err)
		}

		return nil
	},
}

func init() {
	// 添加 chat 子命令到根命令
	RootCmd.AddCommand(chatCmd)

	// 为 chat 子命令添加参数
	chatCmd.Flags().StringP("chat", "c", "", "指定 chat 预设名称（来自配置文件 chats）")
	chatCmd.Flags().StringP("model", "m", "", "指定要聊天的Model（未指定 --chat 时必填）")
	chatCmd.Flags().StringP("tools", "t", "", "指定可以使用的工具，多个工具用逗号分隔（未指定 --chat 时可选）")

	// 不再强制 --model 必填，由运行时校验根据 --chat 决定
	// chatCmd.MarkFlagRequired("model")
}

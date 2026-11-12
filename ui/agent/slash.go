package agent

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tk103331/eino-cli/agent"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/state"
)

// sortedKeys returns the sorted keys of a string-keyed map.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// themeModeName renders the active background mode for the status line.
func themeModeName(dark bool) string {
	if dark {
		return "dark"
	}
	return "light"
}

// ---- AgentApp slash commands ----

// effectiveAgentConfig applies runtime overrides (model / skill) on top of the
// configured agent.
func (app *AgentApp) effectiveAgentConfig() (config.Agent, bool) {
	cfg := config.GetConfig()
	if cfg == nil {
		return config.Agent{}, false
	}
	ac, ok := cfg.Agents[app.agentName]
	if !ok {
		return config.Agent{}, false
	}
	if app.modelOverride != "" {
		ac.Model = app.modelOverride
	}
	if app.skill != "" {
		if sk, ok := cfg.Skills[app.skill]; ok {
			if s := strings.TrimSpace(sk.System); s != "" {
				if strings.TrimSpace(ac.System) != "" {
					ac.System = s + "\n\n" + ac.System
				} else {
					ac.System = s
				}
			}
			if len(sk.Tools) > 0 {
				ac.Tools = append(append([]string{}, ac.Tools...), sk.Tools...)
			}
		}
	}
	return ac, true
}

// rebuildAgent recreates the underlying Eino agent with current overrides.
func (app *AgentApp) rebuildAgent() error {
	ac, ok := app.effectiveAgentConfig()
	if !ok {
		return fmt.Errorf("agent configuration does not exist: %s", app.agentName)
	}
	ra := agent.NewReactAgent(app.agentName, &ac)
	if err := ra.Init(); err != nil {
		return err
	}
	app.agent = ra
	return nil
}

func (app *AgentApp) currentModelName() string {
	if app.modelOverride != "" {
		return app.modelOverride
	}
	if cfg := config.GetConfig(); cfg != nil {
		if ac, ok := cfg.Agents[app.agentName]; ok {
			return ac.Model
		}
	}
	return ""
}

func (app *AgentApp) skillName() string {
	if app.skill == "" {
		return "off"
	}
	return app.skill
}

// slashCommands describes the commands available in agent mode.
func (app *AgentApp) slashCommands() []SlashCommand {
	cfg := config.GetConfig()
	if cfg == nil {
		return nil
	}
	skillValues := append([]string{"off"}, sortedKeys(cfg.Skills)...)
	return []SlashCommand{
		{Name: "agent", Args: "<name>", Description: fmt.Sprintf("切换 agent（当前: %s）", app.agentName), Values: sortedKeys(cfg.Agents)},
		{Name: "model", Args: "<name>", Description: fmt.Sprintf("切换模型（当前: %s）", app.currentModelName()), Values: sortedKeys(cfg.Models)},
		{Name: "skill", Args: "<name>", Description: fmt.Sprintf("启用 skill（当前: %s）", app.skillName()), Values: skillValues},
		{Name: "theme", Args: "<name> [light|dark]", Description: fmt.Sprintf("切换颜色主题（当前: %s/%s）", app.model.ThemeName(), themeModeName(app.model.DarkMode())), Values: themeNames()},
		{Name: "tools", Description: "列出当前可用工具"},
		{Name: "status", Description: "显示当前会话状态"},
	}
}

// executeSlash handles agent-mode commands.
func (app *AgentApp) executeSlash(input string) CommandResult {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return CommandResult{Message: "用法：/agent、/model、/skill、/tools、/status", Error: true}
	}
	name := strings.ToLower(fields[0])
	arg := ""
	if len(fields) > 1 {
		arg = fields[1]
	}
	cfg := config.GetConfig()
	if cfg == nil {
		return CommandResult{Message: "配置未初始化", Error: true}
	}

	switch name {
	case "agent":
		if arg == "" {
			return CommandResult{Message: "用法：/agent <name>\n可用 agent：" + strings.Join(sortedKeys(cfg.Agents), ", "), Error: true}
		}
		if _, ok := cfg.Agents[arg]; !ok {
			return CommandResult{Message: fmt.Sprintf("agent 不存在: %s\n可用 agent：%s", arg, strings.Join(sortedKeys(cfg.Agents), ", ")), Error: true}
		}
		app.agentName = arg
		app.modelOverride = "" // reset model override when switching agent
		if err := app.rebuildAgent(); err != nil {
			return CommandResult{Message: fmt.Sprintf("切换 agent 失败: %v", err), Error: true}
		}
		_ = state.SetLastAgent(arg) // remember for the next no-argument start
		app.registerCommands()
		return CommandResult{Message: "已切换到 agent: " + arg, Subtitle: arg}

	case "model":
		if arg == "" {
			return CommandResult{Message: "用法：/model <name>\n可用模型：" + strings.Join(sortedKeys(cfg.Models), ", "), Error: true}
		}
		if _, ok := cfg.Models[arg]; !ok {
			return CommandResult{Message: fmt.Sprintf("模型不存在: %s\n可用模型：%s", arg, strings.Join(sortedKeys(cfg.Models), ", ")), Error: true}
		}
		app.modelOverride = arg
		if err := app.rebuildAgent(); err != nil {
			return CommandResult{Message: fmt.Sprintf("切换模型失败: %v", err), Error: true}
		}
		app.registerCommands()
		return CommandResult{Message: "已切换模型: " + arg}

	case "skill":
		if arg == "" {
			return CommandResult{Message: "用法：/skill <name>（用 /skill off 关闭）\n可用 skill：" + strings.Join(append([]string{"off"}, sortedKeys(cfg.Skills)...), ", ") +
				"\n目录：" + strings.Join(config.SkillDirectories(), ", "), Error: true}
		}
		if arg == "off" || arg == "none" {
			app.skill = ""
			if err := app.rebuildAgent(); err != nil {
				return CommandResult{Message: fmt.Sprintf("关闭 skill 失败: %v", err), Error: true}
			}
			app.registerCommands()
			return CommandResult{Message: "已关闭 skill"}
		}
		if _, ok := cfg.Skills[arg]; !ok {
			return CommandResult{Message: fmt.Sprintf("skill 不存在: %s\n可用 skill：%s", arg, strings.Join(sortedKeys(cfg.Skills), ", ")), Error: true}
		}
		app.skill = arg
		if err := app.rebuildAgent(); err != nil {
			return CommandResult{Message: fmt.Sprintf("启用 skill 失败: %v", err), Error: true}
		}
		app.registerCommands()
		return CommandResult{Message: "已启用 skill: " + arg}

	case "theme":
		if arg == "" {
			return CommandResult{Message: "用法：/theme <name> [light|dark]\n可用主题：" + strings.Join(themeNames(), ", "), Error: true}
		}
		mode := ""
		if len(fields) > 2 {
			mode = fields[2]
		}
		if err := app.model.ApplyTheme(arg, mode); err != nil {
			return CommandResult{Message: err.Error(), Error: true}
		}
		app.registerCommands()
		return CommandResult{Message: fmt.Sprintf("已切换主题: %s（%s）", app.model.ThemeName(), themeModeName(app.model.DarkMode()))}

	case "tools":
		ac, ok := app.effectiveAgentConfig()
		if !ok {
			return CommandResult{Message: "无法读取当前 agent 配置", Error: true}
		}
		var b strings.Builder
		b.WriteString("当前工具：")
		if len(ac.Tools) == 0 {
			b.WriteString("（无）")
		} else {
			for _, t := range ac.Tools {
				b.WriteString("\n  · " + t)
			}
		}
		if len(ac.MCPServers) > 0 {
			b.WriteString("\nMCP 服务：")
			for _, s := range ac.MCPServers {
				b.WriteString("\n  · " + s)
			}
		}
		return CommandResult{Message: b.String()}

	case "status":
		return CommandResult{Message: fmt.Sprintf("当前状态：\n  agent: %s\n  model: %s\n  skill: %s\n  theme: %s（%s）",
			app.agentName, app.currentModelName(), app.skillName(),
			app.model.ThemeName(), themeModeName(app.model.DarkMode()))}
	}

	return CommandResult{Message: "未知命令: /" + name + "（输入 /help 查看可用命令）", Error: true}
}

// registerCommands wires the command palette into the view model.
func (app *AgentApp) registerCommands() {
	if app.model != nil {
		app.model.SetSlashCommands(app.slashCommands(), app.executeSlash)
	}
}

// ---- ChatApp slash commands ----

// effectiveSystem composes the system prompt with the active skill.
func (app *ChatApp) effectiveSystem() string {
	cfg := config.GetConfig()
	if app.skill != "" && cfg != nil {
		if sk, ok := cfg.Skills[app.skill]; ok {
			if s := strings.TrimSpace(sk.System); s != "" {
				if strings.TrimSpace(app.system) != "" {
					return s + "\n\n" + app.system
				}
				return s
			}
		}
	}
	return app.system
}

// effectiveTools merges base tools with tools added by the active skill.
func (app *ChatApp) effectiveTools() []string {
	tools := append([]string{}, app.tools...)
	cfg := config.GetConfig()
	if app.skill != "" && cfg != nil {
		if sk, ok := cfg.Skills[app.skill]; ok {
			tools = append(tools, sk.Tools...)
		}
	}
	return tools
}

func (app *ChatApp) skillName() string {
	if app.skill == "" {
		return "off"
	}
	return app.skill
}

// slashCommands describes the commands available in chat mode.
func (app *ChatApp) slashCommands() []SlashCommand {
	cfg := config.GetConfig()
	if cfg == nil {
		return nil
	}
	skillValues := append([]string{"off"}, sortedKeys(cfg.Skills)...)
	return []SlashCommand{
		{Name: "model", Args: "<name>", Description: fmt.Sprintf("切换模型（当前: %s）", app.modelName), Values: sortedKeys(cfg.Models)},
		{Name: "skill", Args: "<name>", Description: fmt.Sprintf("启用 skill（当前: %s）", app.skillName()), Values: skillValues},
		{Name: "theme", Args: "<name> [light|dark]", Description: fmt.Sprintf("切换颜色主题（当前: %s/%s）", app.model.ThemeName(), themeModeName(app.model.DarkMode())), Values: themeNames()},
		{Name: "tools", Description: "列出当前可用工具"},
		{Name: "status", Description: "显示当前会话状态"},
	}
}

// executeSlash handles chat-mode commands.
func (app *ChatApp) executeSlash(input string) CommandResult {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return CommandResult{Message: "用法：/model、/skill、/tools、/status", Error: true}
	}
	name := strings.ToLower(fields[0])
	arg := ""
	if len(fields) > 1 {
		arg = fields[1]
	}
	cfg := config.GetConfig()
	if cfg == nil {
		return CommandResult{Message: "配置未初始化", Error: true}
	}

	switch name {
	case "agent":
		return CommandResult{Message: "chat 模式不支持切换 agent，请使用 --agent 重新启动", Error: true}

	case "model":
		if arg == "" {
			return CommandResult{Message: "用法：/model <name>\n可用模型：" + strings.Join(sortedKeys(cfg.Models), ", "), Error: true}
		}
		if _, ok := cfg.Models[arg]; !ok {
			return CommandResult{Message: fmt.Sprintf("模型不存在: %s\n可用模型：%s", arg, strings.Join(sortedKeys(cfg.Models), ", ")), Error: true}
		}
		app.modelName = arg
		app.chatModel = nil
		app.reactAgent = nil
		app.registerCommands()
		return CommandResult{Message: "已切换模型: " + arg, Subtitle: arg}

	case "skill":
		if arg == "" {
			return CommandResult{Message: "用法：/skill <name>（用 /skill off 关闭）\n可用 skill：" + strings.Join(append([]string{"off"}, sortedKeys(cfg.Skills)...), ", ") +
				"\n目录：" + strings.Join(config.SkillDirectories(), ", "), Error: true}
		}
		if arg == "off" || arg == "none" {
			app.skill = ""
		} else {
			if _, ok := cfg.Skills[arg]; !ok {
				return CommandResult{Message: fmt.Sprintf("skill 不存在: %s\n可用 skill：%s", arg, strings.Join(sortedKeys(cfg.Skills), ", ")), Error: true}
			}
			app.skill = arg
		}
		app.chatModel = nil
		app.reactAgent = nil
		app.registerCommands()
		if app.skill == "" {
			return CommandResult{Message: "已关闭 skill"}
		}
		return CommandResult{Message: "已启用 skill: " + app.skill}

	case "theme":
		if arg == "" {
			return CommandResult{Message: "用法：/theme <name> [light|dark]\n可用主题：" + strings.Join(themeNames(), ", "), Error: true}
		}
		mode := ""
		if len(fields) > 2 {
			mode = fields[2]
		}
		if err := app.model.ApplyTheme(arg, mode); err != nil {
			return CommandResult{Message: err.Error(), Error: true}
		}
		app.registerCommands()
		return CommandResult{Message: fmt.Sprintf("已切换主题: %s（%s）", app.model.ThemeName(), themeModeName(app.model.DarkMode()))}

	case "tools":
		tools := app.effectiveTools()
		if len(tools) == 0 {
			return CommandResult{Message: "当前工具：（无）"}
		}
		return CommandResult{Message: "当前工具：\n  · " + strings.Join(tools, "\n  · ")}

	case "status":
		return CommandResult{Message: fmt.Sprintf("当前状态：\n  model: %s\n  skill: %s\n  theme: %s（%s）\n  工具数: %d",
			app.modelName, app.skillName(),
			app.model.ThemeName(), themeModeName(app.model.DarkMode()), len(app.effectiveTools()))}
	}

	return CommandResult{Message: "未知命令: /" + name + "（输入 /help 查看可用命令）", Error: true}
}

// registerCommands wires the command palette into the view model.
func (app *ChatApp) registerCommands() {
	if app.model != nil {
		app.model.SetSlashCommands(app.slashCommands(), app.executeSlash)
	}
}

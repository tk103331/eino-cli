package agent

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tk103331/eino-cli/agent"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/logger"
	"github.com/tk103331/eino-cli/session"
	"github.com/tk103331/eino-cli/state"
	"github.com/tk103331/eino-cli/tools"
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
	} else if ac.Model == "" {
		// An agent without its own model follows the default model.
		ac.Model = cfg.DefaultModelName()
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
	// Interactive sessions always expose the ask/task tools; the TUI
	// implements their host side.
	ac.Tools = appendUnique(ac.Tools, tools.AskUserToolName, tools.TodoToolName)
	return ac, true
}

// rememberedModel returns the model remembered for an agent (see /model).
// It only applies to agents that follow the default model: an agent with its
// own model pinned in the config keeps it, so the config always wins there.
func rememberedModel(cfg *config.Config, agentName string) string {
	if cfg == nil || !cfg.AgentUsesDefaultModel(agentName) {
		return ""
	}
	model := state.RememberedModel(agentName)
	if model == "" {
		return ""
	}
	if _, ok := cfg.Models[model]; !ok {
		return ""
	}
	if ac := cfg.Agents[agentName]; model == ac.Model {
		return ""
	}
	return model
}

// rememberModel persists a runtime model choice for agents that follow the
// default model. Choosing the configured model again clears the override.
func rememberModel(cfg *config.Config, agentName, model string) {
	if cfg == nil || !cfg.AgentUsesDefaultModel(agentName) {
		return
	}
	if ac := cfg.Agents[agentName]; model == ac.Model {
		model = ""
	}
	if err := state.SetRememberedModel(agentName, model); err != nil {
		logger.Warn("UI-AGENT", fmt.Sprintf("failed to remember model for %s: %v", agentName, err))
	}
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
			if ac.Model != "" {
				return ac.Model
			}
			return cfg.DefaultModelName()
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
		{Name: "resume", Args: "<编号|id>", Description: "恢复历史会话（不带参数列出会话）", Values: recentSessionIDs(session.KindAgent, 10)},
	}
}

// executeSlash handles agent-mode commands.
func (app *AgentApp) executeSlash(input string) CommandResult {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return CommandResult{Message: "用法：/agent、/model、/skill、/resume、/tools、/status", Error: true}
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
		app.modelOverride = rememberedModel(cfg, arg) // restore that agent's remembered model
		if err := app.rebuildAgent(); err != nil {
			return CommandResult{Message: fmt.Sprintf("切换 agent 失败: %v", err), Error: true}
		}
		_ = state.SetLastAgent(arg) // remember for the next no-argument start
		app.syncSessionConfig()
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
		rememberModel(cfg, app.agentName, arg)
		app.syncSessionConfig()
		app.registerCommands()
		msg := "已切换模型: " + arg
		if cfg.AgentUsesDefaultModel(app.agentName) {
			msg += "（已记住，下次启动沿用）"
		}
		return CommandResult{Message: msg}

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
			app.syncSessionConfig()
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
		app.syncSessionConfig()
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
		return CommandResult{Message: fmt.Sprintf("当前状态：\n  agent: %s\n  model: %s\n  skill: %s\n  审批: %s\n  theme: %s（%s）\n  会话: %s",
			app.agentName, app.currentModelName(), app.skillName(), approvalModeLabel(),
			app.model.ThemeName(), themeModeName(app.model.DarkMode()), app.sessionSummary())}

	case "resume":
		return app.resumeSession(arg)
	}

	return CommandResult{Message: "未知命令: /" + name + "（输入 /help 查看可用命令）", Error: true}
}

// sessionSummary renders the active session for /status.
func (app *AgentApp) sessionSummary() string {
	app.sessionMu.Lock()
	defer app.sessionMu.Unlock()
	if app.sess == nil {
		return "（无）"
	}
	return fmt.Sprintf("%s（%d 条，创建于 %s）", app.sess.ID, app.sess.TurnCount(), sessionClock(app.sess.Created))
}

// resumeSession lists saved agent sessions or switches to one of them.
func (app *AgentApp) resumeSession(arg string) CommandResult {
	cfg := config.GetConfig()
	if cfg == nil {
		return CommandResult{Message: "配置未初始化", Error: true}
	}
	if arg == "" {
		return CommandResult{Message: sessionListText(session.KindAgent, 10)}
	}

	s, err := resolveSession(session.KindAgent, arg)
	if err != nil {
		return CommandResult{Message: err.Error(), Error: true}
	}
	// Resumed conversations start with a clean approval state.
	app.approvals.reset()

	// Restore the agent (if it still exists), then the model/skill overrides.
	if s.Agent != "" {
		if _, ok := cfg.Agents[s.Agent]; ok {
			app.agentName = s.Agent
		}
	}
	ac, ok := cfg.Agents[app.agentName]
	if !ok {
		return CommandResult{Message: fmt.Sprintf("agent 不存在: %s", app.agentName), Error: true}
	}
	if ac.Model == "" {
		ac.Model = cfg.DefaultModelName()
	}
	// The remembered model is the baseline; a model recorded in the session
	// itself takes precedence for the resumed conversation.
	app.modelOverride = rememberedModel(cfg, app.agentName)
	if s.Model != "" && s.Model != ac.Model {
		if _, ok := cfg.Models[s.Model]; ok {
			app.modelOverride = s.Model
		}
	}
	app.skill = ""
	if s.Skill != "" {
		if _, ok := cfg.Skills[s.Skill]; ok {
			app.skill = s.Skill
		}
	}
	if err := app.rebuildAgent(); err != nil {
		return CommandResult{Message: fmt.Sprintf("恢复会话失败: %v", err), Error: true}
	}

	app.sessionMu.Lock()
	app.sess = s
	app.sessionMu.Unlock()
	_ = state.SetLastAgent(app.agentName)

	app.model.SetTranscript(sessionTranscript(s))
	app.model.SetTodos(viewTodos(s.Todos()))
	app.model.SetSubtitle(app.agentName)
	app.registerCommands()

	return CommandResult{Message: fmt.Sprintf("已恢复会话 %s（agent: %s，%d 条消息）", s.ID, app.agentName, s.TurnCount())}
}

// registerCommands wires the command palette into the view model.
func (app *AgentApp) registerCommands() {
	if app.model != nil {
		app.model.SetSlashCommands(app.slashCommands(), app.executeSlash)
	}
}

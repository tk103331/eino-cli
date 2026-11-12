[English](README.md) | 中文

Eino CLI 是一个基于 [CloudWeGo Eino](https://github.com/cloudwego/eino) 框架的智能 AI Agent 命令行工具。它提供了强大的 Agent 系统，支持多种工具集成和多个 AI 模型提供商，让您能够轻松构建和运行自定义的 AI Agent。

## 功能特点

- **智能 Agent 系统**：基于 ReAct 模式的 AI Agent，支持工具调用和推理
- **交互式聊天界面**：基于 TUI 的聊天界面，可与 AI 模型和agents进行实时对话
- **ACP Agent 模式**：可作为 [Agent Client Protocol](https://agentclientprotocol.com) Agent 运行，供 Zed 等编辑器接入
- **丰富的工具生态**：内置多种工具，包括搜索、浏览器、命令行、HTTP 请求等
- **多模型提供商支持**：支持 OpenAI、Claude、Gemini、Qwen、DeepSeek、Ollama、百度千帆、字节跳动豆包等
- **灵活的配置系统**：通过 YAML 配置文件管理 Agent、工具、模型和聊天预设
- **自定义工具支持**：支持自定义 HTTP 和命令行工具
- **MCP 服务器集成**：支持 Model Context Protocol (MCP) 服务器，包括 SSE 和 STDIO 传输
- **Langfuse 集成**：内置 Langfuse 可观测性支持，用于监控和追踪

## 安装

### 安装方式

**Go install**（需要 Go ≥ 1.24）：

```bash
# 安装短命令 eino（推荐）
go install github.com/tk103331/eino-cli/cmd/eino@latest
# 若提示 sumdb ... 404（新版本校验库索引延迟）：
# GONOSUMDB=github.com/tk103331/eino-cli go install github.com/tk103331/eino-cli/cmd/eino@latest

# 或安装 eino-cli（两者是同一个程序）
go install github.com/tk103331/eino-cli@latest
```

**npm**：

```bash
npm install -g eino-cli    # 同时提供 eino 与 eino-cli 两个命令
eino
```

**Homebrew**：

```bash
brew install tk103331/tap/eino-cli   # 同时提供 eino 与 eino-cli
```

**发布（维护者）**：

```bash
git tag v0.1.0 && git push --tags
```

GitHub Actions（`.github/workflows/release.yml`）会自动完成：
1. goreleaser 构建 darwin/linux × amd64/arm64 的 `eino` / `eino-cli` 二进制并创建 Release
2. 配置了 `NPM_TOKEN` secret 时发布 npm 包（版本号取自 tag）
3. 配置了 `TAP_GITHUB_TOKEN` secret 时更新 `tk103331/homebrew-tap` 的 formula

首次发布前需要准备：
- npm Automation token → 仓库 secret `NPM_TOKEN`
- 创建 `tk103331/homebrew-tap` 仓库，PAT（repo 权限）→ secret `TAP_GITHUB_TOKEN`

**从源码构建**：

```bash
git clone https://github.com/tk103331/eino-cli.git
cd eino-cli
go install ./cmd/eino    # 生成 eino
go install .             # 生成 eino-cli
```

## 使用方法

### 1. 配置文件

首先，复制配置文件模板并根据需要进行修改：

```bash
cp config.yml.example config.yml
```

配置文件包含以下主要部分：
- `providers`: AI 模型提供商配置（API 密钥、基础 URL 等）
- `models`: 模型配置（温度、最大 token 数等）
- `agents`: Agent 配置（系统提示、使用的模型和工具）
- `chats`: 聊天预设配置，用于交互式聊天模式
- `tools`: 工具配置（自定义工具的参数和配置）
- `mcp_servers`: MCP 服务器配置
- `settings`: 全局设置，包括 Langfuse 配置

### 2. 交互式Agent/聊天模式

TUI 内支持斜杠命令（输入 `/` 会显示命令面板与参数候选）：

| 命令 | 说明 |
|---|---|
| `/help` | 显示所有命令 |
| `/clear` | 清空当前对话 |
| `/agent <name>` | 切换 agent（agent 模式） |
| `/model <name>` | 切换模型（无需重启） |
| `/skill <name>` | 启用配置中的 skill；`/skill off` 关闭 |
| `/theme <name> [light\|dark]` | 切换颜色主题（每套主题均含明暗两套配色） |

TUI 默认**不捕获鼠标**，可直接用终端鼠标选择并复制文本；同时自动开启终端「备用屏幕滚轮转发」（DECSET 1007），在支持的终端（iTerm2、Kitty、WezTerm、xterm 等）中滚轮可直接滚动对话，无需按键。若终端不支持 1007，可用 ↑↓/PgUp/PgDn，或按 `Ctrl+Y` 开启鼠标捕获（再按一次关闭）。
| `/tools` | 列出当前可用工具 |
| `/status` | 显示当前 agent/model/skill |

`skills` 是 `config.yml` 中的可切换能力预设（扩展 system prompt、可追加工具）：

```yaml
skills:
  translator:
    description: "翻译助手：中英互译"
    system: "你是一名专业翻译，只输出译文。"
  coder:
    description: "编码助手"
    system: "优先给出可运行的代码示例。"
    tools: [shell]   # 可选：追加工具
```

也支持**目录形式**的 skill（无需写进 config.yml），按优先级扫描两个目录：

- `~/.eino-cli/skills/*.md`（优先）
- `~/.agents/skills/*.md`

同名 skill 以第一个目录为准；目录中的 skill 优先于 config.yml 里的定义。

每个文件是一个 markdown 文档，可用 front matter 声明描述与追加工具：

```markdown
---
description: 代码审查
tools: [shell]
---
你是一位严格的代码审查员，先指出正确性问题……
```



启动与agent或模型的交互式会话：

```bash
# 无参数：恢复上次使用的 agent；没有历史时使用内置 basic agent
eino-cli agent
eino-cli          # 不带子命令等价于 eino-cli agent

# 内置 agent 使用配置中的 default_model（未设置时取排序第一个模型），
# 上次使用的 agent 记录在 ~/.eino-cli/last_agent
```



```bash
# 启动交互式agent会话
eino-cli agent --agent search_agent

# 与指定模型聊天
eino-cli agent --model gpt4

# 使用工具聊天
eino-cli agent --model gpt4 --tools duckduckgo_search,wikipedia_search

# 使用聊天预设
eino-cli agent --chat search_chat
```

参数说明：
- `--agent, -a`: 指定要运行的agent名称（可选）
- `--chat, -c`: 指定配置文件中的聊天预设名称（可选）
- `--model, -m`: 指定要聊天的模型（未使用--agent或--chat时必需）
- `--tools, -t`: 指定可用工具，多个工具用逗号分隔（直接使用--model时可选）

### 3. 运行 Agent

使用 `run` 命令运行指定的 Agent：

```bash
eino-cli run --agent test_agent --prompt "你好，请帮我搜索一下今天的天气"
```

参数说明：
- `--agent, -a`: 指定要运行的 Agent 名称（必需）
- `--prompt, -p`: 指定 Agent 的输入提示（必需）
- `--config`: 指定配置文件路径（可选，默认为 ~/.eino-cli/config.yml）

### 4. ACP Agent 模式（供 Zed 等编辑器接入）

将 Eino CLI 作为 [Agent Client Protocol (ACP)](https://agentclientprotocol.com) Agent 运行。
它通过 stdio 使用 JSON-RPC 2.0 通信，任何 ACP 客户端（如 Zed 编辑器）都可以创建会话、发送提示，并接收流式响应和工具调用事件。

```bash
# 以 ACP 方式暴露某个 agent（不指定时若只配置了一个 agent 则默认使用它）
eino-cli acp --agent search_agent
```

Zed 配置示例（`settings.json`）：

```json
{
  "agent": {
    "profiles": {
      "eino": {
        "provider": "acp",
        "name": "Eino CLI",
        "command": "eino-cli",
        "args": ["acp", "--agent", "search_agent"]
      }
    }
  }
}
```

支持的 ACP 能力：

- `initialize`、`session/new`、`session/prompt`、`session/cancel`（协议 v1）
- 流式 `session/update` 通知：`agent_message_chunk`、`tool_call`、`tool_call_update`
- 停止原因：`end_turn` 与 `cancelled`
- `config.yml` 中配置的工具（内置工具与 MCP 服务）会以 ACP 工具调用形式暴露，带状态（`pending`/`in_progress`/`completed`/`failed`）与类型（`search`、`execute`、`fetch` 等）

注意：日志写入 `~/.eino-cli/eino-cli.log`——`acp` 命令的 stdout 专用于协议通信。

### 5. 配置示例

以下是一个完整的配置示例：

```yaml
# 模型提供商配置
providers:
  openai:
    type: openai
    base_url: https://api.openai.com/v1
    api_key: sk-xxxxx
  claude:
    type: claude
    api_key: sk-ant-xxxxx

# 模型配置
models:
  gpt4:
    provider: openai
    model: gpt-4
    max_tokens: 4096
    temperature: 0.7
  claude_sonnet:
    provider: claude
    model: claude-3-5-sonnet-20241022
    max_tokens: 4096
    temperature: 0.7

# 聊天预设配置
chats:
  search_chat:
    model: gpt4
    system: "你是一个有用的搜索助手。使用搜索工具为用户查找信息。"
    tools:
      - duckduckgo_search
      - wikipedia_search
  coding_chat:
    model: claude_sonnet
    system: "你是一个专业的程序员，帮助用户完成编程任务。"
    tools:
      - commandline
      - http_request

# MCP 服务器配置
mcp_servers:
  # SSE 类型的 MCP 服务器
  sse_server:
    type: mcp
    config:
      url: "http://localhost:3000/mcp"  # MCP 服务器 URL
      headers:
        "Content-Type": "application/json"
        "Authorization": "Bearer your-token"  # 可选的认证头
  # STDIO 类型的 MCP 服务器
  stdio_server:
    type: stdio
    config:
      cmd: "python"                    # 要执行的命令
      args:
        - "-m"
        - "your_mcp_server"             # MCP 服务器模块
      env:
        "PYTHONPATH": "/path/to/server" # 环境变量
        "API_KEY": "your-api-key"

# Agent 配置
agents:
  search_agent:
    system: "你是一个搜索助手，可以帮助用户搜索信息"
    model: gpt4
    tools:
      - duckduckgo_search
      - wikipedia_search
    mcp_servers:                        # Agent 可以使用的 MCP 服务器
      - sse_server
  # 多功能助手示例（包含自定义工具和MCP服务器）
  multi_agent:
    system: "你是一个多功能助手，可以搜索信息、查询天气、获取系统信息等"
    model: gpt4
    tools:
      - duckduckgo_search
      - weather_api
      - system_info
    mcp_servers:
      - sse_server
      - stdio_server

# 工具配置
tools:
  duckduckgo_search:
    type: duckduckgo
    config:
      max_results: 10        # 最大搜索结果数量，默认10
      region: "wt"           # 搜索区域：wt(全球)、cn(中国)、us(美国)、uk(英国)
      safe_search: "off"     # 安全搜索：off(关闭)、moderate(中等)、strict(严格)
      timeout: 10            # 超时时间（秒），默认10秒
  # 自定义 HTTP 工具示例
  weather_api:
    type: customhttp
    description: "获取天气信息的 API 工具"
    config:
      url: "https://api.openweathermap.org/data/2.5/weather?q={{city}}"
      method: "GET"
      headers:
        "Content-Type": "application/json"
    params:
      - name: "city"
        type: "string"
        description: "城市名称"
        required: true
  # 自定义命令行工具示例
  system_info:
    type: customexec
    description: "获取系统信息"
    config:
      cmd: "uname -a && df -h"
      dir: "/tmp"
      timeout: 30
    params: []

# 全局设置
settings:
  langfuse:
    host: https://cloud.langfuse.com
    public_key: pk-xxx
    secret_key: sk-xxx
```

## 支持的工具

Eino CLI 内置了多种工具，可以在 Agent 中使用：

### 搜索工具
- **DuckDuckGo**: 网页搜索
- **Google Search**: Google 搜索（需要 API 密钥）
- **Bing Search**: Bing 搜索（需要 API 密钥）
- **Wikipedia**: 维基百科搜索

### 浏览器工具
- **Browser Use**: 浏览器自动化工具

### 系统工具
- **Command Line**: 执行系统命令
- **HTTP Request**: 发送 HTTP 请求
- **Sequential Thinking**: 顺序思考工具

### 自定义工具
- **Custom HTTP**: 自定义 HTTP 工具
- **Custom Exec**: 自定义命令执行工具

## 支持的模型提供商

- **OpenAI**: GPT-3.5, GPT-4 系列
- **Anthropic Claude**: Claude 3 系列
- **Google Gemini**: Gemini Pro 系列
- **阿里云通义千问**: Qwen 系列
- **DeepSeek**: DeepSeek 系列
- **字节跳动豆包**: 通过 Ark API
- **百度千帆**: 文心一言等
- **Ollama**: 本地模型部署

## 主要依赖

- [CloudWeGo Eino](https://github.com/cloudwego/eino) - AI 应用开发框架
- [Cobra](https://github.com/spf13/cobra) - 命令行界面框架
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - 交互式聊天的 TUI 框架
- 各种 Eino 扩展组件（模型和工具）

## 许可证

[MIT](LICENSE)
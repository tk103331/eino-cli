English

Eino CLI is an intelligent AI Agent command-line tool based on the [CloudWeGo Eino](https://github.com/cloudwego/eino) framework. It provides a powerful Agent system with support for multiple tool integrations and various AI model providers, enabling you to easily build and run custom AI Agents.

## Features

- **Intelligent Agent System**: AI Agent based on ReAct pattern with tool calling and reasoning capabilities
- **Interactive Chat Interface**: TUI-based chat interface for real-time conversations with AI models and agents
- **ACP Agent Mode**: Runs as an [Agent Client Protocol](https://agentclientprotocol.com) agent so editors like Zed can drive Eino CLI
- **Rich Tool Ecosystem**: Built-in various tools including search, browser, command line, HTTP requests, etc.
- **Multi-Model Provider Support**: Supports OpenAI, Claude, Gemini, Qwen, DeepSeek, Ollama, Baidu Qianfan, ByteDance Doubao, and more
- **Flexible Configuration System**: Manage Agents, tools, models, and chat presets through YAML configuration files
- **Custom Tool Support**: Supports custom HTTP and command-line tools
- **MCP Server Integration**: Supports Model Context Protocol (MCP) servers with both SSE and STDIO transport
- **Langfuse Integration**: Built-in observability with Langfuse for monitoring and tracing

## Installation

### Install

**Go install** (requires Go >= 1.24):

```bash
# Short command `eino` (recommended)
go install github.com/tk103331/eino-cli/cmd/eino@latest
# If you hit `sumdb ... 404` (checksum DB indexing delay for a new release):
# GONOSUMDB=github.com/tk103331/eino-cli go install github.com/tk103331/eino-cli/cmd/eino@latest

# Or `eino-cli` (same program)
go install github.com/tk103331/eino-cli@latest
```

**npm**:

```bash
npm install -g eino-cli    # provides both `eino` and `eino-cli`
eino
```

**Homebrew**:

```bash
brew install tk103331/tap/eino-cli   # provides both `eino` and `eino-cli`
```

**Releasing (maintainers)**:

```bash
git tag v0.1.0 && git push --tags
```

GitHub Actions (`.github/workflows/release.yml`) will:
1. build `eino` / `eino-cli` binaries for darwin/linux × amd64/arm64 with goreleaser and create a Release
2. publish the npm package via OIDC when the repository variable `NPM_TRUSTED_PUBLISHING=true` is set and the npm package has a GitHub Actions trusted publisher (version taken from the tag, no token needed)
3. update the `tk103331/homebrew-tap` formula when the `TAP_GITHUB_TOKEN` secret is configured

Preparation before the first release:
- npm automation token -> repo secret `NPM_TOKEN`
- create `tk103331/homebrew-tap`, personal access token (repo scope) -> secret `TAP_GITHUB_TOKEN`

**Build from source**:

```bash
git clone https://github.com/tk103331/eino-cli.git
cd eino-cli
go install ./cmd/eino    # produces `eino`
go install .             # produces `eino-cli`
```

## Usage

### 1. Configuration File

First, copy the configuration template and modify it as needed:

```bash
cp config.yml.example config.yml
```

The configuration file contains the following main sections:
- `providers`: AI model provider configuration (API keys, base URLs, etc.)
- `models`: Model configuration (temperature, max tokens, etc.)
- `agents`: Agent configuration (system prompts, models and tools to use)
- `chats`: Chat preset configuration for interactive chat mode
- `tools`: Tool configuration (parameters and settings for custom tools)
- `mcp_servers`: MCP server configuration
- `settings`: Global settings including Langfuse configuration



### 2. Interactive Agent/Chat Mode

The TUI supports slash commands (type `/` to see the command palette with argument candidates):

| Command | Description |
|---|---|
| `/help` | List all commands |
| `/clear` | Clear the current conversation |
| `/agent <name>` | Switch agent (agent mode) |
| `/model <name>` | Switch model without restarting |
| `/skill <name>` | Activate a configured skill; `/skill off` clears it |
| `/theme <name> [light\|dark]` | Switch color theme (every theme ships light and dark variants) |

The TUI does **not capture the mouse** by default, so native terminal text selection and copy work. It also enables the terminal's alternate-scroll mode (DECSET 1007), so on supported terminals (iTerm2, Kitty, WezTerm, xterm, ...) the wheel scrolls the transcript without any key press. If 1007 is unsupported, use ↑↓/PgUp/PgDn, or press `Ctrl+Y` to capture the mouse (press again to release).
| `/tools` | List available tools |
| `/status` | Show current agent/model/skill |

`skills` are switchable capability presets in `config.yml` (extra system prompt, optional extra tools):

```yaml
skills:
  translator:
    description: "Translation assistant (EN <-> ZH)"
    system: "You are a professional translator. Output only the translation."
  coder:
    description: "Coding assistant"
    system: "Prefer runnable code examples."
    tools: [shell]   # optional: extra tools
```

Skills can also live in **directories** (no config.yml needed), scanned in priority order:

- `~/.eino-cli/skills/*.md` (highest priority)
- `~/.agents/skills/*.md`

On name clashes the first directory wins; directory skills override config.yml entries.

Each file is a markdown document with optional front matter:

```markdown
---
description: Code review
tools: [shell]
---
You are a strict code reviewer. Start with correctness issues...
```

Start an interactive session with an agent or model:

```bash
# No arguments: resume the last used agent, or the built-in basic agent
eino-cli agent
eino-cli          # bare invocation is equivalent to `eino-cli agent`

# The built-in agent uses `default_model` (or the first model alphabetically);
# the last used agent is stored in ~/.eino-cli/last_agent
```



```bash
# Start an interactive agent session
eino-cli agent --agent search_agent

# Chat with a specific model
eino-cli agent --model gpt4

# Chat with tools
eino-cli agent --model gpt4 --tools duckduckgo_search,wikipedia_search

# Use a chat preset
eino-cli agent --chat search_chat
```

Parameter description:
- `--agent, -a`: Specify the agent name to run (optional)
- `--chat, -c`: Specify a chat preset name from configuration file (optional)
- `--model, -m`: Specify the model to chat with (required when not using --agent or --chat)
- `--tools, -t`: Specify available tools, separated by commas (optional when using --model directly)

### 3. Running an Agent

Use the `run` command to run a specified Agent:

```bash
eino-cli run --agent test_agent --prompt "Hello, please help me search for today's weather"
```

Parameter description:
- `--agent, -a`: Specify the Agent name to run (required)
- `--prompt, -p`: Specify the input prompt for the Agent (required)
- `--config`: Specify the configuration file path (optional, defaults to ~/.eino-cli/config.yml)

### 4. ACP Agent Mode (for editors like Zed)

Run Eino CLI as an [Agent Client Protocol (ACP)](https://agentclientprotocol.com) agent.
It speaks JSON-RPC 2.0 over stdio, so any ACP client (Zed, or your own tooling) can create sessions, send prompts, and receive streaming responses and tool call events.

```bash
# Expose an agent over ACP (defaults to the single configured agent)
eino-cli acp --agent search_agent
```

Example Zed configuration (`settings.json`):

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

Supported ACP surface:

- `initialize`, `session/new`, `session/prompt`, `session/cancel` (protocol v1)
- Streaming `session/update` notifications: `agent_message_chunk`, `tool_call`, `tool_call_update`
- Stop reasons: `end_turn` and `cancelled`
- Tools from `config.yml` (built-in tools and MCP servers) are exposed as ACP tool calls with status (`pending`/`in_progress`/`completed`/`failed`) and kind (`search`, `execute`, `fetch`, ...)

Note: logs are written to `~/.eino-cli/eino-cli.log` — stdout of the `acp` command is reserved for the protocol.

### 5. Configuration Example

Here's a complete configuration example:

```yaml
# Model provider configuration
providers:
  openai:
    type: openai
    base_url: https://api.openai.com/v1
    api_key: sk-xxxxx
  claude:
    type: claude
    api_key: sk-ant-xxxxx

# Model configuration
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

# Chat preset configuration
chats:
  search_chat:
    model: gpt4
    system: "You are a helpful search assistant. Use search tools to find information for users."
    tools:
      - duckduckgo_search
      - wikipedia_search
  coding_chat:
    model: claude_sonnet
    system: "You are an expert programmer helping with coding tasks."
    tools:
      - commandline
      - http_request

# MCP server configuration
mcp_servers:
  # SSE type MCP server
  sse_server:
    type: mcp
    config:
      url: "http://localhost:3000/mcp"  # MCP server URL
      headers:
        "Content-Type": "application/json"
        "Authorization": "Bearer your-token"  # Optional authentication header
  # STDIO type MCP server
  stdio_server:
    type: stdio
    config:
      cmd: "python"                    # Command to execute
      args:
        - "-m"
        - "your_mcp_server"             # MCP server module
      env:
        "PYTHONPATH": "/path/to/server" # Environment variables
        "API_KEY": "your-api-key"

# Agent configuration
agents:
  search_agent:
    system: "You are a search assistant that can help users search for information"
    model: gpt4
    tools:
      - duckduckgo_search
      - wikipedia_search
    mcp_servers:                        # MCP servers that the Agent can use
      - sse_server
  # Multi-functional assistant example (including custom tools and MCP servers)
  multi_agent:
    system: "You are a multi-functional assistant that can search information, query weather, get system information, etc."
    model: gpt4
    tools:
      - duckduckgo_search
      - weather_api
      - system_info
    mcp_servers:
      - sse_server
      - stdio_server

# Tool configuration
tools:
  duckduckgo_search:
    type: duckduckgo
    config:
      max_results: 10        # Maximum number of search results, default 10
      region: "wt"           # Search region: wt(global), cn(China), us(USA), uk(UK)
      safe_search: "off"     # Safe search: off(disabled), moderate(moderate), strict(strict)
      timeout: 10            # Timeout in seconds, default 10 seconds
  # Custom HTTP tool example
  weather_api:
    type: customhttp
    description: "API tool for getting weather information"
    config:
      url: "https://api.openweathermap.org/data/2.5/weather?q={{city}}"
      method: "GET"
      headers:
        "Content-Type": "application/json"
    params:
      - name: "city"
        type: "string"
        description: "City name"
        required: true
  # Custom command-line tool example
  system_info:
    type: customexec
    description: "Get system information"
    config:
      cmd: "uname -a && df -h"
      dir: "/tmp"
      timeout: 30
    params: []

# Global settings
settings:
  langfuse:
    host: https://cloud.langfuse.com
    public_key: pk-xxx
    secret_key: sk-xxx
```

## Supported Tools

Eino CLI comes with various built-in tools that can be used in Agents:

### Search Tools
- **DuckDuckGo**: Web search
- **Google Search**: Google search (requires API key)
- **Bing Search**: Bing search (requires API key)
- **Wikipedia**: Wikipedia search

### Browser Tools
- **Browser Use**: Browser automation tool

### System Tools
- **Command Line**: Execute system commands
- **HTTP Request**: Send HTTP requests
- **Sequential Thinking**: Sequential thinking tool

### Custom Tools
- **Custom HTTP**: Custom HTTP tools
- **Custom Exec**: Custom command execution tools

## Supported Model Providers

- **OpenAI**: GPT-3.5, GPT-4 series
- **Anthropic Claude**: Claude 3 series
- **Google Gemini**: Gemini Pro series
- **Alibaba Qwen**: Qwen series
- **DeepSeek**: DeepSeek series
- **ByteDance Doubao**: Through Ark API
- **Baidu Qianfan**: ERNIE and others
- **Ollama**: Local model deployment

## Main Dependencies

- [CloudWeGo Eino](https://github.com/cloudwego/eino) - AI application development framework
- [Cobra](https://github.com/spf13/cobra) - Command-line interface framework
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework for interactive chat
- Various Eino extension components (models and tools)

## License

[MIT](LICENSE)
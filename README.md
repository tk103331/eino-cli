English

Eino CLI is an intelligent AI Agent command-line tool based on the [CloudWeGo Eino](https://github.com/cloudwego/eino) framework. It provides a powerful Agent system with support for multiple tool integrations and various AI model providers, enabling you to easily build and run custom AI Agents.

## Features

- **Intelligent Agent System**: AI Agent based on ReAct pattern with tool calling and reasoning capabilities
- **Interactive Chat Interface**: TUI-based chat interface for real-time conversations with AI models and agents
- **Rich Tool Ecosystem**: Built-in various tools including search, browser, command line, HTTP requests, etc.
- **Multi-Model Provider Support**: Supports OpenAI, Claude, Gemini, Qwen, DeepSeek, Ollama, Baidu Qianfan, ByteDance Doubao, and more
- **Flexible Configuration System**: Manage Agents, tools, models, and chat presets through YAML configuration files
- **Custom Tool Support**: Supports custom HTTP and command-line tools
- **MCP Server Integration**: Supports Model Context Protocol (MCP) servers with both SSE and STDIO transport
- **Langfuse Integration**: Built-in observability with Langfuse for monitoring and tracing

## Installation

### Install from Source

```bash
git clone https://github.com/tk103331/eino-cli.git
cd eino-cli
go install
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

Start an interactive session with an agent or model:

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

### 4. Configuration Example

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
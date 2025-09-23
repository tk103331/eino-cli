# Eino CLI

[English](README.md) | 中文

Eino CLI 是一个基于 [CloudWeGo Eino](https://github.com/cloudwego/eino) 框架的智能 AI Agent 命令行工具。它提供了强大的 Agent 系统，支持多种工具集成和多个 AI 模型提供商，让您能够轻松构建和运行自定义的 AI Agent。

## 功能特点

- **智能 Agent 系统**：基于 ReAct 模式的 AI Agent，支持工具调用和推理
- **丰富的工具生态**：内置多种工具，包括搜索、浏览器、命令行、HTTP 请求等
- **多模型提供商支持**：支持 OpenAI、Claude、Gemini、Qwen、DeepSeek、Ollama 等
- **灵活的配置系统**：通过 YAML 配置文件管理 Agent、工具和模型
- **自定义工具支持**：支持自定义 HTTP 和命令行工具
- **MCP 服务器集成**：支持 Model Context Protocol 服务器

## 安装

### 从源码安装

```bash
git clone https://github.com/tk103331/eino-cli.git
cd eino-cli
go install
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
- `tools`: 工具配置（自定义工具的参数和配置）
- `mcp_servers`: MCP 服务器配置

### 2. 运行 Agent

使用 `run` 命令运行指定的 Agent：

```bash
eino-cli run --agent test_agent --prompt "你好，请帮我搜索一下今天的天气"
```

参数说明：
- `--agent, -a`: 指定要运行的 Agent 名称（必需）
- `--prompt, -p`: 指定 Agent 的输入提示（必需）
- `--config`: 指定配置文件路径（可选，默认为 config.yml）

### 3. 配置示例

以下是一个完整的配置示例：

```yaml
# 模型提供商配置
providers:
  openai:
    type: openai
    base_url: https://api.openai.com/v1
    api_key: sk-xxxxx

# 模型配置
models:
  gpt4:
    provider: openai
    model: gpt-4
    max_tokens: 4096
    temperature: 0.7

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
- 各种 Eino 扩展组件（模型和工具）

## 许可证

[MIT](LICENSE)
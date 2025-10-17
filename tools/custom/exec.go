package custom

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/template"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/tk103331/eino-cli/config"
)

// ExecConfig 执行工具配置结构体
type ExecConfig struct {
	Cmd     string            `yaml:"cmd"`     // 执行命令模板
	WorkDir string            `yaml:"workdir"` // 工作目录
	Env     map[string]string `yaml:"env"`     // 环境变量
	Timeout int               `yaml:"timeout"` // 超时时间(秒)
}

// ExecTool 执行工具实现
type ExecTool struct {
	info       *schema.ToolInfo
	config     config.Tool
	execConfig *ExecConfig
}

// NewExecTool 创建执行工具
func NewExecTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	// 初始化ExecConfig
	execConfig := &ExecConfig{}
	if cfg.Config != nil {
		if cmdValue, exists := cfg.Config["cmd"]; exists {
			execConfig.Cmd = cmdValue.String()
		}
		if workdirValue, exists := cfg.Config["workdir"]; exists {
			execConfig.WorkDir = workdirValue.String()
		}
		if timeoutValue, exists := cfg.Config["timeout"]; exists {
			execConfig.Timeout = timeoutValue.Int()
		}
		if envValue, exists := cfg.Config["env"]; exists && envValue.IsMap() {
			execConfig.Env = make(map[string]string)
			for k, v := range envValue.Map() {
				execConfig.Env[k] = v.String()
			}
		}
	}

	// 检查必要属性
	if execConfig.Cmd == "" {
		return nil, fmt.Errorf("exec工具必须配置cmd属性")
	}

	// 设置默认值
	if execConfig.Timeout == 0 {
		execConfig.Timeout = 30 // 默认30秒超时
	}

	// 获取描述信息
	desc := cfg.Description
	if desc == "" {
		desc = "执行工具"
	}

	// 创建工具信息
	toolInfo := &schema.ToolInfo{
		Name: name,
		Desc: desc,
	}

	// 添加参数信息
	params := make(map[string]*schema.ParameterInfo)
	for _, param := range cfg.Params {
		// 将字符串类型转换为schema.DataType
		var dataType schema.DataType
		switch param.Type {
		case "string":
			dataType = schema.String
		case "number":
			dataType = schema.Number
		case "integer":
			dataType = schema.Integer
		case "boolean":
			dataType = schema.Boolean
		case "array":
			dataType = schema.Array
		case "object":
			dataType = schema.Object
		default:
			dataType = schema.String
		}

		params[param.Name] = &schema.ParameterInfo{
			Type: dataType,
			Desc: param.Description,
		}
	}
	toolInfo.ParamsOneOf = schema.NewParamsOneOfByParams(params)

	return &ExecTool{
		info:       toolInfo,
		config:     cfg,
		execConfig: execConfig,
	}, nil
}

// Info 获取工具信息
func (e *ExecTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return e.info, nil
}

// InvokableRun 实现InvokableTool接口
func (e *ExecTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// 解析参数
	var args map[string]interface{}
	if argumentsInJSON != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
			return "", fmt.Errorf("解析参数失败: %v", err)
		}
	}

	// 渲染命令模板
	cmdStr, err := e.renderTemplate(e.execConfig.Cmd, args)
	if err != nil {
		return "", fmt.Errorf("渲染命令模板失败: %v", err)
	}

	// 解析命令和参数
	cmdParts := strings.Fields(cmdStr)
	if len(cmdParts) == 0 {
		return "", fmt.Errorf("命令为空")
	}

	// 创建命令
	cmd := exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)

	// 设置工作目录
	if e.execConfig.WorkDir != "" {
		workDir := e.execConfig.WorkDir
		// 处理 ~ 符号
		if strings.HasPrefix(workDir, "~/") {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("获取用户主目录失败: %v", err)
			}
			workDir = strings.Replace(workDir, "~", homeDir, 1)
		}
		cmd.Dir = workDir
	}

	// 设置环境变量
	cmd.Env = os.Environ()
	if e.execConfig.Env != nil {
		for key, value := range e.execConfig.Env {
			envValue, err := e.renderTemplate(value, args)
			if err != nil {
				return "", fmt.Errorf("渲染环境变量模板失败: %v", err)
			}
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, envValue))
		}
	}

	// 执行命令并捕获输出
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		// 如果命令执行失败，返回错误信息和stderr
		errorMsg := fmt.Sprintf("命令执行失败: %v", err)
		if stderr.Len() > 0 {
			errorMsg += fmt.Sprintf("\nstderr: %s", stderr.String())
		}
		if stdout.Len() > 0 {
			errorMsg += fmt.Sprintf("\nstdout: %s", stdout.String())
		}
		return "", errors.New(errorMsg)
	}

	// 返回标准输出
	result := stdout.String()
	if stderr.Len() > 0 {
		// 如果有stderr但命令成功执行，将stderr作为警告信息附加
		result += fmt.Sprintf("\n[警告] %s", stderr.String())
	}

	return result, nil
}

// renderTemplate 渲染模板
func (e *ExecTool) renderTemplate(templateStr string, args map[string]interface{}) (string, error) {
	tmpl, err := template.New("exec").Parse(templateStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, args); err != nil {
		return "", err
	}

	return buf.String(), nil
}

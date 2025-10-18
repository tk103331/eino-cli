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

// ExecConfig execution tool configuration structure
type ExecConfig struct {
	Cmd     string            `yaml:"cmd"`     // Execution command template
	WorkDir string            `yaml:"workdir"` // Working directory
	Env     map[string]string `yaml:"env"`     // Environment variables
	Timeout int               `yaml:"timeout"` // Timeout in seconds
}

// ExecTool execution tool implementation
type ExecTool struct {
	info       *schema.ToolInfo
	config     config.Tool
	execConfig *ExecConfig
}

// NewExecTool creates execution tool
func NewExecTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	// Initialize ExecConfig
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

	// Check required attributes
	if execConfig.Cmd == "" {
		return nil, fmt.Errorf("exec tool must configure cmd attribute")
	}

	// Set default values
	if execConfig.Timeout == 0 {
		execConfig.Timeout = 30 // Default 30 seconds timeout
	}

	// Get description information
	desc := cfg.Description
	if desc == "" {
		desc = "execution tool"
	}

	// Create tool information
	toolInfo := &schema.ToolInfo{
		Name: name,
		Desc: desc,
	}

	// Add parameter information
	params := make(map[string]*schema.ParameterInfo)
	for _, param := range cfg.Params {
		// Convert string type to schema.DataType
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

// Info gets tool information
func (e *ExecTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return e.info, nil
}

// InvokableRun implements InvokableTool interface
func (e *ExecTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// Parse parameters
	var args map[string]interface{}
	if argumentsInJSON != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
			return "", fmt.Errorf("failed to parse parameters: %v", err)
		}
	}

	// Render command template
	cmdStr, err := e.renderTemplate(e.execConfig.Cmd, args)
	if err != nil {
		return "", fmt.Errorf("failed to render command template: %v", err)
	}

	// Parse command and arguments
	cmdParts := strings.Fields(cmdStr)
	if len(cmdParts) == 0 {
		return "", fmt.Errorf("command is empty")
	}

	// Create command
	cmd := exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)

	// Set working directory
	if e.execConfig.WorkDir != "" {
		workDir := e.execConfig.WorkDir
		// Handle ~ symbol
		if strings.HasPrefix(workDir, "~/") {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("failed to get user home directory: %v", err)
			}
			workDir = strings.Replace(workDir, "~", homeDir, 1)
		}
		cmd.Dir = workDir
	}

	// Set environment variables
	cmd.Env = os.Environ()
	if e.execConfig.Env != nil {
		for key, value := range e.execConfig.Env {
			envValue, err := e.renderTemplate(value, args)
			if err != nil {
				return "", fmt.Errorf("failed to render environment variable template: %v", err)
			}
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, envValue))
		}
	}

	// Execute command and capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		// If command execution fails, return error message and stderr
		errorMsg := fmt.Sprintf("command execution failed: %v", err)
		if stderr.Len() > 0 {
			errorMsg += fmt.Sprintf("\nstderr: %s", stderr.String())
		}
		if stdout.Len() > 0 {
			errorMsg += fmt.Sprintf("\nstdout: %s", stdout.String())
		}
		return "", errors.New(errorMsg)
	}

	// Return standard output
	result := stdout.String()
	if stderr.Len() > 0 {
		// If there's stderr but command succeeded, append stderr as warning
		result += fmt.Sprintf("\n[warning] %s", stderr.String())
	}

	return result, nil
}

// renderTemplate renders template
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

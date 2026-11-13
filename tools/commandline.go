package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/tools/approval"
)

// shellTool runs a shell command on the local system.
//
// Previously this wrapped eino-ext commandline.NewStrReplaceEditor, which
// requires an Operator implementation (upstream examples need a Docker
// sandbox). For a local CLI a plain self-contained shell tool is the right
// fit, so it is implemented directly on top of os/exec.
type shellTool struct {
	info    *schema.ToolInfo
	workdir string
	timeout time.Duration
}

// NewCommandLineTool creates the "commandline" shell tool.
//
// Config options:
//   - workdir: working directory for commands (optional)
//   - timeout: execution timeout in seconds (default 60)
func NewCommandLineTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	t := &shellTool{
		timeout: 60 * time.Second,
	}

	if cfg.Config != nil {
		if v, ok := cfg.Config["workdir"]; ok {
			t.workdir = v.String()
		}
		if v, ok := cfg.Config["timeout"]; ok {
			if secs := v.Int(); secs > 0 {
				t.timeout = time.Duration(secs) * time.Second
			}
		}
	}

	desc := cfg.Description
	if desc == "" {
		desc = "Execute a shell command on the local system and return its output. " +
			"Prefer read-only commands (ls, cat, echo, pwd, uname). " +
			"The command runs with /bin/sh -c."
	}

	t.info = &schema.ToolInfo{
		Name: name,
		Desc: desc,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"command": {
				Type:     schema.String,
				Desc:     "The shell command to execute",
				Required: true,
			},
		}),
	}
	return t, nil
}

func (t *shellTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.info, nil
}

type shellInput struct {
	Command string `json:"command"`
}

// ApprovalRequest flags every shell command: the tool can do anything.
func (t *shellTool) ApprovalRequest(arguments string) (approval.Request, bool) {
	var in shellInput
	summary := "运行 shell 命令"
	if err := json.Unmarshal([]byte(arguments), &in); err == nil && strings.TrimSpace(in.Command) != "" {
		summary = "运行命令：" + strings.TrimSpace(in.Command)
	}
	return approval.Request{Tool: t.info.Name, Summary: summary, Detail: arguments}, true
}

func (t *shellTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	in := &shellInput{}
	if err := json.Unmarshal([]byte(argumentsInJSON), in); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	command := strings.TrimSpace(in.Command)
	if command == "" {
		return "", fmt.Errorf("command is required")
	}

	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	if t.workdir != "" {
		cmd.Dir = t.workdir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := strings.TrimSpace(stdout.String())
	if stderrText := strings.TrimSpace(stderr.String()); stderrText != "" {
		if output != "" {
			output += "\n[stderr] " + stderrText
		} else {
			output = "[stderr] " + stderrText
		}
	}

	if err != nil {
		if ctx.Err() != nil {
			return fmt.Sprintf("command timed out after %s\n%s", t.timeout, output), nil
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Sprintf("command exited with code %d\n%s", exitErr.ExitCode(), output), nil
		}
		return "", fmt.Errorf("failed to run command: %w", err)
	}

	if output == "" {
		return "(no output)", nil
	}
	return output, nil
}

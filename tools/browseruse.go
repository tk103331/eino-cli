package tools

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudwego/eino-ext/components/tool/browseruse"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/tk103331/eino-cli/config"
)

// NewBrowserUseTool creates browser usage tool.
//
// The upstream constructor launches Chrome immediately, which made every agent
// startup pop a browser window (browseruse is one of the built-in agent's
// default tools). The returned tool therefore starts the browser lazily, on
// the first actual tool call.
func NewBrowserUseTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	t := &browserTool{}
	if v, ok := cfg.Config["headless"]; ok {
		t.headless = v.Bool()
	}
	if v, ok := cfg.Config["disable_security"]; ok {
		t.disableSecurity = v.Bool()
	}
	if v, ok := cfg.Config["chrome_path"]; ok {
		t.chromePath = strings.TrimSpace(v.String())
	}
	if v, ok := cfg.Config["proxy"]; ok {
		t.proxy = strings.TrimSpace(v.String())
	}
	if v, ok := cfg.Config["args"]; ok && v.IsArray() {
		for _, arg := range v.Array() {
			if s := strings.TrimSpace(arg.String()); s != "" {
				t.extraArgs = append(t.extraArgs, s)
			}
		}
	}

	desc := cfg.Description
	if desc == "" {
		desc = "Control a real web browser. Actions: 'go_to_url' (open a URL), " +
			"'web_search' (search the query in the current tab), 'click_element' (by index), " +
			"'input_text' (type text into an element), 'scroll_down'/'scroll_up', " +
			"'extract_content' (pull information from the page for a goal), " +
			"'switch_tab'/'open_tab'/'close_tab', and 'wait'."
	}
	t.info = &schema.ToolInfo{
		Name: name,
		Desc: desc,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"action": {
				Type:     schema.String,
				Desc:     "The browser action to perform.",
				Enum:     browserActions(),
				Required: true,
			},
			"url": {
				Type: schema.String,
				Desc: "URL for 'go_to_url' or 'open_tab' actions.",
			},
			"index": {
				Type: schema.Integer,
				Desc: "Element index for 'click_element' or 'input_text'.",
			},
			"text": {
				Type: schema.String,
				Desc: "Text to type for 'input_text'.",
			},
			"query": {
				Type: schema.String,
				Desc: "Search query for 'web_search'.",
			},
			"goal": {
				Type: schema.String,
				Desc: "What to extract for 'extract_content'.",
			},
			"keys": {
				Type: schema.String,
				Desc: "Keys to press (optional).",
			},
			"seconds": {
				Type: schema.Integer,
				Desc: "Seconds to wait for 'wait'.",
			},
			"scroll_amount": {
				Type: schema.Integer,
				Desc: "Pixels to scroll (optional).",
			},
			"tab_id": {
				Type: schema.Integer,
				Desc: "Tab id for 'switch_tab' or 'close_tab'.",
			},
		}),
	}
	return t, nil
}

// browserActions lists the upstream action names offered to the model.
func browserActions() []string {
	return []string{
		string(browseruse.ActionGoToURL),
		string(browseruse.ActionClickElement),
		string(browseruse.ActionInputText),
		string(browseruse.ActionScrollDown),
		string(browseruse.ActionScrollUp),
		string(browseruse.ActionWebSearch),
		string(browseruse.ActionWait),
		string(browseruse.ActionExtractContent),
		string(browseruse.ActionSwitchTab),
		string(browseruse.ActionOpenTab),
		string(browseruse.ActionCloseTab),
	}
}

// browserTool starts the underlying browser only when it is first used.
type browserTool struct {
	info            *schema.ToolInfo
	headless        bool
	disableSecurity bool
	chromePath      string
	proxy           string
	extraArgs       []string

	mu    sync.Mutex
	inner tool.InvokableTool
}

// Info returns the tool metadata without touching the browser.
func (t *browserTool) Info(context.Context) (*schema.ToolInfo, error) {
	return t.info, nil
}

// InvokableRun starts the browser on the first call, then reuses it. Calls are
// serialized because a browser session is stateful.
func (t *browserTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.inner == nil {
		inner, err := browseruse.NewBrowserUseTool(context.Background(), &browseruse.Config{
			Headless:           t.headless,
			DisableSecurity:    t.disableSecurity,
			ExtraChromiumArgs:  t.extraArgs,
			ChromeInstancePath: t.chromePath,
			ProxyServer:        t.proxy,
		})
		if err != nil {
			return "", fmt.Errorf("failed to start browser: %w", err)
		}
		t.inner = inner
	}
	return t.inner.InvokableRun(ctx, argumentsInJSON, opts...)
}

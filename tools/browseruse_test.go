package tools

import (
	"context"
	"testing"

	"github.com/tk103331/eino-cli/config"
	"gopkg.in/yaml.v3"
)

// The browser must not be launched while the agent is being built: startup
// used to open a Chrome window because the upstream constructor initializes
// the browser eagerly.
func TestBrowserUseToolIsLazy(t *testing.T) {
	tl, err := NewBrowserUseTool("browseruse", config.Tool{Type: "browseruse"})
	if err != nil {
		t.Fatal(err)
	}
	bt, ok := tl.(*browserTool)
	if !ok {
		t.Fatalf("unexpected tool type %T", tl)
	}
	if bt.inner != nil {
		t.Fatal("browser must not start before the first call")
	}

	info, err := tl.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "browseruse" {
		t.Fatalf("tool name = %q, want browseruse", info.Name)
	}
	if info.Desc == "" {
		t.Fatal("description must be set")
	}

	schema, err := info.ParamsOneOf.ToJSONSchema()
	if err != nil {
		t.Fatal(err)
	}
	if schema == nil || schema.Properties == nil {
		t.Fatal("parameters missing")
	}
	if _, ok := schema.Properties.Get("action"); !ok {
		t.Fatalf("action parameter missing: %+v", schema.Properties)
	}
	if _, ok := schema.Properties.Get("url"); !ok {
		t.Fatalf("url parameter missing: %+v", schema.Properties)
	}
	required := false
	for _, name := range schema.Required {
		if name == "action" {
			required = true
		}
	}
	if !required {
		t.Fatalf("action must be required: %v", schema.Required)
	}
}

func TestBrowserUseToolConfig(t *testing.T) {
	var cfg config.Tool
	if err := yaml.Unmarshal([]byte("type: browseruse\nconfig:\n  headless: true\n  chrome_path: /opt/chrome\n  proxy: http://proxy:8080\n  args: [--no-sandbox]\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	tl, err := NewBrowserUseTool("browseruse", cfg)
	if err != nil {
		t.Fatal(err)
	}
	bt := tl.(*browserTool)
	if !bt.headless {
		t.Fatal("headless should be enabled from config")
	}
	if bt.chromePath != "/opt/chrome" || bt.proxy != "http://proxy:8080" {
		t.Fatalf("browser config = %+v", bt)
	}
	if len(bt.extraArgs) != 1 || bt.extraArgs[0] != "--no-sandbox" {
		t.Fatalf("extra args = %v", bt.extraArgs)
	}
}

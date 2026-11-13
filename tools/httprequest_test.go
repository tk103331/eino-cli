package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/tools/approval"
	"gopkg.in/yaml.v3"
)

func newHTTPRequestTestTool(t *testing.T, name, spec string) tool.InvokableTool {
	t.Helper()
	var cfg config.Tool
	if err := yaml.Unmarshal([]byte(spec), &cfg); err != nil {
		t.Fatalf("cannot parse test config: %v", err)
	}
	tl, err := NewHTTPRequestTool(name, cfg)
	if err != nil {
		t.Fatalf("NewHTTPRequestTool: %v", err)
	}
	return tl
}

func TestHTTPRequestToolUsesConfiguredName(t *testing.T) {
	tl := newHTTPRequestTestTool(t, "httprequest", "type: httprequest\n")

	info, err := tl.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "httprequest" {
		t.Fatalf("tool name = %q, want the configured name", info.Name)
	}

	classifier, ok := tl.(approval.Classifier)
	if !ok {
		t.Fatal("http request tool should classify its calls")
	}
	if _, needs := classifier.ApprovalRequest(`{"url":"https://example.com"}`); needs {
		t.Fatal("GET should not need approval")
	}
}

func TestHTTPRequestToolFlagsWrites(t *testing.T) {
	tl := newHTTPRequestTestTool(t, "webhook", "type: httprequest\nconfig:\n  method: post\n")

	info, err := tl.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "webhook" {
		t.Fatalf("tool name = %q", info.Name)
	}

	classifier := tl.(approval.Classifier)
	req, needs := classifier.ApprovalRequest(`{"url":"https://example.com/hook","data":"{}"}`)
	if !needs {
		t.Fatal("POST should need approval")
	}
	if !strings.Contains(req.Summary, "POST") || !strings.Contains(req.Summary, "https://example.com/hook") {
		t.Fatalf("summary = %q", req.Summary)
	}
}

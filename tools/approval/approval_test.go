package approval

import "testing"

// fakeClassifier flags every call as dangerous and returns a fixed summary.
type fakeClassifier struct {
	summary   string
	dangerous bool
}

func (f fakeClassifier) ApprovalRequest(arguments string) (Request, bool) {
	return Request{Summary: f.summary, Detail: arguments}, f.dangerous
}

func TestModeOrDefault(t *testing.T) {
	cases := map[string]string{
		"":          ModeDangerous,
		"off":       ModeOff,
		"OFF":       ModeOff,
		"all":       ModeAll,
		"dangerous": ModeDangerous,
		"nonsense":  ModeDangerous,
		"  all  ":   ModeAll,
	}
	for mode, want := range cases {
		if got := (Policy{Mode: mode}).ModeOrDefault(); got != want {
			t.Fatalf("ModeOrDefault(%q) = %q, want %q", mode, got, want)
		}
	}
}

func TestNeedsApprovalDangerousMode(t *testing.T) {
	policy := Policy{Mode: ModeDangerous}

	req, needs := policy.NeedsApproval("shell", `{"command":"ls"}`, fakeClassifier{summary: "运行命令：ls", dangerous: true})
	if !needs || req.Tool != "shell" || req.Summary != "运行命令：ls" || req.Detail != `{"command":"ls"}` {
		t.Fatalf("dangerous call not flagged: %+v %v", req, needs)
	}

	if _, needs := policy.NeedsApproval("search", `{}`, fakeClassifier{dangerous: false}); needs {
		t.Fatal("safe call should not need approval")
	}

	// Tools without a classifier are not guessed at in dangerous mode.
	if _, needs := policy.NeedsApproval("mcp_tool", `{}`, nil); needs {
		t.Fatal("unclassified tool should not need approval in dangerous mode")
	}
}

func TestNeedsApprovalAllMode(t *testing.T) {
	policy := Policy{Mode: ModeAll}

	for _, classifier := range []Classifier{nil, fakeClassifier{dangerous: false}} {
		req, needs := policy.NeedsApproval("search", `{"q":"x"}`, classifier)
		if !needs {
			t.Fatalf("all mode should always ask (classifier=%v)", classifier)
		}
		if req.Summary != "调用工具 search" || req.Detail != `{"q":"x"}` {
			t.Fatalf("default request wrong: %+v", req)
		}
	}

	// A classifier summary is still preferred in all mode.
	req, needs := policy.NeedsApproval("shell", `{}`, fakeClassifier{summary: "运行命令：rm", dangerous: true})
	if !needs || req.Summary != "运行命令：rm" {
		t.Fatalf("classifier summary ignored: %+v", req)
	}
}

func TestNeedsApprovalOverrides(t *testing.T) {
	if _, needs := (Policy{Mode: ModeOff}).NeedsApproval("shell", "", fakeClassifier{dangerous: true}); needs {
		t.Fatal("off mode must never ask")
	}
	if _, needs := (Policy{Mode: ModeDangerous, Allow: map[string]bool{"shell": true}}).NeedsApproval("shell", "", fakeClassifier{dangerous: true}); needs {
		t.Fatal("allow list must win")
	}
	if _, needs := (Policy{Mode: ModeDangerous, Always: map[string]bool{"mcp": true}}).NeedsApproval("mcp", "", nil); !needs {
		t.Fatal("always list must force approval")
	}
}

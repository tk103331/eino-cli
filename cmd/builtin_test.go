package cmd

import (
	"testing"

	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/state"
)

func TestEnsureBuiltinAgentUsesDefaultModel(t *testing.T) {
	cfg := &config.Config{
		Models: map[string]config.Model{
			"alpha": {Provider: "p", Model: "m"},
			"zeta":  {Provider: "p", Model: "m"},
		},
		DefaultModel: "zeta",
	}

	name, err := ensureBuiltinAgent(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if name != builtinAgentName {
		t.Fatalf("name = %q", name)
	}
	agent, ok := cfg.Agents[name]
	if !ok {
		t.Fatal("builtin agent not registered")
	}
	if agent.Model != "zeta" {
		t.Fatalf("model = %q, want default_model zeta", agent.Model)
	}
	if agent.System == "" {
		t.Fatal("builtin agent system prompt missing")
	}
}

func TestEnsureBuiltinAgentFallbackModel(t *testing.T) {
	cfg := &config.Config{
		Models: map[string]config.Model{
			"beta":  {Provider: "p", Model: "m"},
			"alpha": {Provider: "p", Model: "m"},
		},
	}
	if _, err := ensureBuiltinAgent(cfg); err != nil {
		t.Fatal(err)
	}
	if got := cfg.Agents[builtinAgentName].Model; got != "alpha" {
		t.Fatalf("fallback model = %q, want alphabetically first", got)
	}

	// No models at all is a helpful error.
	empty := &config.Config{}
	if _, err := ensureBuiltinAgent(empty); err == nil {
		t.Fatal("expected error without models")
	}
}

func TestResolveDefaultAgent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_ = state.ClearLastAgent()

	cfg := &config.Config{
		Agents: map[string]config.Agent{},
		Models: map[string]config.Model{"m1": {Provider: "p", Model: "m"}},
	}

	// No previous agent -> built-in.
	name, err := resolveDefaultAgent(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if name != builtinAgentName {
		t.Fatalf("default = %q, want builtin", name)
	}

	// A previous agent that still exists wins.
	cfg.Agents["worker"] = config.Agent{Model: "m1"}
	if err := state.SetLastAgent("worker"); err != nil {
		t.Fatal(err)
	}
	name, err = resolveDefaultAgent(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if name != "worker" {
		t.Fatalf("default = %q, want worker", name)
	}

	// A stale previous agent falls back to built-in.
	if err := state.SetLastAgent("gone"); err != nil {
		t.Fatal(err)
	}
	name, err = resolveDefaultAgent(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if name != builtinAgentName {
		t.Fatalf("default = %q, want builtin for stale last agent", name)
	}
}

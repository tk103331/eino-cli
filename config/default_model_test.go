package config

import "testing"

func TestDefaultModelName(t *testing.T) {
	cfg := &Config{
		DefaultModel: "zeta",
		Models: map[string]Model{
			"alpha": {},
			"zeta":  {},
		},
	}
	if got := cfg.DefaultModelName(); got != "zeta" {
		t.Fatalf("default model = %q, want zeta", got)
	}

	// A default_model that is not configured falls back to the first model.
	cfg.DefaultModel = "missing"
	if got := cfg.DefaultModelName(); got != "alpha" {
		t.Fatalf("fallback default model = %q, want alpha", got)
	}

	cfg.DefaultModel = ""
	if got := cfg.DefaultModelName(); got != "alpha" {
		t.Fatalf("fallback default model = %q, want alpha", got)
	}

	cfg.Models = nil
	if got := cfg.DefaultModelName(); got != "" {
		t.Fatalf("default model = %q, want empty", got)
	}
}

func TestAgentUsesDefaultModel(t *testing.T) {
	cfg := &Config{
		DefaultModel: "zeta",
		Models: map[string]Model{
			"alpha": {},
			"zeta":  {},
		},
		Agents: map[string]Agent{
			"builtin": {Model: "zeta"},
			"pinned":  {Model: "alpha"},
			"empty":   {},
		},
	}
	if !cfg.AgentUsesDefaultModel("builtin") {
		t.Fatal("agent using the default model should match")
	}
	if cfg.AgentUsesDefaultModel("pinned") {
		t.Fatal("agent with its own model must not match")
	}
	if !cfg.AgentUsesDefaultModel("empty") {
		t.Fatal("agent without a model follows the default model")
	}
	if cfg.AgentUsesDefaultModel("missing") {
		t.Fatal("unknown agent must not match")
	}
}

package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLastAgentRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if got := LastAgent(); got != "" {
		t.Fatalf("expected empty last agent, got %q", got)
	}

	if err := SetLastAgent("minimax_agent"); err != nil {
		t.Fatal(err)
	}
	if got := LastAgent(); got != "minimax_agent" {
		t.Fatalf("last agent = %q", got)
	}

	// The state file lives under ~/.eino-cli.
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".eino-cli", "last_agent")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("state file missing: %v", err)
	}

	if err := ClearLastAgent(); err != nil {
		t.Fatal(err)
	}
	if got := LastAgent(); got != "" {
		t.Fatalf("last agent should be cleared, got %q", got)
	}
}

func TestRememberedModelRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if got := RememberedModel("Eino"); got != "" {
		t.Fatalf("expected no remembered model, got %q", got)
	}

	if err := SetRememberedModel("Eino", "claude"); err != nil {
		t.Fatal(err)
	}
	if err := SetRememberedModel("other", "gpt4"); err != nil {
		t.Fatal(err)
	}
	if got := RememberedModel("Eino"); got != "claude" {
		t.Fatalf("remembered model = %q", got)
	}
	if got := RememberedModel("other"); got != "gpt4" {
		t.Fatalf("remembered model = %q", got)
	}

	// The state file lives under ~/.eino-cli.
	home, _ := os.UserHomeDir()
	if _, err := os.Stat(filepath.Join(home, ".eino-cli", "agent_models.json")); err != nil {
		t.Fatalf("state file missing: %v", err)
	}

	// An empty model forgets the entry and leaves the others intact.
	if err := SetRememberedModel("Eino", ""); err != nil {
		t.Fatal(err)
	}
	if got := RememberedModel("Eino"); got != "" {
		t.Fatalf("remembered model should be cleared, got %q", got)
	}
	if got := RememberedModel("other"); got != "gpt4" {
		t.Fatalf("other agent must keep its model, got %q", got)
	}
}

func TestRememberedModelToleratesCorruptFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".eino-cli")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "agent_models.json"), []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}

	if got := RememberedModel("Eino"); got != "" {
		t.Fatalf("corrupt file should read as empty, got %q", got)
	}
	// Writing must recover the file rather than fail.
	if err := SetRememberedModel("Eino", "claude"); err != nil {
		t.Fatal(err)
	}
	if got := RememberedModel("Eino"); got != "claude" {
		t.Fatalf("remembered model = %q", got)
	}
}
